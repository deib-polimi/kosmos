package e2e_test

import (
	"context"
	sa "github.com/lterrac/system-autoscaler/pkg/apis/systemautoscaler/v1beta1"
	"github.com/lterrac/system-autoscaler/pkg/podscale-controller/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

var _ = Describe("PodScale controller", func() {
	Context("With an application deployed inside the cluster", func() {
		ctx := context.Background()

		BeforeEach(func() {

		})

		AfterEach(func() {

		})

		It("Update the pods as described in the pod scales contained in the channel", func() {

			slaName := "foo-sla-ru1"
			appName := "foo-app-ru1"

			labels := map[string]string{
				"app": "foo",
			}

			podLabels := map[string]string{
				"match": "podlabels",
			}

			sla := newSLA(slaName, labels)
			sla, err := saClient.SystemautoscalerV1beta1().ServiceLevelAgreements(namespace).Create(ctx, sla, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			svc := newService(appName, labels, podLabels)
			svc, err = kubeClient.CoreV1().Services(namespace).Create(ctx, svc, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			pod := newPod(appName, podLabels)
			pod, err = kubeClient.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			podScale := newPodScale(sla, pod, labels)
			podScale, err = saClient.SystemautoscalerV1beta1().PodScales(namespace).Create(ctx, podScale, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			updatedPodScale := podScale.DeepCopy()
			updatedPodScale.Status.ActualResources[corev1.ResourceCPU] = *resource.NewScaledQuantity(500, resource.Milli)
			updatedPodScale.Status.ActualResources[corev1.ResourceMemory] = *resource.NewScaledQuantity(500, resource.Mega)
			klog.Info(updatedPodScale)

			podScales := make([]*sa.PodScale, 0)
			podScales = append(podScales, updatedPodScale)

			nodeScale := types.NodeScales{
				Node:      "",
				PodScales: podScales,
			}

			contentionManagerOut <- nodeScale

			Eventually(func() bool {
				// Wait for pod to be scheduled
				pod, err = kubeClient.CoreV1().Pods(namespace).Get(ctx, pod.Name, metav1.GetOptions{})
				Expect(err).ShouldNot(HaveOccurred())
				// TODO: Check for the first container
				requestCpu := pod.Spec.Containers[0].Resources.Requests.Cpu().ScaledValue(resource.Milli)
				requestMem := pod.Spec.Containers[0].Resources.Requests.Memory().ScaledValue(resource.Mega)
				limitCpu := pod.Spec.Containers[0].Resources.Limits.Cpu().ScaledValue(resource.Milli)
				limitMem := pod.Spec.Containers[0].Resources.Limits.Memory().ScaledValue(resource.Mega)
				return requestCpu == limitCpu &&
					requestMem == limitMem &&
					requestCpu == updatedPodScale.Status.ActualResources.Cpu().ScaledValue(resource.Milli) &&
					requestMem == updatedPodScale.Status.ActualResources.Memory().ScaledValue(resource.Mega)
			}, timeout, interval).Should(BeTrue())

			err = saClient.SystemautoscalerV1beta1().PodScales(namespace).Delete(ctx, podScale.Name, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			err = saClient.SystemautoscalerV1beta1().ServiceLevelAgreements(namespace).Delete(ctx, sla.Name, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			err = kubeClient.CoreV1().Services(namespace).Delete(ctx, svc.Name, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			err = kubeClient.CoreV1().Pods(namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

		})
	})
})
