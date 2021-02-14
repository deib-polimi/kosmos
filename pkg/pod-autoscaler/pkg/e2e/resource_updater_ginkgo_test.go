package e2e_test

import (
	"context"
	sa "github.com/lterrac/system-autoscaler/pkg/apis/systemautoscaler/v1beta1"
	"github.com/lterrac/system-autoscaler/pkg/containerscale-controller/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

var _ = Describe("Resource Updater controller", func() {
	Context("With an application deployed inside the cluster", func() {
		ctx := context.Background()

		It("Update the pods as described in the pod scales contained in the channel", func() {

			slaName := "foo-sla-ru1"
			appName := "foo-app-ru1"
			containerName := "container"

			labels := map[string]string{
				"app": "foo",
			}

			podLabels := map[string]string{
				"match": "podlabels",
			}

			sla := newSLA(slaName, containerName, labels)
			sla, err := saClient.SystemautoscalerV1beta1().ServiceLevelAgreements(namespace).Create(ctx, sla, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			svc := newService(appName, labels, podLabels)
			svc, err = kubeClient.CoreV1().Services(namespace).Create(ctx, svc, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			pod := newPod(appName, containerName, podLabels)
			pod, err = kubeClient.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			// wait for pod1 to be assigned to a node
			Eventually(func() bool {
				pod, err = kubeClient.CoreV1().Pods(namespace).Get(ctx, pod.Name, metav1.GetOptions{})
				return pod.Spec.NodeName != ""
			}, timeout, interval).Should(BeTrue())

			containerScale := newContainerScale(sla, pod, labels)
			containerScale, err = saClient.SystemautoscalerV1beta1().ContainerScales(namespace).Create(ctx, containerScale, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			updatedContainerScale := containerScale.DeepCopy()
			updatedContainerScale.Status.ActualResources[corev1.ResourceCPU] = *resource.NewScaledQuantity(500, resource.Milli)
			updatedContainerScale.Status.ActualResources[corev1.ResourceMemory] = *resource.NewScaledQuantity(500, resource.Mega)
			klog.Info(updatedContainerScale)

			containerScales := make([]*sa.ContainerScale, 0)
			containerScales = append(containerScales, updatedContainerScale)

			nodeScale := types.NodeScales{
				Node:            pod.Spec.NodeName,
				ContainerScales: containerScales,
			}

			contentionManagerOut <- nodeScale

			Eventually(func() bool {
				// Wait for pod to be scheduled
				pod, err = kubeClient.CoreV1().Pods(namespace).Get(ctx, pod.Name, metav1.GetOptions{})
				Expect(err).ShouldNot(HaveOccurred())

				requestCpu := pod.Spec.Containers[0].Resources.Requests.Cpu().ScaledValue(resource.Milli)
				requestMem := pod.Spec.Containers[0].Resources.Requests.Memory().ScaledValue(resource.Mega)
				limitCpu := pod.Spec.Containers[0].Resources.Limits.Cpu().ScaledValue(resource.Milli)
				limitMem := pod.Spec.Containers[0].Resources.Limits.Memory().ScaledValue(resource.Mega)

				return requestCpu == limitCpu &&
					requestMem == limitMem &&
					requestCpu == updatedContainerScale.Status.ActualResources.Cpu().ScaledValue(resource.Milli) &&
					requestMem == updatedContainerScale.Status.ActualResources.Memory().ScaledValue(resource.Mega)
			}, timeout, interval).Should(BeTrue())

			err = saClient.SystemautoscalerV1beta1().ContainerScales(namespace).Delete(ctx, containerScale.Name, metav1.DeleteOptions{})
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
