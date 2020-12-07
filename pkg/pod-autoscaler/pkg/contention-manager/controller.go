package contentionmanager

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"

	corelisters "k8s.io/client-go/listers/core/v1"

	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"

	clientset "github.com/lterrac/system-autoscaler/pkg/generated/clientset/versioned"
	samplescheme "github.com/lterrac/system-autoscaler/pkg/generated/clientset/versioned/scheme"
	informers "github.com/lterrac/system-autoscaler/pkg/generated/informers/externalversions/systemautoscaler/v1beta1"
	listers "github.com/lterrac/system-autoscaler/pkg/generated/listers/systemautoscaler/v1beta1"
	"github.com/lterrac/system-autoscaler/pkg/podscale-controller/pkg/types"
)

// AgentName is the controller name used
// both in logs and labels to identify it
const AgentName = "contention-manager"


// Controller is responsible of adjusting podscale partially computed by the recommender
// taking into account the actual node capacity
type Controller struct {
	kubeClientset      kubernetes.Interface
	podScalesClientset clientset.Interface

	podScalesLister listers.PodScaleLister
	nodeLister      corelisters.NodeLister

	podScalesSynced cache.InformerSynced
	nodesSynced     cache.InformerSynced

	recorder record.EventRecorder

	in  chan types.NodeScales
	out chan types.NodeScales
}

// NewController returns a new PodScale controller
func NewController(
	kubeClient kubernetes.Interface,
	podScalesClient clientset.Interface,
	podScaleInformer informers.PodScaleInformer,
	nodeInformer coreinformers.NodeInformer,
	in chan types.NodeScales,
	out chan types.NodeScales) *Controller {

	// Create event broadcaster
	// Add System Autoscaler types to the default Kubernetes Scheme so Events can be
	// logged for types.
	utilruntime.Must(samplescheme.AddToScheme(scheme.Scheme))
	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: AgentName})

	controller := &Controller{
		kubeClientset:      kubeClient,
		podScalesClientset: podScalesClient,

		podScalesLister: podScaleInformer.Lister(),
		nodeLister:      nodeInformer.Lister(),

		podScalesSynced: podScaleInformer.Informer().HasSynced,
		nodesSynced:     nodeInformer.Informer().HasSynced,

		recorder: recorder,

		in: in,
		out: out,
	}

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting contention manager")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh,
		c.podScalesSynced,
		c.nodesSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Starting workers")
	// Launch two workers to process podScale resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextNode(c.in) {
	}
}
