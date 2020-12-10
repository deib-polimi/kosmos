package pod_resource_updater

import (
	"context"
	podscalesclientset "github.com/lterrac/system-autoscaler/pkg/generated/clientset/versioned"
	samplescheme "github.com/lterrac/system-autoscaler/pkg/generated/clientset/versioned/scheme"
	"github.com/lterrac/system-autoscaler/pkg/podscale-controller/pkg/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"time"
)

const controllerAgentName = "pod-resource-updater"

// Controller is the controller that recommends resources to the pods.
// For each Pod Scale assigned to the recommender, it will have a pod saved in a list.
// Every x seconds, the recommender polls the metrics from the pod by using an http request.
// For pod metrics it retrieves, it computes the new resources to assign to the pod.
type Controller struct {

	// podScalesClientset is a clientset for our own API group
	podScalesClientset podscalesclientset.Interface

	// kubernetesCLientset is the client-go of kubernetes
	kubernetesClientset kubernetes.Clientset

	// TODO: we don't need the queue
	// podScalesQueue in a work queue that contains the pod that needs to be updated

	// recorder is an event recorder for recording Event resources to the

	// Kubernetes API.
	recorder record.EventRecorder

	// in is the input channel.
	in chan types.NodeScales
}

// NewController returns a new sample controller
func NewController(
	kubernetesClientset *kubernetes.Clientset,
	podScalesClientset podscalesclientset.Interface,
	in chan types.NodeScales,
) *Controller {

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
		podScalesClientset:  podScalesClientset,
		kubernetesClientset: *kubernetesClientset,
		recorder:            recorder,
		in:                  in,
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
	klog.Info("Starting pod resource updater controller")

	klog.Info("Starting pod resource updater workers")
	// Launch the workers to process podScale resources and recommend new pod scales
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runNodeScaleWorker, time.Second, stopCh)
	}

	klog.Info("Started pod resource updater workers")

	return nil
}

func (c *Controller) Shutdown() {
	utilruntime.HandleCrash()
}

func (c *Controller) runNodeScaleWorker() {
	for nodeScale := range c.in {
		klog.Info("Processing ", nodeScale)
		for _, podScale := range nodeScale.PodScales {

			// TODO: use lister if possible
			pod, err := c.kubernetesClientset.CoreV1().Pods(podScale.Spec.PodRef.Namespace).Get(context.TODO(), podScale.Spec.PodRef.Name, metav1.GetOptions{})
			if err != nil {
				klog.Error("Error retrieving the pod: ", err)
				return
			}

			newPod, err := syncPod(*pod, *podScale)
			if err != nil {
				klog.Error("Error syncing the pod: ", err)
				return
			}

			// TODO: we should use something like a transaction in order to make the two updates consistent

			// TODO: use lister if possible
			updatedPod, err := c.kubernetesClientset.CoreV1().Pods(podScale.Spec.PodRef.Namespace).Update(context.TODO(), newPod, metav1.UpdateOptions{})
			if err != nil {
				klog.Error("Error updating the pod: ", err)
				return
			}

			// TODO: use lister if possible
			_, err = c.podScalesClientset.SystemautoscalerV1beta1().PodScales(podScale.Namespace).Update(context.TODO(), podScale, metav1.UpdateOptions{})
			if err != nil {
				klog.Error("Error updating the pod scale: ", err)
				return
			}

			klog.Info("Desired resources:", podScale.Spec.DesiredResources)
			klog.Info("Actual resources:", podScale.Status.ActualResources)
			klog.Info("Pod resources:", updatedPod.Spec.Containers[0].Resources)
		}
	}
}
