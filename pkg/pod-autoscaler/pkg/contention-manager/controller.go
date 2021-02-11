package contentionmanager

import (
	"fmt"
	"github.com/lterrac/system-autoscaler/pkg/informers"
	"time"

	corev1 "k8s.io/api/core/v1"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"

	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"

	"github.com/lterrac/system-autoscaler/pkg/containerscale-controller/pkg/types"
	clientset "github.com/lterrac/system-autoscaler/pkg/generated/clientset/versioned"
	samplescheme "github.com/lterrac/system-autoscaler/pkg/generated/clientset/versioned/scheme"
)

// AgentName is the controller name used
// both in logs and labels to identify it
const AgentName = "contention-manager"

// Controller is responsible of adjusting containerscale partially computed by the recommender
// taking into account the actual node capacity
type Controller struct {
	kubeClientset            kubernetes.Interface
	containerScalesClientset clientset.Interface

	listers informers.Listers

	containerScalesSynced cache.InformerSynced
	nodesSynced           cache.InformerSynced

	recorder record.EventRecorder

	in  chan types.NodeScales
	out chan types.NodeScales
}

// NewController returns a new ContainerScale controller
func NewController(
	kubeClient kubernetes.Interface,
	containerScalesClient clientset.Interface,
	informers informers.Informers,
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
		kubeClientset:            kubeClient,
		containerScalesClientset: containerScalesClient,

		listers: informers.GetListers(),

		containerScalesSynced: informers.ContainerScale.Informer().HasSynced,
		nodesSynced:           informers.Node.Informer().HasSynced,

		recorder: recorder,

		in:  in,
		out: out,
	}

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting contention manager controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh,
		c.containerScalesSynced,
		c.nodesSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Starting contention manager  workers")
	// Launch two workers to process containerScale resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	klog.Info("Started contention manager  workers")

	return nil
}

// Shutdown stops the contention manager
func (c *Controller) Shutdown() {
	utilruntime.HandleCrash()
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextNode(c.in) {
	}
}
