package e2e_test

import (
	"context"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"

	systemautoscaler "github.com/lterrac/system-autoscaler/pkg/apis/systemautoscaler/v1beta1"
	. "github.com/lterrac/system-autoscaler/pkg/podscale-controller/pkg/controller"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const namespace = "e2e"
const timeout = 60 * time.Second
const interval = 1 * time.Second

var _ = Describe("PodScale controller", func() {
	Context("With an application deployed inside the cluster", func() {
		ctx := context.Background()

		BeforeEach(func() {

		})

		AfterEach(func() {

		})

		It("Creates the podscale if it matches the SLA service selector", func() {
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

			// resource cleanup
			err = kubeClient.CoreV1().Services(namespace).Delete(ctx, svc.GetName(), metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			err = systemAutoscalerClient.SystemautoscalerV1beta1().ServiceLevelAgreements(namespace).Delete(ctx, slaName, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			err = kubeClient.CoreV1().Pods(namespace).Delete(ctx, pod.GetName(), metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("Handles the service selector changes", func() {
			oldServiceSelector := map[string]string{
				"app": "foo",
			}

			sla := newSLA("bar-sla", oldServiceSelector)
			matchedSvc, matchedPod := newApplication("bar-app", oldServiceSelector)

			newServiceSelector := map[string]string{
				"app": "bar",
			}

			matchedSvc.Labels[SubjectToLabel] = sla.Name

			_, err := kubeClient.CoreV1().Services(namespace).Create(ctx, matchedSvc, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			_, err = kubeClient.CoreV1().Pods(namespace).Create(ctx, matchedPod, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			sla, err = systemAutoscalerClient.SystemautoscalerV1beta1().ServiceLevelAgreements(namespace).Create(ctx, sla, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() bool {
				actual, err := systemAutoscalerClient.SystemautoscalerV1beta1().PodScales(namespace).Get(ctx, "pod-"+matchedPod.GetName(), metav1.GetOptions{})
				return err == nil &&
					actual.Spec.Pod == matchedPod.GetName() &&
					actual.Spec.SLA == sla.GetName()
			}, timeout, interval).Should(BeTrue())

			sla.Spec.ServiceSelector = &metav1.LabelSelector{
				MatchLabels: newServiceSelector,
			}

			_, err = systemAutoscalerClient.SystemautoscalerV1beta1().ServiceLevelAgreements(namespace).Update(ctx, sla, metav1.UpdateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() bool {
				_, err := systemAutoscalerClient.SystemautoscalerV1beta1().PodScales(namespace).Get(ctx, "pod-"+matchedPod.GetName(), metav1.GetOptions{})
				return apierrors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())

			// resource cleanup
			err = kubeClient.CoreV1().Services(namespace).Delete(ctx, matchedSvc.GetName(), metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			err = systemAutoscalerClient.SystemautoscalerV1beta1().ServiceLevelAgreements(namespace).Delete(ctx, sla.GetName(), metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			err = kubeClient.CoreV1().Pods(namespace).Delete(ctx, matchedPod.GetName(), metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())
		})
	})
	Context("With a Service Level Agreement matching and application", func() {
		ctx := context.Background()

		It("Changes the podscales based on existing pods increasing them when a pod is added", func() {
			oldServiceSelector := map[string]string{
				"app": "foo",
			}

			sla := newSLA("foobar-sla", oldServiceSelector)
			matchedSvc, matchedPod := newApplication("foobar-app", oldServiceSelector)
			matchedSvc.Labels[SubjectToLabel] = sla.Name

			_, err := kubeClient.CoreV1().Services(namespace).Create(ctx, matchedSvc, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			_, err = kubeClient.CoreV1().Pods(namespace).Create(ctx, matchedPod, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			sla, err = systemAutoscalerClient.SystemautoscalerV1beta1().ServiceLevelAgreements(namespace).Create(ctx, sla, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() bool {
				actual, err := systemAutoscalerClient.SystemautoscalerV1beta1().PodScales(namespace).Get(ctx, "pod-"+matchedPod.GetName(), metav1.GetOptions{})
				return err == nil &&
					actual.Spec.Pod == matchedPod.GetName() &&
					actual.Spec.SLA == sla.GetName()
			}, timeout, interval).Should(BeTrue())

			newPod := &corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					APIVersion: corev1.SchemeGroupVersion.String(),
					Kind:       "pods",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foobarfoo",
					Namespace: namespace,
					Labels: map[string]string{
						"match": "bar",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "foobarfoo",
							Image: "gcr.io/distroless/static:nonroot",
						},
					},
				},
			}

			_, err = kubeClient.CoreV1().Pods(namespace).Create(ctx, newPod, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() bool {
				actual, err := systemAutoscalerClient.SystemautoscalerV1beta1().PodScales(namespace).Get(ctx, "pod-"+newPod.GetName(), metav1.GetOptions{})
				return err == nil &&
					actual.Spec.Pod == newPod.GetName() &&
					actual.Spec.SLA == sla.GetName()
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				podscales, podscaleErr := systemAutoscalerClient.SystemautoscalerV1beta1().PodScales(namespace).List(ctx, metav1.ListOptions{
					LabelSelector: labels.Set(matchedSvc.Spec.Selector).AsSelector().String(),
				})
				pods, podErr := kubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
				return podscaleErr == nil &&
					podErr == nil &&
					len(podscales.Items) == len(pods.Items)
			}, timeout, interval).Should(BeTrue())

			err = kubeClient.CoreV1().Pods(namespace).Delete(ctx, newPod.GetName(), metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() bool {
				_, err := systemAutoscalerClient.SystemautoscalerV1beta1().PodScales(namespace).Get(ctx, "pod-"+newPod.GetName(), metav1.GetOptions{})
				return apierrors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				podscales, podscaleErr := systemAutoscalerClient.SystemautoscalerV1beta1().PodScales(namespace).List(ctx, metav1.ListOptions{
					LabelSelector: labels.Set(matchedSvc.Spec.Selector).AsSelector().String(),
				})
				pods, podErr := kubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
				return podscaleErr == nil &&
					podErr == nil &&
					len(podscales.Items) == len(pods.Items)
			}, timeout, interval).Should(BeTrue())

			// resource cleanup
			err = kubeClient.CoreV1().Services(namespace).Delete(ctx, matchedSvc.GetName(), metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			err = systemAutoscalerClient.SystemautoscalerV1beta1().ServiceLevelAgreements(namespace).Delete(ctx, sla.GetName(), metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			err = kubeClient.CoreV1().Pods(namespace).Delete(ctx, matchedPod.GetName(), metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())
		})
	})
})

func newSLA(name string, labels map[string]string) *systemautoscaler.ServiceLevelAgreement {
	return &systemautoscaler.ServiceLevelAgreement{
		TypeMeta: metav1.TypeMeta{APIVersion: systemautoscaler.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
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
		}, &corev1.Pod{
			TypeMeta: metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "pods"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels:    podLabels,
			},
			Spec: corev1.PodSpec{Containers: []corev1.Container{
				{
					Name:  name,
					Image: "gcr.io/distroless/static:nonroot",
				},
			},
			},
		}
}
