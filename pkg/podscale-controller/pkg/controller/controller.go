package controller

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
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	clientset "github.com/lterrac/system-autoscaler/pkg/generated/clientset/versioned"
	samplescheme "github.com/lterrac/system-autoscaler/pkg/generated/clientset/versioned/scheme"
	informers "github.com/lterrac/system-autoscaler/pkg/generated/informers/externalversions/systemautoscaler/v1beta1"
	listers "github.com/lterrac/system-autoscaler/pkg/generated/listers/systemautoscaler/v1beta1"
)

// ControllerAgentName is the controller name used
// both in logs and labels to identify it
const AgentName = "podscale-controller"

// SubjectToLabel is used to identify the ServiceLevelAgreement
// matched by the Service
const SubjectToLabel = "app.kubernetes.io/subject-to"

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

// Controller is the controller implementation for podScale resources
type Controller struct {
	// podScalesClientset is a clientset for our own API group
	kubeClientset      kubernetes.Interface
	podScalesClientset clientset.Interface

	servicesLister corelisters.ServiceLister
	servicesSynced cache.InformerSynced
	slasLister     listers.ServiceLevelAgreementLister
	slasSynced     cache.InformerSynced
	// podScalesLister listers.PodScaleLister
	// podScalesSynced cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.

	servicesworkqueue workqueue.RateLimitingInterface
	slasworkqueue     workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
}

// NewController returns a new PodScale controller
func NewController(
	kubeClient kubernetes.Interface,
	podScalesClient clientset.Interface,
	serviceInformer coreinformers.ServiceInformer,
	slaInformer informers.ServiceLevelAgreementInformer,
	podScaleInformer informers.PodScaleInformer) *Controller {

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
		servicesLister:     serviceInformer.Lister(),
		slasLister:         slaInformer.Lister(),
		// podScalesLister:    podScaleInformer.Lister(),
		servicesSynced: serviceInformer.Informer().HasSynced,
		slasSynced:     slaInformer.Informer().HasSynced,
		// podScalesSynced:    podScaleInformer.Informer().HasSynced,
		servicesworkqueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Services"),
		slasworkqueue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ServiceLevelAgreements"),
		recorder:          recorder,
	}

	klog.Info("Setting up event handlers")
	// Set up an event handler for when ServiceLevelAgreements resources change
	slaInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueSLA,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueSLA(new)
		},
		DeleteFunc: controller.enqueueSLA,
	})

	//// Set up an event handler for when Services resources change
	//serviceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
	//	AddFunc: controller.enqueueService,
	//	UpdateFunc: func(old, new interface{}) {
	//		controller.enqueueService(new)
	//	},
	//	DeleteFunc: controller.enqueueService,
	//})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.servicesworkqueue.ShutDown()
	defer c.slasworkqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting podScale controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.slasSynced, c.servicesSynced); !ok {
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
	for c.processNextQueueItem(c.slasworkqueue, c.syncSLAHandler){
	}
}

//c.processNextQueueItem(c.servicesworkqueue, c.syncServiceHandler)
