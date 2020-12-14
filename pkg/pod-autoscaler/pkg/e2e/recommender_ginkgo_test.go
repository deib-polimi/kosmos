package e2e_test

import (
	"context"
	"math/rand"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("PodScale controller", func() {
	Context("With an application deployed inside the cluster", func() {
		ctx := context.Background()

		BeforeEach(func() {
			namespace = strconv.Itoa(rand.Intn(time.Now().Nanosecond()*200)) + "e2e"
			By("creating testing namespace")
			nsSpec := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
			_, err := kubeClient.CoreV1().Namespaces().Create(ctx, nsSpec, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			By("deleting testing namespace")
			err := kubeClient.CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

		})

		// Test 1: Add a service and verify that it is monitored.
		// Test 2: Add a service and verify that it is monitored, than delete it and verify it is not monitored.
		// Test 3: Verify that pods running in a node, receives recommendations all together.
		It("Monitor pods whenever a new pod scale is created", func() {

			slaName := "foo-sla"
			appName := "foo-app"

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

			slaName := "foo-sla"
			appName := "foo-app"

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

			pod1 := newPod("replica1", podLabels)
			pod1, err = kubeClient.CoreV1().Pods(namespace).Create(ctx, pod1, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

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

			pod2 := newPod("replica2", podLabels)
			pod2, err = kubeClient.CoreV1().Pods(namespace).Create(ctx, pod2, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

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
				for i:=0; i < x; i++ {
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
