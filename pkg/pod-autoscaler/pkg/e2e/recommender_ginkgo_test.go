package e2e_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Recommender controller", func() {
	Context("With an application deployed inside the cluster", func() {
		ctx := context.Background()

		It("Monitor pods whenever a new pod scale is created", func() {

			slaName := "foo-sla-rec1"
			appName := "foo-app-rec1"
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

			// TODO: wait for pod to be assigned in a better way
			// wait for pod to be assigned to a node
			Eventually(func() bool {
				pod, err = kubeClient.CoreV1().Pods(namespace).Get(ctx, pod.Name, metav1.GetOptions{})
				return pod.Spec.NodeName != ""
			}, timeout, interval).Should(BeTrue())

			podScale := newPodScale(sla, pod, labels)
			podScale, err = saClient.SystemautoscalerV1beta1().PodScales(namespace).Create(ctx, podScale, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() bool {
				// Wait for pod to be scheduled
				pod, err = kubeClient.CoreV1().Pods(namespace).Get(ctx, pod.Name, metav1.GetOptions{})
				Expect(err).ShouldNot(HaveOccurred())
				nodeScale := <-recommenderOut
				return nodeScale.Node == pod.Spec.NodeName &&
					len(nodeScale.PodScales) == 1 &&
					nodeScale.PodScales[0].Namespace == podScale.Namespace &&
					nodeScale.PodScales[0].Name == podScale.Name
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

		It("Stop to monitor pods whenever a pod scale is deleted", func() {

			slaName := "foo-sla-rec2"
			appName := "foo-app-rec2"
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

			pod1 := newPod("replica1", containerName, podLabels)
			pod1, err = kubeClient.CoreV1().Pods(namespace).Create(ctx, pod1, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			// wait for pod1 to be assigned to a node
			Eventually(func() bool {
				pod1, err = kubeClient.CoreV1().Pods(namespace).Get(ctx, pod1.Name, metav1.GetOptions{})
				return pod1.Spec.NodeName != ""
			}, timeout, interval).Should(BeTrue())

			podScale1 := newPodScale(sla, pod1, labels)
			podScale1, err = saClient.SystemautoscalerV1beta1().PodScales(namespace).Create(ctx, podScale1, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() bool {
				// Wait for pod to be scheduled
				pod1, err = kubeClient.CoreV1().Pods(namespace).Get(ctx, pod1.Name, metav1.GetOptions{})
				Expect(err).ShouldNot(HaveOccurred())
				nodeScale := <-recommenderOut
				return nodeScale.Node == pod1.Spec.NodeName &&
					len(nodeScale.PodScales) == 1 &&
					nodeScale.PodScales[0].Namespace == podScale1.Namespace &&
					nodeScale.PodScales[0].Name == podScale1.Name
			}, timeout, interval).Should(BeTrue())

			pod2 := newPod("replica2", containerName, podLabels)
			pod2, err = kubeClient.CoreV1().Pods(namespace).Create(ctx, pod2, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			// wait for pod2 to be assigned to a node
			Eventually(func() bool {
				pod2, err = kubeClient.CoreV1().Pods(namespace).Get(ctx, pod2.Name, metav1.GetOptions{})
				return pod2.Spec.NodeName != ""
			}, timeout, interval).Should(BeTrue())

			podScale2 := newPodScale(sla, pod2, labels)
			podScale2, err = saClient.SystemautoscalerV1beta1().PodScales(namespace).Create(ctx, podScale2, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			err = saClient.SystemautoscalerV1beta1().PodScales(namespace).Delete(ctx, podScale1.Name, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			err = kubeClient.CoreV1().Pods(namespace).Delete(ctx, pod1.Name, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(func() bool {
				// Wait for pod to be scheduled
				pod2, err = kubeClient.CoreV1().Pods(namespace).Get(ctx, pod2.Name, metav1.GetOptions{})
				Expect(err).ShouldNot(HaveOccurred())
				nodeScale := <-recommenderOut
				return nodeScale.Node == pod2.Spec.NodeName &&
					len(nodeScale.PodScales) == 1 &&
					nodeScale.PodScales[0].Namespace == podScale2.Namespace &&
					nodeScale.PodScales[0].Name == podScale2.Name
			}, timeout, interval).Should(BeTrue())

			// For x control periods, pod 1 does not appear in the channel
			x := 5
			Eventually(func() bool {
				// Wait for pod to be scheduled
				pod2, err = kubeClient.CoreV1().Pods(namespace).Get(ctx, pod2.Name, metav1.GetOptions{})
				Expect(err).ShouldNot(HaveOccurred())
				for i := 0; i < x; i++ {
					nodeScale := <-recommenderOut
					if nodeScale.Node == pod1.Spec.NodeName &&
						len(nodeScale.PodScales) == 1 &&
						nodeScale.PodScales[0].Namespace == podScale1.Namespace &&
						nodeScale.PodScales[0].Name == podScale1.Name {
						return false
					}
				}
				return true
			}, timeout, interval).Should(BeTrue())

			err = saClient.SystemautoscalerV1beta1().PodScales(namespace).Delete(ctx, podScale2.Name, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			err = kubeClient.CoreV1().Pods(namespace).Delete(ctx, pod2.Name, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			err = saClient.SystemautoscalerV1beta1().ServiceLevelAgreements(namespace).Delete(ctx, sla.Name, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			err = kubeClient.CoreV1().Services(namespace).Delete(ctx, svc.Name, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

		})
	})
})
