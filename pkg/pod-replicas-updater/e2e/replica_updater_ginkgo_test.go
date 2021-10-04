package e2e_test

import (
	"context"

	sa "github.com/lterrac/system-autoscaler/pkg/apis/systemautoscaler/v1beta1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Replica updater controller", func() {
	Context("With an application deployed inside the cluster", func() {
		ctx := context.Background()

		It("Reduces the amount of replica when the application is under light load", func() {

			slaName := "foo-sla-rec1"
			appName := "foo-app-rec1"
			containerName := "container"

			labels := map[string]string{
				"app": "foo",
			}

			podLabels := map[string]string{
				"app": "foo",
			}

			selector := metav1.LabelSelector{
				MatchLabels:      podLabels,
				MatchExpressions: nil,
			}

			nReplicas := 10

			sla := newSLA(slaName, containerName, labels, 100)
			sla, err := saClient.SystemautoscalerV1beta1().ServiceLevelAgreements(namespace).Create(ctx, sla, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			svc := newService(appName, labels, podLabels)
			svc, err = kubeClient.CoreV1().Services(namespace).Create(ctx, svc, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			dp := newDeployment(appName, containerName, labels, selector, int32(nReplicas))
			dp, err = kubeClient.AppsV1().Deployments(namespace).Create(ctx, dp, metav1.CreateOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			// wait for pod to be created
			Eventually(func() bool {
				podList, err := getPodsForSvc(svc, namespace, *kubeClient)
				Expect(err).ShouldNot(HaveOccurred())
				return len(podList.Items) == nReplicas
			}, timeout, interval).Should(BeTrue())

			podList, err := getPodsForSvc(svc, namespace, *kubeClient)
			Expect(err).ShouldNot(HaveOccurred())

			var podScales []*sa.PodScale
			for _, pod := range podList.Items {
				podScale := newPodScale(sla, svc, &pod, labels)
				podScale, err = saClient.SystemautoscalerV1beta1().PodScales(namespace).Create(ctx, podScale, metav1.CreateOptions{})
				Expect(err).ShouldNot(HaveOccurred())
				podScales = append(podScales, podScale)
			}

			Eventually(func() bool {
				dp, err = kubeClient.AppsV1().Deployments(namespace).Get(ctx, appName, metav1.GetOptions{})
				Expect(err).ShouldNot(HaveOccurred())
				return *(dp.Spec.Replicas) < int32(nReplicas)
			}, timeout, interval).Should(BeTrue())

			for _, podScale := range podScales {
				err = saClient.SystemautoscalerV1beta1().PodScales(namespace).Delete(ctx, podScale.Name, metav1.DeleteOptions{})
				Expect(err).ShouldNot(HaveOccurred())
			}

			err = saClient.SystemautoscalerV1beta1().ServiceLevelAgreements(namespace).Delete(ctx, sla.Name, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			err = kubeClient.CoreV1().Services(namespace).Delete(ctx, svc.Name, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

			err = kubeClient.AppsV1().Deployments(namespace).Delete(ctx, dp.Name, metav1.DeleteOptions{})
			Expect(err).ShouldNot(HaveOccurred())

		})

		//It("Increases the amount of replica when the application is under heavy load", func() {
		//
		//	slaName := "foo-sla-rec1"
		//	appName := "foo-app-rec1"
		//	containerName := "container"
		//
		//	labels := map[string]string{
		//		"app": "foo",
		//	}
		//
		//	podLabels := map[string]string{
		//		"app": "foo",
		//	}
		//
		//	selector := metav1.LabelSelector{
		//		MatchLabels:      podLabels,
		//		MatchExpressions: nil,
		//	}
		//	nReplicas := 5
		//
		//	sla := newSLA(slaName, containerName, labels, 25)
		//	sla, err := saClient.SystemautoscalerV1beta1().ServiceLevelAgreements(namespace).Create(ctx, sla, metav1.CreateOptions{})
		//	Expect(err).ShouldNot(HaveOccurred())
		//
		//	svc := newService(appName, labels, podLabels)
		//	svc, err = kubeClient.CoreV1().Services(namespace).Create(ctx, svc, metav1.CreateOptions{})
		//	Expect(err).ShouldNot(HaveOccurred())
		//
		//	dp := newDeployment(appName, containerName, labels, selector, int32(nReplicas))
		//	dp, err = kubeClient.AppsV1().Deployments(namespace).Create(ctx, dp, metav1.CreateOptions{})
		//	Expect(err).ShouldNot(HaveOccurred())
		//
		//	// wait for pod to be created
		//	Eventually(func() bool {
		//		podList, err := getPodsForSvc(svc, namespace, *kubeClient)
		//		Expect(err).ShouldNot(HaveOccurred())
		//		klog.Info(podList.Size())
		//		return len(podList.Items) == nReplicas
		//	}, timeout, interval).Should(BeTrue())
		//
		//	podList, err := getPodsForSvc(svc, namespace, *kubeClient)
		//	Expect(err).ShouldNot(HaveOccurred())
		//
		//	var podScales []*sa.PodScale
		//	for _, pod := range podList.Items {
		//		podScale := newPodScale(sla, svc, &pod, labels)
		//		klog.Info(podScale)
		//		podScale, err = saClient.SystemautoscalerV1beta1().PodScales(namespace).Create(ctx, podScale, metav1.CreateOptions{})
		//		Expect(err).ShouldNot(HaveOccurred())
		//		podScales = append(podScales, podScale)
		//	}
		//
		//	Eventually(func() bool {
		//		dp, err = kubeClient.AppsV1().Deployments(namespace).Get(ctx, appName, metav1.GetOptions{})
		//		return *(dp.Spec.Replicas) > int32(nReplicas)
		//	}, timeout, interval).Should(BeTrue())
		//
		//	for _, podScale := range podScales {
		//		err = saClient.SystemautoscalerV1beta1().PodScales(namespace).Delete(ctx, podScale.Name, metav1.DeleteOptions{})
		//		Expect(err).ShouldNot(HaveOccurred())
		//	}
		//
		//	err = saClient.SystemautoscalerV1beta1().ServiceLevelAgreements(namespace).Delete(ctx, sla.Name, metav1.DeleteOptions{})
		//	Expect(err).ShouldNot(HaveOccurred())
		//
		//	err = kubeClient.CoreV1().Services(namespace).Delete(ctx, svc.Name, metav1.DeleteOptions{})
		//	Expect(err).ShouldNot(HaveOccurred())
		//
		//	err = kubeClient.AppsV1().Deployments(namespace).Delete(ctx, dp.Name, metav1.DeleteOptions{})
		//	Expect(err).ShouldNot(HaveOccurred())
		//
		//})
	})
})
