package e2e_test

import (
	"github.com/lterrac/system-autoscaler/pkg/informers"
	"k8s.io/apimachinery/pkg/labels"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	sainformers "github.com/lterrac/system-autoscaler/pkg/generated/informers/externalversions"
	resupd "github.com/lterrac/system-autoscaler/pkg/pod-autoscaler/pkg/pod-resource-updater"
	"github.com/lterrac/system-autoscaler/pkg/pod-autoscaler/pkg/recommender"
	"github.com/lterrac/system-autoscaler/pkg/podscale-controller/pkg/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	coreinformers "k8s.io/client-go/informers"

	sa "github.com/lterrac/system-autoscaler/pkg/apis/systemautoscaler/v1beta1"
	"github.com/lterrac/system-autoscaler/pkg/signals"
	"k8s.io/client-go/kubernetes"

	clientset "github.com/lterrac/system-autoscaler/pkg/generated/clientset/versioned"
	. "github.com/onsi/gomega"

	systemautoscaler "github.com/lterrac/system-autoscaler/pkg/generated/clientset/versioned"
	. "github.com/onsi/ginkgo"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var cfg *rest.Config
var testEnv *envtest.Environment
var kubeClient *kubernetes.Clientset
var saClient *systemautoscaler.Clientset
var recommenderOut chan types.NodeScales
var contentionManagerOut chan types.NodeScales
var recommenderController *recommender.Controller
var updaterController *resupd.Controller

const namespace = "e2e"
const timeout = 60 * time.Second
const interval = 1 * time.Second

var _ = BeforeSuite(func(done Done) {
	useCluster := true

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		UseExistingCluster:       &useCluster,
		AttachControlPlaneOutput: true,
	}

	var err error

	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	By("bootstrapping clients")
	kubeClient = kubernetes.NewForConfigOrDie(cfg)
	saClient = clientset.NewForConfigOrDie(cfg)

	By("bootstrapping informers")
	crdInformerFactory := sainformers.NewSharedInformerFactory(saClient, time.Second*30)
	coreInformerFactory := coreinformers.NewSharedInformerFactory(kubeClient, time.Second*30)

	By("creating informers")
	informers := informers.Informers{
		Pod:                   coreInformerFactory.Core().V1().Pods(),
		Node:                  coreInformerFactory.Core().V1().Nodes(),
		Service:               coreInformerFactory.Core().V1().Services(),
		PodScale:              crdInformerFactory.Systemautoscaler().V1beta1().PodScales(),
		ServiceLevelAgreement: crdInformerFactory.Systemautoscaler().V1beta1().ServiceLevelAgreements(),
	}

	By("bootstrapping controller")
	stopCh := signals.SetupSignalHandler()

	By("starting channels")
	recommenderOut = make(chan types.NodeScales, 100)
	contentionManagerOut = make(chan types.NodeScales, 100)

	By("instantiating recommender")
	recommenderController = recommender.NewController(
		kubeClient,
		saClient,
		informers,
		recommenderOut,
	)
	client := recommender.NewMetricClient()
	server := serverMock()
	client.Host = server.URL[7:]
	recommenderController.MetricClient = client

	By("instantiating pod resource updater")
	updaterController = resupd.NewController(
		kubeClient,
		saClient,
		informers,
		contentionManagerOut,
	)

	By("starting informers")
	crdInformerFactory.Start(stopCh)
	coreInformerFactory.Start(stopCh)

	By("running recommender")
	err = recommenderController.Run(2, stopCh)
	Expect(err).NotTo(HaveOccurred())

	By("running pod resource updater")
	err = updaterController.Run(1, stopCh)
	Expect(err).NotTo(HaveOccurred())

	close(done)
}, 15)

var _ = AfterSuite(func() {
	recommenderController.Shutdown()
	updaterController.Shutdown()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})

func serverMock() *httptest.Server {
	handler := http.NewServeMux()
	handler.HandleFunc("/", usersMock)

	srv := httptest.NewServer(handler)

	return srv
}

func usersMock(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte(`{"response_time":5.0}`))
}

func newSLA(name string, labels map[string]string) *sa.ServiceLevelAgreement {
	return &sa.ServiceLevelAgreement{
		TypeMeta: metav1.TypeMeta{APIVersion: sa.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: sa.ServiceLevelAgreementSpec{
			ServiceSelector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Metric: sa.MetricRequirement{
				ResponseTime: *resource.NewQuantity(3, resource.BinarySI),
			},
			DefaultResources: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:    *resource.NewScaledQuantity(50, resource.Milli),
				corev1.ResourceMemory: *resource.NewScaledQuantity(50, resource.Mega),
			},
		},
	}
}

func newService(name string, labels map[string]string, podLabels map[string]string) *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "services"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: podLabels,
			Ports: []corev1.ServicePort{
				{
					Name: "http",
					Port: 8000,
					TargetPort: intstr.IntOrString{
						Type:   0,
						IntVal: 8000,
					},
				},
			},
		},
	}
}

func newPod(name string, podLabels map[string]string) *corev1.Pod {
	return &corev1.Pod{
		TypeMeta: metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "pods"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    podLabels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  name,
					Image: "gcr.io/distroless/static:nonroot",
					Resources: corev1.ResourceRequirements{
						Limits: map[corev1.ResourceName]resource.Quantity{
							corev1.ResourceCPU:    *resource.NewScaledQuantity(50, resource.Milli),
							corev1.ResourceMemory: *resource.NewScaledQuantity(50, resource.Mega),
						},
						Requests: map[corev1.ResourceName]resource.Quantity{
							corev1.ResourceCPU:    *resource.NewScaledQuantity(50, resource.Milli),
							corev1.ResourceMemory: *resource.NewScaledQuantity(50, resource.Mega),
						},
					},
				},
			},
		},
	}
}

func newPodScale(sla *sa.ServiceLevelAgreement, pod *corev1.Pod, selectorLabels map[string]string) *sa.PodScale {
	podLabels := make(labels.Set)
	for k, v := range selectorLabels {
		podLabels[k] = v
	}
	podLabels["node"] = pod.Spec.NodeName
	return &sa.PodScale{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "sa.polimi.it/v1beta1",
			Kind:       "PodScale",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-" + pod.GetName(),
			Namespace: sla.Namespace,
			Labels:    podLabels,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "sa.polimi.it/v1beta1",
					Kind:       "ServiceLevelAgreement",
					Name:       sla.GetName(),
					UID:        sla.GetUID(),
				},
			},
		},
		Spec: sa.PodScaleSpec{
			SLARef: sa.SLARef{
				Name:      sla.GetName(),
				Namespace: sla.GetNamespace(),
			},
			PodRef: sa.PodRef{
				Name:      pod.GetName(),
				Namespace: pod.GetNamespace(),
			},
			DesiredResources: sla.Spec.DefaultResources,
		},
		Status: sa.PodScaleStatus{
			ActualResources: sla.Spec.DefaultResources,
		},
	}
}
