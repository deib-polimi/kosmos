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
)

// syncHandler compares the actual state with the desired, and attempts to
// converge the two.
func (c *Controller) syncServiceLevelAgreement(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)

	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the SLA resource with this namespace/name
	sla, err := c.listers.ServiceLevelAgreements(namespace).Get(name)

	if err != nil {
		// The SLA resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			// PodScale cleanup is achieved via OwnerReferences
			utilruntime.HandleError(fmt.Errorf("ServiceLevelAgreement '%s' in work queue no longer exists", key))
			return nil
		}
		return err
	}

	// Get all desired services to track matching the SLA selector inside the namespace
	serviceSelector := labels.Set(sla.Spec.ServiceSelector.MatchLabels).AsSelector()
	desired, err := c.listers.Services(namespace).List(serviceSelector)

	if err != nil {
		utilruntime.HandleError(fmt.Errorf("error while getting Services to track in Namespace '%s'", namespace))
		return nil
	}

	// Get all the services currently tracked by the SLA
	trackedSelector := labels.Set(map[string]string{
		SubjectToLabel: name,
	}).AsSelector()
	actual, err := c.listers.Services(namespace).List(trackedSelector)

	if err != nil {
		utilruntime.HandleError(fmt.Errorf("error while getting Services tracked in Namespace '%s'", namespace))
		return nil
	}

	for _, service := range desired {

		// TODO: Decide what happens if service matches a SLA but already have one and decide a tracking mechanism
		// Do nothing if the service is already tracked by the controller
		//_, ok := service.Labels[SubjectToLabel]
		// 			 At the moment, the SLA considered is the first match.
		//if ok {
		//	klog.V(4).Infof("Service %s is already tracked by %s", service.GetName(), name)
		//	return nil
		//}

		// adjust Service's PodScale according to its Pods
		err = c.syncService(namespace, service, sla)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("error while syncing PodScales for Service '%s'", service.GetName()))
			utilruntime.HandleError(err)
			return nil
		}

		// keep track of the SLA applied to the Service
		service.Labels[SubjectToLabel] = sla.GetName()

		_, err = c.kubeClientset.CoreV1().Services(namespace).Update(context.TODO(), service, metav1.UpdateOptions{})
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("error while updating Service labels of '%s'", service.GetName()))
			utilruntime.HandleError(err)
			return nil
		}
	}

	// Once the service, pod and podscales adhere to the desired state derived from SLA
	// delete old PodScale without a Service matched due to a change in ServiceSelector
	err = c.handleServiceSelectorChange(actual, desired, namespace)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("error while cleaning PodScales due to a ServiceSelector change"))
		return nil
	}

	c.recorder.Event(sla, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

// handleServiceSelectorChange performs a resource cleanup on service no longer tracked by a ServiceLevelAgreement.
func (c *Controller) handleServiceSelectorChange(actual []*corev1.Service, desired []*corev1.Service, namespace string) error {
	for _, service := range actual {
		if utils.ContainsService(desired, service) {
			continue
		}
		// get all podscales currently associated to a Service
		podscaleSelector := labels.Set(service.Spec.Selector).AsSelector()
		podscales, err := c.listers.PodScaleLister.List(podscaleSelector)

		if err != nil {
			utilruntime.HandleError(fmt.Errorf("error while getting PodScales for Service '%s'", service.GetName()))
			return nil
		}

		for _, p := range podscales {
			err := c.podScalesClientset.SystemautoscalerV1beta1().PodScales(namespace).Delete(context.TODO(), p.Name, metav1.DeleteOptions{})
			if err != nil {
				utilruntime.HandleError(fmt.Errorf("error while deleting PodScale for Service '%s'", service.GetName()))
				return nil
			}
		}
	}
	return nil
}

// syncService keeps a Service up to date with the corresponding ServiceLevelAgreement
// by creating and deleting the corresponding `PodScale` resources. It uses the `Selector`
// to retrive the corresponding `Pod` and `PodScale`. The `Pod` resources are used as
// a desired state so `PodScale` are changed accordingly.
func (c *Controller) syncService(namespace string, service *corev1.Service, sla *v1beta1.ServiceLevelAgreement) error {
	label := labels.Set(service.Spec.Selector)
	pods, err := c.listers.PodLister.List(label.AsSelector())

	if err != nil {
		utilruntime.HandleError(fmt.Errorf("error while getting Pods for Service '%s'", service.GetName()))
		return nil
	}

	podscales, err := c.listers.PodScaleLister.List(label.AsSelector())

	if err != nil {
		utilruntime.HandleError(fmt.Errorf("error while getting PodScales for Service '%s'", service.GetName()))
		return nil
	}

	stateDiff := utils.DiffPods(pods, podscales)

	for _, pod := range stateDiff.AddList {
		//TODO: change when a policy to handle other QOS class will be discussed
		if pod.Status.QOSClass != corev1.PodQOSGuaranteed {
			c.recorder.Eventf(pod, corev1.EventTypeWarning, QOSNotSupported, "Unsupported QOS for Pod %s/%s: ", pod.Namespace, pod.Name, pod.Status.QOSClass)
			continue
		}
		podscale := NewPodScale(pod, sla, label)

		_, err := c.podScalesClientset.SystemautoscalerV1beta1().PodScales(namespace).Create(context.TODO(), podscale, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			utilruntime.HandleError(fmt.Errorf("error while creating PodScale for Pod '%s'", podscale.GetName()))
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

// NewPodScale creates a new PodScale resource using the corresponding Pod and ServiceLevelAgreement infos.
// The SLA is the resource Owner in order to enable garbage collection on its deletion.
func NewPodScale(pod *corev1.Pod, sla *v1beta1.ServiceLevelAgreement, selectorLabels labels.Set) *v1beta1.PodScale {
	podLabels := make(labels.Set)
	for k, v := range selectorLabels {
		podLabels[k] = v
	}
	podLabels["node"] = pod.Spec.NodeName
	return &v1beta1.PodScale{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "systemautoscaler.polimi.it/v1beta1",
			Kind:       "PodScale",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-" + pod.GetName(),
			Namespace: sla.Namespace,
			Labels:    podLabels,
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
			SLARef: v1beta1.SLARef{
				Name:      sla.GetName(),
				Namespace: sla.GetNamespace(),
			},
			PodRef: v1beta1.PodRef{
				Name:      pod.GetName(),
				Namespace: pod.GetNamespace(),
			},
			DesiredResources: sla.Spec.DefaultResources,
		},
		Status: v1beta1.PodScaleStatus{
			ActualResources: sla.Spec.DefaultResources,
		},
	}
}
