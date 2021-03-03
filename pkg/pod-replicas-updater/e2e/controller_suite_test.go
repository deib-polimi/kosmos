package e2e_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	metricsgetter "github.com/lterrac/system-autoscaler/pkg/pod-autoscaler/pkg/metrics"
	replicaupdater "github.com/lterrac/system-autoscaler/pkg/pod-replicas-updater/pkg"

	"github.com/lterrac/system-autoscaler/pkg/informers"
	"k8s.io/apimachinery/pkg/labels"

	sainformers "github.com/lterrac/system-autoscaler/pkg/generated/informers/externalversions"
	appsv1 "k8s.io/api/apps/v1"
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
var replicaUpdater *replicaupdater.Controller

const namespace = "e2e"
const timeout = 180 * time.Second
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

	metricClient := &metricsgetter.FakeGetter{
		ResponseTime: 50,
	}

	By("instantiating recommender")
	replicaUpdater = replicaupdater.NewController(
		kubeClient,
		saClient,
		informers,
		metricClient,
	)

	By("starting informers")
	crdInformerFactory.Start(stopCh)
	coreInformerFactory.Start(stopCh)

	By("running recommender")
	err = replicaUpdater.Run(2, stopCh)
	Expect(err).NotTo(HaveOccurred())

	close(done)
}, 15)

var _ = AfterSuite(func() {
	replicaUpdater.Shutdown()
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

func usersMock(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte(`{"response_time":50.0}`))
}

func getPodsForSvc(svc *corev1.Service, namespace string, client kubernetes.Clientset) (*corev1.PodList, error) {
	set := labels.Set(svc.Spec.Selector)
	listOptions := metav1.ListOptions{LabelSelector: set.AsSelector().String()}
	pods, err := client.CoreV1().Pods(namespace).List(context.TODO(), listOptions)
	return pods, err
}

func newSLA(name string, container string, labels map[string]string, responseTime int64) *sa.ServiceLevelAgreement {
	return &sa.ServiceLevelAgreement{
		TypeMeta: metav1.TypeMeta{APIVersion: sa.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: sa.ServiceLevelAgreementSpec{
			Service: &sa.Service{
				Container: container,
				Selector: &metav1.LabelSelector{
					MatchLabels: labels,
				},
			},
			Metric: sa.MetricRequirement{
				ResponseTime: *resource.NewMilliQuantity(responseTime, resource.BinarySI),
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

func newPod(name string, container string, podLabels map[string]string) *corev1.Pod {
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
					Name:  container,
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

func newDeployment(name string, container string, labels map[string]string, selector metav1.LabelSelector, nReplicas int32) *appsv1.Deployment {
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "pods"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &nReplicas,
			Selector: &selector,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					Labels:    labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  container,
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
			},
		},
	}
}

func newPodScale(sla *sa.ServiceLevelAgreement, pod *corev1.Pod, selectorLabels map[string]string) *sa.PodScale {
	podLabels := make(labels.Set)
	for k, v := range selectorLabels {
		podLabels[k] = v
	}
	podLabels["system.autoscaler/node"] = pod.Spec.NodeName
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
			SLA:              sla.GetName(),
			Namespace:        sla.GetNamespace(),
			Pod:              pod.GetName(),
			Container:        pod.Spec.Containers[0].Name,
			DesiredResources: sla.Spec.DefaultResources,
		},
		Status: sa.PodScaleStatus{
			ActualResources: sla.Spec.DefaultResources,
			CappedResources: sla.Spec.DefaultResources,
		},
	}
}
