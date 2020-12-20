package controller

import (
	"fmt"
	"time"

	"github.com/lterrac/system-autoscaler/pkg/informers"
	"github.com/lterrac/system-autoscaler/pkg/queue"
	corev1 "k8s.io/api/core/v1"
	typev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"

	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"

	clientset "github.com/lterrac/system-autoscaler/pkg/generated/clientset/versioned"
	samplescheme "github.com/lterrac/system-autoscaler/pkg/generated/clientset/versioned/scheme"
)

const (

	// AgentName is the controller name used
	// both in logs and labels to identify it
	AgentName = "podscale-controller"

	// SubjectToLabel is used to identify the ServiceLevelAgreement
	// matched by the Service
	SubjectToLabel = "app.kubernetes.io/subject-to"

	// QOSNotSupported is used as part of the event 'reason' fired when the controller
	// process a pod with a QOS other than Guaranteed
	QOSNotSupported = "Unsupported QOS"

	// SuccessSynced is used as part of the Event 'reason' when a podScale is synced
	SuccessSynced = "Synced"

	// MessageResourceSynced is the message used for an Event fired when a podScale
	// is synced successfully
	MessageResourceSynced = "podScale synced successfully"
)

// Controller is the controller implementation for podScale resources
type Controller struct {
	kubeClientset      kubernetes.Interface
	podScalesClientset clientset.Interface

	listers informers.Listers

	slasSynced      cache.InformerSynced
	podScalesSynced cache.InformerSynced
	servicesSynced  cache.InformerSynced
	podSynced       cache.InformerSynced

	slasworkqueue queue.Queue

	recorder record.EventRecorder
}

// NewController returns a new PodScale controller
func NewController(
	kubeClient kubernetes.Interface,
	podScalesClient clientset.Interface,
	informers informers.Informers) *Controller {

	// Create event broadcaster
	// Add System Autoscaler types to the default Kubernetes Scheme so Events can be
	// logged for types.
	utilruntime.Must(samplescheme.AddToScheme(scheme.Scheme))
	klog.V(4).Info("Creating event broadcaster")

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&typev1.EventSinkImpl{
		Interface: kubeClient.CoreV1().Events(corev1.NamespaceAll),
	})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: AgentName})

	controller := &Controller{
		kubeClientset:      kubeClient,
		podScalesClientset: podScalesClient,

		listers: informers.GetListers(),

		slasSynced:      informers.ServiceLevelAgreement.Informer().HasSynced,
		podScalesSynced: informers.PodScale.Informer().HasSynced,
		servicesSynced:  informers.Service.Informer().HasSynced,
		podSynced:       informers.Pod.Informer().HasSynced,

		slasworkqueue: queue.NewQueue("ServiceLevelAgreements"),
		recorder:      recorder,
	}

	klog.Info("Setting up event handlers")
	// Set up an event handler for when ServiceLevelAgreements resources change
	informers.ServiceLevelAgreement.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.handleServiceLevelAgreementAdd,
		UpdateFunc: controller.handleServiceLevelAgreementUpdate,
		DeleteFunc: controller.handleServiceLevelAgreementDeletion,
	})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.slasworkqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting podScale controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh,
		c.slasSynced,
		c.servicesSynced,
		c.podScalesSynced,
		c.podSynced); !ok {
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
	for c.slasworkqueue.ProcessNextItem(c.syncServiceLevelAgreement) {
	}
}
