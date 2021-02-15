package replicaupdater

import (
	"context"
	"fmt"
	"github.com/lterrac/system-autoscaler/pkg/apis/systemautoscaler/v1beta1"
	saclientset "github.com/lterrac/system-autoscaler/pkg/generated/clientset/versioned"
	samplescheme "github.com/lterrac/system-autoscaler/pkg/generated/clientset/versioned/scheme"
	"github.com/lterrac/system-autoscaler/pkg/informers"
	"github.com/lterrac/system-autoscaler/pkg/queue"
	"github.com/modern-go/concurrent"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"time"
)

const (
	controllerAgentName = "pod-replica-updater"
)

// Controller is the component that controls the number of replicas of a pod.
// For each pod under the same ServiceLevelAgreement, it periodically suggest new replica values.
type Controller struct {

	// saClientSet is a clientset for our own API group
	saClientSet saclientset.Interface

	// kubernetesCLientset is the client-go of kubernetes
	kubernetesClientset kubernetes.Interface

	listers informers.Listers

	containerScaleSynced cache.InformerSynced
	podSynced            cache.InformerSynced

	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder

	// MetricClient is a client that polls the metrics from the pod.
	MetricClient *Client

	// Key: namespace-name of the application, Value: assigned logic
	logicMap concurrent.Map

	// workqueue contains all the servicelevelagreements that needs a recommendation
	workqueue queue.Queue
}

// NewController returns a new sample controller
func NewController(kubernetesClientset *kubernetes.Clientset,
	saClientSet saclientset.Interface,
	informers informers.Informers) *Controller {

	// Create event broadcaster
	// Add sample-controller types to the default Kubernetes Scheme so Events can be
	// logged for sample-controller types.
	utilruntime.Must(samplescheme.AddToScheme(scheme.Scheme))
	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	// Instantiate the Controller
	controller := &Controller{
		saClientSet:          saClientSet,
		kubernetesClientset:  kubernetesClientset,
		recorder:             recorder,
		listers:              informers.GetListers(),
		containerScaleSynced: informers.ContainerScale.Informer().HasSynced,
		podSynced:            informers.Pod.Informer().HasSynced,
		MetricClient:         NewMetricClient(),
		workqueue:            queue.NewQueue("SLAQueue"),
	}

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting pod replica updater controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh,
		c.containerScaleSynced,
		c.podSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Starting pod replica updater workers")
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, 5*time.Second, stopCh)
		go wait.Until(c.runSLAWorker, time.Second, stopCh)
	}

	return nil
}

// Shutdown is called when the controller has finished its work
func (c *Controller) Shutdown() {
	utilruntime.HandleCrash()
}

// runWorker enqueues slas that needs to be processed
func (c *Controller) runWorker() {
	slas, err := c.listers.ServiceLevelAgreementLister.List(labels.Everything())
	if err != nil {
		klog.Error(err)
		return
	}

	for _, sla := range slas {
		c.workqueue.Enqueue(sla)
	}
}

func (c *Controller) runSLAWorker() {
	for c.workqueue.ProcessNextItem(c.handleSLA) {
	}
}

func (c *Controller) handleSLA(key string) error {

	slaNamespace, slaName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		klog.Error("Failed to split key, error: ", err)
		return err
	}

	sla, err := c.listers.ServiceLevelAgreements(slaNamespace).Get(slaName)
	if err != nil {
		klog.Error("Failed to retrieve the sla, error: ", err)
		return err
	}

	containerScales, err := c.listers.ContainerScaleLister.List(labels.Everything())
	if err != nil {
		klog.Error("Failed to retrieve the container scales, error: ", err)
		return err
	}

	// Filter all pod scales and pods matched by the sla
	var matchedContainerScales []*v1beta1.ContainerScale
	var matchedPods []*corev1.Pod
	for _, containerScale := range containerScales {
		if containerScale.Spec.SLARef.Namespace == sla.Namespace && containerScale.Spec.SLARef.Name == sla.Name {
			matchedContainerScales = append(matchedContainerScales, containerScale)
			pod, err := c.listers.Pods(containerScale.Spec.PodRef.Namespace).Get(containerScale.Spec.PodRef.Name)
			if err != nil {
				klog.Error("Failed to retrieve the pod, error: ", err)
				return err
			} else {
				matchedPods = append(matchedPods, pod)
			}
		}
	}

	// Retrieve the metrics for the pods
	var podMetrics []map[string]interface{}
	for _, pod := range matchedPods {
		metrics, err := c.MetricClient.getMetrics(pod)
		if err != nil {
			klog.Errorf("Failed to retrieve the metrics for pod with name %s and namespace %s, error: %s. Probably the pod is not ready yet, retrying", pod.Name, pod.Namespace, err)
			return err
		} else {
			podMetrics = append(podMetrics, metrics)
		}
	}

	// TODO: handle also statefulset, replicaset, etc. (all types of 'apps')
	// Check that all pods are in the same namespace and retrieve the deployment of the pods
	var namespace = ""
	var deploymentName string
	for _, pod := range matchedPods {
		if namespace == "" {
			namespace = pod.Namespace
			replicaSetName, err := getReplicaSetNameFromPod(pod)
			if err != nil {
				klog.Errorf("Failed to retrieve the name of the replicaset for pod with name %s and namespace %s, error: %s.", pod.Name, pod.Namespace, err)
				return err
			}
			replicaSet, err := c.kubernetesClientset.AppsV1().ReplicaSets(namespace).Get(context.TODO(), replicaSetName, v1.GetOptions{})
			if err != nil {
				klog.Errorf("Failed to retrieve the replicaset with name %s and namespace %s, error: %s.", replicaSetName, namespace, err)
				return err
			}
			deploymentName, err = getDeploymentNameFromReplicaSet(replicaSet)
			if err != nil {
				klog.Errorf("Failed to retrieve the name of the deployment for replicaset with name %s and namespace %s, error: %s.", replicaSet.Name, replicaSet.Namespace, err)
				return err
			}
		} else if namespace != pod.Namespace {
			klog.Error("The pods are not in the same namespace")
			return err
		}
	}

	// Retrieve the deployment
	deployment, err := c.kubernetesClientset.AppsV1().Deployments(namespace).Get(context.TODO(), deploymentName, v1.GetOptions{})
	if err != nil {
		klog.Errorf("Failed to retrieve the deployment with name %s and namespace %s, error: %s.", deploymentName, namespace, err)
		return err
	}

	// Retrieve the associated logic
	logicInterface, ok := c.logicMap.LoadOrStore(key, newHPALogic())
	if !ok {
		klog.Errorf("the key %s has no previous logic associated with it, initializing it", key)
		return err
	}
	logic, ok := logicInterface.(Logic)
	if !ok {
		klog.Errorf("error: %s, failed to cast logic for deployment with name %s and namespace %s", err, deploymentName, namespace)
		return err
	}

	// Compute the new amount of replicas
	nReplicas := logic.computeReplica(sla, matchedPods, matchedContainerScales, podMetrics, *deployment.Spec.Replicas)
	klog.Info("SLA key: ", key, " new amount of replicas: ", nReplicas)

	// Set the new amount of replicas
	deployment.Spec.Replicas = &nReplicas
	_, err = c.kubernetesClientset.AppsV1().Deployments(namespace).Update(context.TODO(), deployment, v1.UpdateOptions{})
	if err != nil {
		klog.Errorf("Failed to update the deployment with name %s and namespace %s, error: %s.", deploymentName, namespace, err)
		return err
	}

	return nil
}