package recommender

import (
	"context"
	"fmt"
	"github.com/modern-go/concurrent"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"

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

const (
	// SuccessSynced is used as part of the Event 'reason' when a podScale is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a podScale fails
	// to sync due to a Deployment of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Deployment already existing
	MessageResourceExists = "Resource %q already exists and is not managed by podScale"
	// MessageResourceSynced is the message used for an Event fired when a podScale
	// is synced successfully
	MessageResourceSynced = "podScale synced successfully"
)

// Controller is the controller that recommends resources to the pods.
// For each Pod Scale assigned to the recommender, it will have a pod saved in a list.
// Every x seconds, the recommender polls the metrics from the pod by using an http request.
// For pod metrics it retrieves, it computes the new resources to assign to the pod.
type Controller struct {

	// podScalesClientset is a clientset for our own API group
	podScalesClientset podscalesclientset.Interface

	podScalesLister listers.PodScaleLister

	podScalesSynced cache.InformerSynced

	// kubernetesCLientset is the client-go of kubernetes
	kubernetesClientset kubernetes.Clientset

	// podScalesAdded contains all the pods that should be monitored
	podScalesAdded workqueue.RateLimitingInterface

	// podScalesDeleted contains all the pods that should not be monitored
	podScalesDeleted workqueue.RateLimitingInterface

	// podsMap is a map. The keys are a set a of namaspace/name of the podscale and the values are the pods.
	podsMap concurrent.Map

	// metricPoller is a client that polls the metrics from the pod.
	metricPoller Client

	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
}

// NewController returns a new sample controller
func NewController(
	podScalesClientset podscalesclientset.Interface,
	podScaleInformer informers.PodScaleInformer) *Controller {

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
		podScalesClientset: podScalesClientset,
		podScalesLister:    podScaleInformer.Lister(),
		podScalesSynced:    podScaleInformer.Informer().HasSynced,
		podScalesAdded:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "PodScalesAdded"),
		podScalesDeleted:   workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "PodScalesDeleted"),
		podsMap:            *concurrent.NewMap(),
		metricPoller:       NewMetricClient(),
		recorder:           recorder,
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
	defer utilruntime.HandleCrash()
	defer c.podScalesAdded.ShutDown()
	defer c.podScalesDeleted.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting podScale controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.podScalesSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Starting workers")
	// Launch two workers to process podScale resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runPodScaleAddedWorker, time.Second, stopCh)
		go wait.Until(c.runPodScaleRemovedWorker, time.Second, stopCh)
		go wait.Until(c.runRecommenderWorker, 5*time.Second, stopCh)
	}

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

func (c *Controller) runPodScaleAddedWorker() {
	for c.processPodScalesAdded() {
	}
}

func (c *Controller) runPodScaleRemovedWorker() {
	for c.processPodScalesAdded() {
	}
}

func (c *Controller) runRecommenderWorker() {
	for c.recommend() {
	}
}

func (c *Controller) recommend() bool {

	c.podsMap.Range(func(key, value interface{}) bool {
		// Retrieve the pod scale namespace and name
		namespace, name, err := cache.SplitMetaNamespaceKey(key.(string))
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
			return true
		}
		// Get the PodScale resource with this namespace/name
		podScale, err := c.podScalesLister.PodScales(namespace).Get(name)
		if err != nil {
			klog.Error(err)
		}
		// Retrieve the pod
		pod := value.(*corev1.Pod)
		metrics, err := c.metricPoller.GetMetrics(pod)
		if err != nil {
			klog.Error(err)
		}
		newPodScale := c.computePodScale(pod, podScale, metrics)

		// Update the PodScale
		_, err = c.podScalesClientset.SystemautoscalerV1beta1().PodScales(podScale.Namespace).Update(context.TODO(), newPodScale, metav1.UpdateOptions{})
		if err != nil {
			klog.Error(err)
		}

		return true
	})
	return true
}
