package controller_test

import (
	"context"
	systemautoscaler "github.com/lterrac/system-autoscaler/pkg/apis/systemautoscaler/v1beta1"
	. "github.com/lterrac/system-autoscaler/pkg/podscale-controller/pkg/controller"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	"time"
)


var _ = Describe("PodScale controller", func() {
	Context("With an application deployed inside the cluster", func() {
		ns := "default"
		ctx := context.Background()
		slaName := "foo-sla"
		appName := "foo-app"

		labels := map[string]string{
			"app": "foo",
		}

		sla := newSLA(slaName, labels)
		svc, pod := newApplication(appName, labels)
		Expect(kubeClient.CoreV1().Services(ns).Create(ctx, svc, metav1.CreateOptions{})).Should(Succeed())
		Expect(kubeClient.CoreV1().Pods(ns).Create(ctx, pod, metav1.CreateOptions{})).Should(Succeed())
		Expect(systemAutoscalerClient.SystemautoscalerV1beta1().ServiceLevelAgreements(ns).Create(ctx, sla, metav1.CreateOptions{})).Should(Succeed())

		expectedPodScale := NewPodScale(pod, sla, svc.Spec.Selector)


		expectedLabels := map[string]string{
			"app": "foo",
			SubjectToLabel: slaName,
		}

		expectedSvc, _ := newApplication(appName, expectedLabels)

		const timeout = 60 * time.Second
		const interval = 1 * time.Second

		Eventually(func() bool {
			actual, err := kubeClient.CoreV1().Services(ns).Get(ctx, svc.GetName(), metav1.GetOptions{})
			return err == nil && reflect.DeepEqual(actual, expectedSvc)
		}, timeout, interval).Should(BeTrue())

		Eventually(func() bool {
			actual, err := systemAutoscalerClient.SystemautoscalerV1beta1().ServiceLevelAgreements(ns).Get(ctx, expectedPodScale.GetName(), metav1.GetOptions{})
			return err == nil && reflect.DeepEqual(actual, expectedPodScale)
		}, timeout, interval).Should(BeTrue())
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
			},
		}, &corev1.Pod{
			TypeMeta: metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "pods"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foobar",
				Namespace: metav1.NamespaceDefault,
				Labels:    podLabels,
			},
		}
}
