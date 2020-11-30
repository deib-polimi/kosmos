package e2e_test

import (
	"context"
	"time"

	systemautoscaler "github.com/lterrac/system-autoscaler/pkg/apis/systemautoscaler/v1beta1"
	. "github.com/lterrac/system-autoscaler/pkg/podscale-controller/pkg/controller"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const namespace = "default"

var _ = Describe("PodScale controller", func() {
	Context("With an application deployed inside the cluster", func() {

		const timeout = 60 * time.Second
		const interval = 1 * time.Second

		BeforeEach(func() {

		})

		AfterEach(func() {

		})

		It("Creates the podscale if it matches the SLA service selector", func() {
			ctx := context.Background()
			slaName := "foo-sla"
			appName := "foo-app"

			labels := map[string]string{
				"app": "foo",
			}

			sla := newSLA(slaName, labels)
			svc, pod := newApplication(appName, labels)

			_, err := kubeClient.CoreV1().Services(namespace).Create(ctx, svc, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			_, err = kubeClient.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			_, err = systemAutoscalerClient.SystemautoscalerV1beta1().ServiceLevelAgreements(namespace).Create(ctx, sla, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() bool {
				actual, err := kubeClient.CoreV1().Services(namespace).Get(ctx, svc.GetName(), metav1.GetOptions{})
				return err == nil && actual.GetLabels()[SubjectToLabel] == sla.GetName()
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				actual, err := systemAutoscalerClient.SystemautoscalerV1beta1().PodScales(namespace).Get(ctx, "pod-"+pod.GetName(), metav1.GetOptions{})
				return err == nil &&
					actual.Spec.Pod == pod.GetName() &&
					actual.Spec.SLA == sla.GetName()
			}, timeout, interval).Should(BeTrue())
		})
	})
})

func newSLA(name string, labels map[string]string) *systemautoscaler.ServiceLevelAgreement {
	return &systemautoscaler.ServiceLevelAgreement{
		TypeMeta: metav1.TypeMeta{APIVersion: systemautoscaler.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: systemautoscaler.ServiceLevelAgreementSpec{
			ServiceSelector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
		},
	}
}

func newApplication(name string, labels map[string]string) (*corev1.Service, *corev1.Pod) {
	podLabels := map[string]string{
		"match": "bar",
	}
	return &corev1.Service{
			TypeMeta: metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "services"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: metav1.NamespaceDefault,
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
		}, &corev1.Pod{
			TypeMeta: metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "pods"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foobar",
				Namespace: namespace,
				Labels:    podLabels,
			},
			Spec: corev1.PodSpec{Containers: []corev1.Container{
				{
					Name:  "foobar",
					Image: "gcr.io/distroless/static:nonroot",
				},
			},
			},
		}
}
