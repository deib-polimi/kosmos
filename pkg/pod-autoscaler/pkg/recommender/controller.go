package recommender

import (
	"fmt"
	"time"

	"github.com/lterrac/system-autoscaler/pkg/informers"
	"github.com/lterrac/system-autoscaler/pkg/queue"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/lterrac/system-autoscaler/pkg/apis/systemautoscaler/v1beta1"
	"github.com/lterrac/system-autoscaler/pkg/containerscale-controller/pkg/types"
	"github.com/modern-go/concurrent"

	corev1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"

	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"

	containerscalesclientset "github.com/lterrac/system-autoscaler/pkg/generated/clientset/versioned"
	samplescheme "github.com/lterrac/system-autoscaler/pkg/generated/clientset/versioned/scheme"
)

const controllerAgentName = "recommender"
const responseTime = "response_time"

// Controller is the controller that recommends resources to the pods.
// For each Pod Scale assigned to the recommender, it will have a pod saved in a list.
// Every x seconds, the recommender polls the metrics from the pod by using an http request.
// For pod metrics it retrieves, it computes the new resources to assign to the pod.
type Controller struct {

	// containerScalesClientset is a clientset for our own API group
	containerScalesClientset containerscalesclientset.Interface

	listers informers.Listers

	containerScalesSynced cache.InformerSynced

	// kubernetesCLientset is the client-go of kubernetes
	kubernetesClientset kubernetes.Interface

	// recommendNodeQueue contains all the nodes that needs a recommendation
	recommendNodeQueue queue.Queue

	// status represents the state of the controller
	status *Status

	// MetricGetter is a client that polls the metrics from the pod.
	MetricGetter MetricGetter

	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder

	// out is the output channel of the recommender.
	out chan types.NodeScales
}

// Status represents the state of the controller
type Status struct {
	// Key: namespace-name of the pod scale, Value: assigned logic
	logicMap concurrent.Map
}

// NewController returns a new recommender
func NewController(
	kubernetesClientset kubernetes.Interface,
	containerScalesClientset containerscalesclientset.Interface,
	metricsClient MetricGetter,
	informers informers.Informers,
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
		logicMap: *concurrent.NewMap(),
	}

	// Instantiate the Controller
	controller := &Controller{
		containerScalesClientset: containerScalesClientset,
		listers:                  informers.GetListers(),
		containerScalesSynced:    informers.ContainerScale.Informer().HasSynced,
		kubernetesClientset:      kubernetesClientset,
		recommendNodeQueue:       queue.NewQueue("RecommendQueue"),
		status:                   status,
		MetricGetter:             metricsClient,
		recorder:                 recorder,
		out:                      out,
	}

	klog.Info("Setting up event handlers")

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
	if ok := cache.WaitForCacheSync(stopCh, c.containerScalesSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Starting recommender workers")
	// Launch the workers to process containerScale resources and recommendContainer new pod scales
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runNodeRecommenderWorker, time.Second, stopCh)
	}
	go wait.Until(c.runRecommenderWorker, 5*time.Second, stopCh)
	klog.Info("Started recommender workers")

	return nil
}

// Shutdown gracefully terminates the controller
func (c *Controller) Shutdown() {
	utilruntime.HandleCrash()
	c.recommendNodeQueue.ShutDown()
}

// Enqueue a node to the recommend node queue
func (c *Controller) runRecommenderWorker() {

	nodes, err := c.listers.NodeLister.List(labels.Everything())
	if err != nil {
		klog.Error(err)
		return
	}

	for _, node := range nodes {
		c.recommendNodeQueue.Enqueue(node)
	}
}

// TODO: make better comment. It is not very clear.
// Handle all the nodes that needs a recommendation
func (c *Controller) runNodeRecommenderWorker() {
	for c.recommendNodeQueue.ProcessNextItem(c.recommendNode) {
	}
}

// recommendNode recommends new resources to a set of pods that are deployed on the same node.
// it writes on the out channel a node scale which contains all the pods that are monitored on the node.
func (c *Controller) recommendNode(node string) error {
	// Recommend to all pods in a node new pod scales resources.
	klog.Info("Recommending to node ", node)

	newContainerScales := make([]*v1beta1.ContainerScale, 0)

	listSelector := labels.Set(map[string]string{"system.autoscaler/node": node}).AsSelector()

	containerscales, err := c.listers.ContainerScaleLister.List(listSelector)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("list pod scales failed: %s", err))
		return nil
	}

	if len(containerscales) == 0 {
		utilruntime.HandleError(fmt.Errorf("no containerscales found on node: %s", node))
		return nil
	}

	for _, containerscale := range containerscales {
		newContainerScale, err := c.recommendContainer(containerscale)
		if err != nil {
			//utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
			// TODO: evaluate if we should use a 'continue'
			klog.Info(err)
			return err
		}
		newContainerScales = append(newContainerScales, newContainerScale)
	}

	nodeScales := types.NodeScales{
		Node:            node,
		ContainerScales: newContainerScales,
	}

	// Send to output channel.
	// The contention manager will handle the new pod scales of the node.
	c.out <- nodeScales
	return nil
}

// recommendContainer recommends the new resources to assign to a pod
func (c *Controller) recommendContainer(containerScale *v1beta1.ContainerScale) (*v1beta1.ContainerScale, error) {
	key := fmt.Sprintf("%s/%s", containerScale.Namespace, containerScale.Name)

	klog.Info("Recommending for ", key)

	// Get the pod associated with the pod scale
	pod, err := c.listers.Pods(containerScale.Spec.PodRef.Namespace).Get(containerScale.Spec.PodRef.Name)
	if err != nil {
		return nil, fmt.Errorf("error: %s, cannot retrieve pod with name %s and namespace %s", err, containerScale.Spec.PodRef.Name, containerScale.Spec.PodRef.Namespace)
	}

	// Retrieve the sla
	sla, err := c.listers.ServiceLevelAgreements(containerScale.Spec.SLARef.Namespace).Get(containerScale.Spec.SLARef.Name)
	if err != nil {
		//utilruntime.HandleError(fmt.Errorf("error: %s, failed to get sla with name %s and namespace %s from lister", err, containerScale.Spec.SLARef.Name, containerScale.Spec.SLARef.Namespace))
		return nil, err
	}

	// Retrieve the logic
	logicInterface, ok := c.status.logicMap.LoadOrStore(key, newControlTheoryLogic(containerScale))
	if !ok {
		return nil, fmt.Errorf("the key %s has no logic associated with it", key)
	}
	logic, ok := logicInterface.(Logic)
	if !ok {
		return nil, fmt.Errorf("error: %s, failed to cast logic with name %s and namespace %s", err, containerScale.Spec.SLARef.Name, containerScale.Spec.SLARef.Namespace)
	}

	// Retrieve the metrics
	metrics, err := c.MetricGetter.GetMetrics(pod)
	if err != nil {
		return nil, fmt.Errorf("error: %s, failed to get metrics from pod with name %s and namespace %s from lister", err, pod.GetName(), pod.GetNamespace())
	}

	// Compute the new resources
	newContainerScale, err := logic.computeContainerScale(pod, containerScale, sla, metrics)

	return newContainerScale, nil
}
