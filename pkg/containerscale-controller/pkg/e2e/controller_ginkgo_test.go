package e2e_test

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/record"

	systemautoscaler "github.com/lterrac/system-autoscaler/pkg/apis/systemautoscaler/v1beta1"
	"github.com/lterrac/system-autoscaler/pkg/containerscale-controller/pkg/controller"
	. "github.com/lterrac/system-autoscaler/pkg/containerscale-controller/pkg/controller"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const namespace = "e2e"
const timeout = 40 * time.Second
const interval = 1 * time.Second

var _ = Describe("ContainerScale controller", func() {
	Context("With an application deployed inside the cluster", func() {
		ctx := context.Background()

		It("Creates the containerscale if it matches the SLA service selector", func() {
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

			_, err = saClient.SystemautoscalerV1beta1().ServiceLevelAgreements(namespace).Create(ctx, sla, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() bool {
				actual, err := kubeClient.CoreV1().Services(namespace).Get(ctx, svc.GetName(), metav1.GetOptions{})
				return err == nil && actual.GetLabels()[SubjectToLabel] == sla.GetName()
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				actual, err := saClient.SystemautoscalerV1beta1().ContainerScales(namespace).Get(ctx, "pod-"+pod.GetName(), metav1.GetOptions{})
				return err == nil &&
					actual.Spec.PodRef.Name == pod.GetName() &&
					actual.Spec.PodRef.Namespace == namespace &&
					actual.Spec.SLARef.Name == sla.GetName() &&
					actual.Spec.SLARef.Namespace == namespace
			}, timeout, interval).Should(BeTrue())

			// resource cleanup
			err = kubeClient.CoreV1().Services(namespace).Delete(ctx, svc.GetName(), metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			err = saClient.SystemautoscalerV1beta1().ServiceLevelAgreements(namespace).Delete(ctx, slaName, metav1.DeleteOptions{})
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

			sla, err = saClient.SystemautoscalerV1beta1().ServiceLevelAgreements(namespace).Create(ctx, sla, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() bool {
				actual, err := saClient.SystemautoscalerV1beta1().ContainerScales(namespace).Get(ctx, "pod-"+matchedPod.GetName(), metav1.GetOptions{})
				return err == nil &&
					actual.Spec.PodRef.Name == matchedPod.GetName() &&
					actual.Spec.PodRef.Namespace == namespace &&
					actual.Spec.SLARef.Name == sla.GetName() &&
					actual.Spec.SLARef.Namespace == namespace
			}, timeout, interval).Should(BeTrue())

			sla.Spec.Service = &systemautoscaler.Service{
				Container: "",
				Selector: &metav1.LabelSelector{
					MatchLabels: newServiceSelector,
				},
			}

			_, err = saClient.SystemautoscalerV1beta1().ServiceLevelAgreements(namespace).Update(ctx, sla, metav1.UpdateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() bool {
				_, err := saClient.SystemautoscalerV1beta1().ContainerScales(namespace).Get(ctx, "pod-"+matchedPod.GetName(), metav1.GetOptions{})
				return apierrors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())

			// resource cleanup
			err = kubeClient.CoreV1().Services(namespace).Delete(ctx, matchedSvc.GetName(), metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			err = saClient.SystemautoscalerV1beta1().ServiceLevelAgreements(namespace).Delete(ctx, sla.GetName(), metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			err = kubeClient.CoreV1().Pods(namespace).Delete(ctx, matchedPod.GetName(), metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())
		})
	})

	Context("With a Service Level Agreement matching and application", func() {
		ctx := context.Background()

		It("Changes the containerscales based on existing pods increasing them when a pod is added", func() {
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

			sla, err = saClient.SystemautoscalerV1beta1().ServiceLevelAgreements(namespace).Create(ctx, sla, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() bool {
				actual, err := saClient.SystemautoscalerV1beta1().ContainerScales(namespace).Get(ctx, "pod-"+matchedPod.GetName(), metav1.GetOptions{})
				return err == nil &&
					actual.Spec.PodRef.Name == matchedPod.GetName() &&
					actual.Spec.PodRef.Namespace == namespace &&
					actual.Spec.SLARef.Name == sla.GetName() &&
					actual.Spec.SLARef.Namespace == namespace
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
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    *resource.NewScaledQuantity(50, resource.Milli),
									corev1.ResourceMemory: *resource.NewScaledQuantity(50, resource.Mega),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    *resource.NewScaledQuantity(50, resource.Milli),
									corev1.ResourceMemory: *resource.NewScaledQuantity(50, resource.Mega),
								},
							},
						},
					},
				},
			}

			_, err = kubeClient.CoreV1().Pods(namespace).Create(ctx, newPod, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() bool {
				actual, err := saClient.SystemautoscalerV1beta1().ContainerScales(namespace).Get(ctx, "pod-"+newPod.GetName(), metav1.GetOptions{})
				return err == nil &&
					actual.Spec.PodRef.Name == newPod.GetName() &&
					actual.Spec.PodRef.Namespace == namespace &&
					actual.Spec.SLARef.Name == sla.GetName() &&
					actual.Spec.SLARef.Namespace == namespace
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				containerscales, containerscaleErr := saClient.SystemautoscalerV1beta1().ContainerScales(namespace).List(ctx, metav1.ListOptions{
					LabelSelector: labels.Set(matchedSvc.Spec.Selector).AsSelector().String(),
				})
				pods, podErr := kubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
				return containerscaleErr == nil &&
					podErr == nil &&
					len(containerscales.Items) == len(pods.Items)
			}, timeout, interval).Should(BeTrue())

			err = kubeClient.CoreV1().Pods(namespace).Delete(ctx, newPod.GetName(), metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() bool {
				_, err := saClient.SystemautoscalerV1beta1().ContainerScales(namespace).Get(ctx, "pod-"+newPod.GetName(), metav1.GetOptions{})
				return apierrors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				containerscales, containerscaleErr := saClient.SystemautoscalerV1beta1().ContainerScales(namespace).List(ctx, metav1.ListOptions{
					LabelSelector: labels.Set(matchedSvc.Spec.Selector).AsSelector().String(),
				})
				pods, podErr := kubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
				return containerscaleErr == nil &&
					podErr == nil &&
					len(containerscales.Items) == len(pods.Items)
			}, timeout, interval).Should(BeTrue())

			// resource cleanup
			err = kubeClient.CoreV1().Services(namespace).Delete(ctx, matchedSvc.GetName(), metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			err = saClient.SystemautoscalerV1beta1().ServiceLevelAgreements(namespace).Delete(ctx, sla.GetName(), metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			err = kubeClient.CoreV1().Pods(namespace).Delete(ctx, matchedPod.GetName(), metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())
		})
	})

	Context("With a Pod with a QOS class other than Guaranteed", func() {
		ctx := context.Background()

		It("Fires an event telling that is not able to process the Pod", func() {
			oldServiceSelector := map[string]string{
				"app": "foo",
			}

			sla := newSLA("foobarfooz-sla", oldServiceSelector)
			matchedSvc, matchedPod := newApplication("foobarfooz-app", oldServiceSelector)
			matchedSvc.Labels[SubjectToLabel] = sla.Name

			// Having different request and limit will automatically foreclose Guaranteed QOS class
			matchedPod.Spec.Containers[0].Resources = corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    *resource.NewScaledQuantity(100, resource.Milli),
					corev1.ResourceMemory: *resource.NewScaledQuantity(50, resource.Mega),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    *resource.NewScaledQuantity(50, resource.Milli),
					corev1.ResourceMemory: *resource.NewScaledQuantity(50, resource.Mega),
				},
			}

			_, err := kubeClient.CoreV1().Services(namespace).Create(ctx, matchedSvc, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			_, err = kubeClient.CoreV1().Pods(namespace).Create(ctx, matchedPod, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			sla, err = saClient.SystemautoscalerV1beta1().ServiceLevelAgreements(namespace).Create(ctx, sla, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			eventBroadcaster := record.NewBroadcaster()
			eventBroadcaster.StartStructuredLogging(0)

			Eventually(func() bool {
				events, err := kubeClient.EventsV1beta1().Events(matchedPod.Namespace).List(ctx, metav1.ListOptions{})

				if err != nil {
					return false
				}

				for _, event := range events.Items {
					if event.Reason == controller.QOSNotSupported &&
						event.Regarding.Kind == matchedPod.Kind &&
						event.Regarding.Name == matchedPod.Name &&
						event.Regarding.Namespace == matchedPod.Namespace {
						return true
					}
				}

				return false
			}, timeout, interval).Should(BeTrue())

			// resource cleanup
			err = kubeClient.CoreV1().Services(namespace).Delete(ctx, matchedSvc.GetName(), metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())
			err = saClient.SystemautoscalerV1beta1().ServiceLevelAgreements(namespace).Delete(ctx, sla.GetName(), metav1.DeleteOptions{})
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
			Service: &systemautoscaler.Service{
				Selector:  &metav1.LabelSelector{
					MatchLabels: labels,
				},
				Container: "",
			},
			Metric: systemautoscaler.MetricRequirement{
				ResponseTime: *resource.NewQuantity(3, resource.BinarySI),
			},
			DefaultResources: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:    *resource.NewScaledQuantity(50, resource.Milli),
				corev1.ResourceMemory: *resource.NewScaledQuantity(50, resource.Mega),
			},
		},
	}
}

func newApplication(name string, labels map[string]string) (*corev1.Service, *corev1.Pod) {
	podLabels := map[string]string{
		"match": "bar",
	}
	return &corev1.Service{
			TypeMeta: metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "Service"},
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
			TypeMeta: metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "Pod"},
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
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    *resource.NewScaledQuantity(50, resource.Milli),
								corev1.ResourceMemory: *resource.NewScaledQuantity(50, resource.Mega),
							},
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    *resource.NewScaledQuantity(50, resource.Milli),
								corev1.ResourceMemory: *resource.NewScaledQuantity(50, resource.Mega),
							},
						},
					},
				},
			},
		}
}
