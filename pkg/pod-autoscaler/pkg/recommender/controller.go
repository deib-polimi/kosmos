package recommender

import (
	"context"
	"fmt"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"

	"github.com/lterrac/system-autoscaler/pkg/apis/systemautoscaler/v1beta1"
	"github.com/lterrac/system-autoscaler/pkg/podscale-controller/pkg/types"
	"github.com/modern-go/concurrent"

	corev1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"

	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	podscalesclientset "github.com/lterrac/system-autoscaler/pkg/generated/clientset/versioned"
	samplescheme "github.com/lterrac/system-autoscaler/pkg/generated/clientset/versioned/scheme"
	informers "github.com/lterrac/system-autoscaler/pkg/generated/informers/externalversions/systemautoscaler/v1beta1"
	listers "github.com/lterrac/system-autoscaler/pkg/generated/listers/systemautoscaler/v1beta1"
)

const controllerAgentName = "recommender"

// Controller is the controller that recommends resources to the pods.
// For each Pod Scale assigned to the recommender, it will have a pod saved in a list.
// Every x seconds, the recommender polls the metrics from the pod by using an http request.
// For pod metrics it retrieves, it computes the new resources to assign to the pod.
type Controller struct {

	// podScalesClientset is a clientset for our own API group
	podScalesClientset podscalesclientset.Interface

	podScalesInformer informers.PodScaleInformer

	podScalesLister listers.PodScaleLister

	podScalesSynced cache.InformerSynced

	slaLister listers.ServiceLevelAgreementLister

	// kubernetesCLientset is the client-go of kubernetes
	kubernetesClientset kubernetes.Clientset

	// TODO: we have three queues? Do we really need all of them?
	// podScalesAddedQueue contains all the pods that should be monitored
	podScalesAddedQueue workqueue.RateLimitingInterface

	// podScalesDeletedQueue contains all the pods that should not be monitored
	podScalesDeletedQueue workqueue.RateLimitingInterface

	// recommendNodeQueue contains all the nodes that needs a recommendation
	recommendNodeQueue workqueue.RateLimitingInterface

	// status represents the state of the controller
	status *Status

	// metricPoller is a client that polls the metrics from the pod.
	metricPoller *Client

	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder

	// out is the output channel of the recommender.
	out chan types.NodeScales
}

// Status represents the state of the controller
type Status struct {

	// Key: NodeName, Value: namespace-name of the pod scale
	nodeMap concurrent.Map

	// Key: namespace-name of the pod scale, Value: assigned pod
	podMap concurrent.Map

	// Key: namespace-name of the pod scale, Value: assigned logic
	logicMap concurrent.Map
}

// NewController returns a new recommender
func NewController(
	kubernetesClientset *kubernetes.Clientset,
	podScalesClientset podscalesclientset.Interface,
	podScaleInformer informers.PodScaleInformer,
	slaInformer informers.ServiceLevelAgreementInformer,
	out chan types.NodeScales,
) *Controller {

	// Create event broadcaster
	// Add sample-controller types to the default Kubernetes Scheme so Events can be
	// logged for sample-controller types.
	utilruntime.Must(samplescheme.AddToScheme(scheme.Scheme))
	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	// Create Controller status
	status := &Status{
		nodeMap:  *concurrent.NewMap(),
		podMap:   *concurrent.NewMap(),
		logicMap: *concurrent.NewMap(),
	}

	// Instantiate the Controller
	controller := &Controller{
		podScalesClientset:    podScalesClientset,
		podScalesInformer:     podScaleInformer,
		podScalesLister:       podScaleInformer.Lister(),
		slaLister:             slaInformer.Lister(),
		podScalesSynced:       podScaleInformer.Informer().HasSynced,
		kubernetesClientset:   *kubernetesClientset,
		podScalesAddedQueue:   workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "PodScalesAdded"),
		podScalesDeletedQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "PodScalesDeleted"),
		recommendNodeQueue:    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "RecommendQueue"),
		status:                status,
		metricPoller:          NewMetricClient(),
		recorder:              recorder,
		out:                   out,
	}

	klog.Info("Setting up event handlers")

	// Set up an event handler for when podScale resources change
	podScaleInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.enqueuePodScaleAdded,
		UpdateFunc: controller.enqueuePodScaleUpdated,
		DeleteFunc: controller.enqueuePodScaleDeleted,
	})
	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting recommender controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.podScalesSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Starting recommender workers")
	// Launch the workers to process podScale resources and recommend new pod scales
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runPodScaleAddedWorker, time.Second, stopCh)
		go wait.Until(c.runPodScaleRemovedWorker, time.Second, stopCh)
		go wait.Until(c.runRecommendNodeWorker, time.Second, stopCh)
	}
	go wait.Until(c.runRecommenderWorker, 5*time.Second, stopCh)

	klog.Info("Started recommender workers")

	return nil
}

func (c *Controller) Shutdown() {
	utilruntime.HandleCrash()
	c.podScalesAddedQueue.ShutDown()
	c.podScalesDeletedQueue.ShutDown()
	c.recommendNodeQueue.ShutDown()
}

// Handle all the pod scales that has been added
func (c *Controller) runPodScaleAddedWorker() {
	for c.processPodScalesAdded() {
	}
}

// Handle all the pod scales that has been deleted
func (c *Controller) runPodScaleRemovedWorker() {
	for c.processPodScalesDeleted() {
	}
}

// Enqueue a node to the recommend node queue
func (c *Controller) runRecommenderWorker() {
	c.status.nodeMap.Range(func(key, value interface{}) bool {
		c.recommendNodeQueue.Add(key)
		return true
	})
}

// TODO: the name is not auto-explicative
// Handle all the nodes that needs a recommendation
func (c *Controller) runRecommendNodeWorker() {
	for c.processRecommendNode() {
	}
}

func (c *Controller) recommendNode(node string) error {
	// Recommend to all pods in a node new pod scales resources.
	klog.Info("Recommending to node ", node)

	result, present := c.status.nodeMap.Load(node)
	if !present {
		return fmt.Errorf("the node does not have any pod scale associated with it")
	}
	keys, ok := result.(map[string]struct{})
	if !ok {
		return fmt.Errorf("unable to cast the podscale keys from node map")
	}

	newPodScales := make([]*v1beta1.PodScale, 0)
	for key := range keys {
		newPodScale, err := c.recommend(key)
		if err != nil {
			//utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
			return err
		}
		newPodScales = append(newPodScales, newPodScale)
	}

	nodeScales := types.NodeScales{
		Node:      node,
		PodScales: newPodScales,
	}

	// Send to output channel.
	// The contention manager will handle the new pod scales of the node.
	c.out <- nodeScales
	return nil
}

// TODO: need to document it
func (c *Controller) recommend(key string) (*v1beta1.PodScale, error) {
	klog.Info("Recommending for ", key)

	// Retrieve the pod scale namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		//utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil, err
	}

	// Get the PodScale resource with this namespace/name
	podScale, err := c.podScalesLister.PodScales(namespace).Get(name)
	if err != nil {
		//utilruntime.HandleError(fmt.Errorf("error: %s, failed to get pod scale with name %s and namespace %s from lister", err, name, namespace))
		return nil, err
	}

	// Retrieve the pod
	//podInterface, ok := c.status.podMap.Load(key)
	//if !ok {
	//	return nil, fmt.Errorf("the key %s has no pod associated with it", key)
	//}
	//pod, ok := podInterface.(*corev1.Pod)
	//if !ok {
	//	return nil, fmt.Errorf("error: %s, failed to cast logic with name %s and namespace %s", err, podScale.Spec.SLARef.Name, podScale.Spec.SLARef.Namespace)
	//}
	// Get the pod associated with the pod scale
	// TODO: the pod should be taken from the lister if possible.
	pod, err := c.kubernetesClientset.CoreV1().Pods(podScale.Spec.PodRef.Namespace).Get(context.TODO(), podScale.Spec.PodRef.Name, v1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error: %s, cannot retrieve pod with name %s and namespace %s", err, podScale.Spec.PodRef.Name, podScale.Spec.PodRef.Namespace)
	}
	// TODO: the pod should be taken from the lister if possible.
	// Retrieve the sla
	sla, err := c.slaLister.ServiceLevelAgreements(podScale.Spec.SLARef.Namespace).Get(podScale.Spec.SLARef.Name)
	if err != nil {
		//utilruntime.HandleError(fmt.Errorf("error: %s, failed to get sla with name %s and namespace %s from lister", err, podScale.Spec.SLARef.Name, podScale.Spec.SLARef.Namespace))
		return nil, err
	}

	// Retrieve the logic
	logicInterface, ok := c.status.logicMap.Load(key)
	if !ok {
		return nil, fmt.Errorf("the key %s has no logic associated with it", key)
	}
	logic, ok := logicInterface.(Logic)
	if !ok {
		return nil, fmt.Errorf("error: %s, failed to cast logic with name %s and namespace %s", err, podScale.Spec.SLARef.Name, podScale.Spec.SLARef.Namespace)
	}

	// Retrieve the metrics
	metrics, err := c.metricPoller.getMetrics(pod)
	if err != nil {
		return nil, fmt.Errorf("error: %s, failed to get metrics from pod with name %s and namespace %s from lister", err, pod.GetName(), pod.GetNamespace())
	}

	// Compute the new resources
	newPodScale := logic.computePodScale(pod, podScale, sla, metrics)

	return newPodScale, nil

}
