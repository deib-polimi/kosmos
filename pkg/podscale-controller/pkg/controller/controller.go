package recommender

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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

	"github.com/lterrac/system-autoscaler/pkg/apis/systemautoscaler/v1beta1"
	clientset "github.com/lterrac/system-autoscaler/pkg/generated/clientset/versioned"
	samplescheme "github.com/lterrac/system-autoscaler/pkg/generated/clientset/versioned/scheme"
	informers "github.com/lterrac/system-autoscaler/pkg/generated/informers/externalversions/systemautoscaler/v1beta1"
	listers "github.com/lterrac/system-autoscaler/pkg/generated/listers/systemautoscaler/v1beta1"
)

// ControllerAgentName is the controller name used
// both in logs and labels to identify it
const ControllerAgentName = "podscale-controller"

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

	servicesLister  corelisters.ServiceLister
	servicesSynced  cache.InformerSynced
	slasLister      listers.ServiceLevelAgreementLister
	slasSynced      cache.InformerSynced
	podScalesLister listers.PodScaleLister
	podScalesSynced cache.InformerSynced

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
	podScalesClientset clientset.Interface,
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
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: ControllerAgentName})

	controller := &Controller{
		kubeClientset:      kubeClient,
		podScalesClientset: podScalesClientset,
		servicesLister:     serviceInformer.Lister(),
		slasLister:         slaInformer.Lister(),
		podScalesLister:    podScaleInformer.Lister(),
		servicesSynced:     serviceInformer.Informer().HasSynced,
		slasSynced:         slaInformer.Informer().HasSynced,
		podScalesSynced:    podScaleInformer.Informer().HasSynced,
		servicesworkqueue:  workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Services"),
		slasworkqueue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ServiceLevelAgreements"),
		recorder:           recorder,
	}

	klog.Info("Setting up event handlers")
	// Set up an event handler for when ServiceLevelAgreements resources change
	slaInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueSLA,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueSLA(new)
		},
		DeleteFunc: func(obj interface{}) {
			controller.enqueueSLA(obj)
		},
	})

	// Set up an event handler for when Services resources change
	serviceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueService,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueService(new)
		},
		DeleteFunc: func(obj interface{}) {
			controller.enqueueService(obj)
		},
	})

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
	if ok := cache.WaitForCacheSync(stopCh, c.servicesSynced, c.slasSynced, c.podScalesSynced); !ok {
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
	for c.processNextSLAItem() {
	}
}

// processNextSLAItem will read a single work service item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextSLAItem() bool {
	sla, shutdown := c.slasworkqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		defer c.slasworkqueue.Done(obj)

		var slaKey string
		var ok bool

		if slaKey, ok = obj.(string); !ok {
			c.slasworkqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in services workqueue but got %#v", obj))
			return nil
		}

		// Run the syncHandler, passing it the namespace/name string of the
		// podScale resource to be synced.
		if err := c.syncSLAHandler(slaKey); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.slasworkqueue.AddRateLimited(slaKey)
			return fmt.Errorf("error syncing '%s': %s, requeuing", slaKey, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.slasworkqueue.Forget(obj)
		klog.Infof("Successfully synced '%s'", slaKey)
		return nil
	}(sla)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two.
func (c *Controller) syncSLAHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the SLA resource with this namespace/name
	sla, err := c.slasLister.ServiceLevelAgreements(namespace).Get(name)
	if err != nil {
		// The SLA resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("ServiceLevelAgreement '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	services, err := c.servicesLister.Services(namespace).List(labels.NewSelector())

	if err != nil {
		utilruntime.HandleError(fmt.Errorf("error while getting Services in namespace '%s'", namespace))
		return nil
	}

	for _, service := range services {
		// Do nothing if the service is already tracked by the controller
		_, ok := service.Labels[SubjectToLabel]

		// TODO: Decide what happens if service matches a SLA but already have one
		if ok {
			klog.V(4).Infof("Service %s is already tracked by %s", service.GetName(), name)
			return nil
		}

		set := labels.Set(service.Spec.Selector)

		pods, err := c.kubeClientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: set.AsSelector().String()})

		if err != nil {
			utilruntime.HandleError(fmt.Errorf("error while getting Pods for Service '%s'", service.GetName()))
			return nil
		}

		podscales, err := c.podScalesClientset.SystemautoscalerV1beta1().PodScales(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: set.AsSelector().String()})

		if err != nil {
			utilruntime.HandleError(fmt.Errorf("error while getting PodScales for Service '%s'", service.GetName()))
			return nil
		}

		var orphanPods []corev1.Pod

		orphanPods = podDiff(podscales.Items, pods.Items)

		klog.V(4).Info("Adding PodScale to: ", orphanPods)

		for _, orphan := range orphanPods {
			podscale := &v1beta1.PodScale{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "systemautoscaler.polimi.it/v1beta1",
					Kind:       "PodScale",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "podscale-" + orphan.GetName(),
					Namespace: namespace,
				},
				Spec: v1beta1.PodScaleSpec{
					SLA:              sla.GetName(),
					Pod:              orphan.GetName(),
					DesiredResources: sla.Spec.DefaultResources,
				},
				Status: v1beta1.PodScaleStatus{
					ActualResources: sla.Spec.DefaultResources,
				},
			}

			_, err := c.podScalesClientset.SystemautoscalerV1beta1().PodScales(namespace).Create(context.TODO(), podscale, metav1.CreateOptions{})
			if err != nil {
				utilruntime.HandleError(fmt.Errorf("error while creating PodScale for Pod '%s'", orphan.GetName()))
				utilruntime.HandleError(err)
				return nil
			}
		}

		service.Labels[SubjectToLabel] = name
		_, err = c.kubeClientset.CoreV1().Services(namespace).Update(context.TODO(), service, metav1.UpdateOptions{})
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("error while updateing Service labels of '%s'", service.GetName()))
			utilruntime.HandleError(err)
			return nil
		}
	}

	c.recorder.Event(sla, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func podDiff(a []v1beta1.PodScale, b []corev1.Pod) (diff []corev1.Pod) {
	m := make(map[string]bool)

	for _, item := range a {
		m[item.Spec.Pod] = true
	}

	for _, item := range b {
		if _, ok := m[item.GetName()]; !ok {
			diff = append(diff, item)
		}
	}
	return diff
}

// enqueueService takes a Service resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Service.
func (c *Controller) enqueueService(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.servicesworkqueue.Add(key)
}

// enqueueSLA takes a ServiceLevelAgreement resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than ServiceLevelAgreement.
func (c *Controller) enqueueSLA(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.slasworkqueue.Add(key)
}
