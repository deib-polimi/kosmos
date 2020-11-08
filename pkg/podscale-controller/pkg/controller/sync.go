package controller

import (
	"context"
	"fmt"
	"github.com/lterrac/system-autoscaler/pkg/apis/systemautoscaler/v1beta1"
	"github.com/lterrac/system-autoscaler/pkg/podscale-controller/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// syncHandler compares the actual state with the desired, and attempts to
// converge the two.
func (c *Controller) syncSLAHandler(key string) error {
	// Convert the namespace/name string into a distinct slaNamespace and slaName
	slaNamespace, slaName, err := cache.SplitMetaNamespaceKey(key)

	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the SLA resource with this namespace/name
	sla, err := c.slasLister.ServiceLevelAgreements(slaNamespace).Get(slaName)

	if err != nil {
		// The SLA resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			// resource cleanup done via OwnerReferences
			utilruntime.HandleError(fmt.Errorf("ServiceLevelAgreement '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}
	// TODO: check here maybe for serviceSelector changes

	// Get all services matching the SLA selector inside the namespace
	serviceSelector := labels.Set(sla.Spec.ServiceSelector.MatchLabels).AsSelector()
	services, err := c.servicesLister.Services(slaNamespace).List(serviceSelector)

	klog.Info("service")
	for _,s := range services{
		klog.Info(s.Name)
	}

	if err != nil {
		utilruntime.HandleError(fmt.Errorf("error while getting Services in Namespace '%s'", slaNamespace))
		return nil
	}

	for _, service := range services {

		// TODO: Decide what happens if service matches a SLA but already have one and decide a tracking mechanism
		// Do nothing if the service is already tracked by the controller
		//_, ok := service.Labels[SubjectToLabel]
		// 			 At the moment, the SLA considered is the first match.
		//if ok {
		//	klog.V(4).Infof("Service %s is already tracked by %s", service.GetName(), slaName)
		//	return nil
		//}

		err = c.syncService(slaNamespace, service, sla)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("error while syncing PodScales for Service '%s'", service.GetName()))
			utilruntime.HandleError(err)
			return nil
		}
		service.Labels[SubjectToLabel] = sla.GetName()

		_, err = c.kubeClientset.CoreV1().Services(slaNamespace).Update(context.TODO(), service, metav1.UpdateOptions{})
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("error while updateing Service labels of '%s'", service.GetName()))
			utilruntime.HandleError(err)
			return nil
		}
	}

	c.recorder.Event(sla, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

// syncService keeps a Service up to date with the corresponding ServiceLevelAgreement
// by creating and deleting the corresponding `PodScale` resources. It uses the `Selector`
// to retrive the corresponding `Pod` and `PodScale`. The `Pod` resources are used as
// a desired state so `PodScale` are changed accordingly.
func (c *Controller) syncService(namespace string, service *corev1.Service, sla *v1beta1.ServiceLevelAgreement) error {
	label := labels.Set(service.Spec.Selector)
	pods, err := c.kubeClientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: label.AsSelector().String()})

	if err != nil {
		utilruntime.HandleError(fmt.Errorf("error while getting Pods for Service '%s'", service.GetName()))
		return nil
	}

	podscales, err := c.podScalesClientset.SystemautoscalerV1beta1().PodScales(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: label.AsSelector().String()})

	if err != nil {
		utilruntime.HandleError(fmt.Errorf("error while getting PodScales for Service '%s'", service.GetName()))
		return nil
	}

	stateDiff := utils.DiffPods(pods.Items, podscales.Items)

	for _, orphan := range stateDiff.AddList {

		podscale := &v1beta1.PodScale{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "systemautoscaler.polimi.it/v1beta1",
				Kind:       "PodScale",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "podscale-" + orphan.GetName(),
				Namespace: namespace,
				Labels: label,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "systemautoscaler.polimi.it/v1beta1",
						Kind:       "ServiceLevelAgreement",
						Name:       sla.GetName(),
						UID:        sla.GetUID(),
					},
				},
			},
			Spec: v1beta1.PodScaleSpec{
				SLA:              sla.GetName(),
				Pod:              orphan.GetName(),
				DesiredResources: sla.Spec.DefaultResources,
			},
			Status: v1beta1.PodScaleStatus{
				ActualResources: sla.Spec.DefaultResources,
			},
		}

		_, err := c.podScalesClientset.SystemautoscalerV1beta1().PodScales(namespace).Create(context.TODO(), podscale, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			utilruntime.HandleError(fmt.Errorf("error while creating PodScale for Pod '%s'", orphan.GetName()))
			utilruntime.HandleError(err)
			return nil
		}
	}

	for _, podscale := range stateDiff.DeleteList {

		err := c.podScalesClientset.SystemautoscalerV1beta1().PodScales(namespace).Delete(context.TODO(), podscale.Name, metav1.DeleteOptions{})
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("error while deleting PodScale for Pod '%s'", podscale.Name))
			utilruntime.HandleError(err)
			return nil
		}
	}

	return nil
}
