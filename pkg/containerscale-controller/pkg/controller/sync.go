package controller

import (
	"context"
	"fmt"
	"github.com/lterrac/system-autoscaler/pkg/apis/systemautoscaler/v1beta1"
	"github.com/lterrac/system-autoscaler/pkg/containerscale-controller/pkg/utils"
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
			// ContainerScale cleanup is achieved via OwnerReferences
			utilruntime.HandleError(fmt.Errorf("ServiceLevelAgreement '%s' in work queue no longer exists", key))
			return nil
		}
		return err
	}

	// Get all desired services to track matching the SLA selector inside the namespace
	serviceSelector := labels.Set(sla.Spec.Service.Selector.MatchLabels).AsSelector()
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

		// adjust Service's ContainerScale according to its Pods
		err = c.syncService(namespace, service, sla)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("error while syncing ContainerScales for Service '%s'", service.GetName()))
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

	// Once the service, pod and containerscales adhere to the desired state derived from SLA
	// delete old ContainerScale without a Service matched due to a change in ServiceSelector
	err = c.handleServiceSelectorChange(actual, desired, namespace)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("error while cleaning ContainerScales due to a ServiceSelector change"))
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
		// get all containerscales currently associated to a Service
		containerscaleSelector := labels.Set(service.Spec.Selector).AsSelector()
		containerscales, err := c.listers.ContainerScaleLister.List(containerscaleSelector)

		if err != nil {
			utilruntime.HandleError(fmt.Errorf("error while getting ContainerScales for Service '%s'", service.GetName()))
			return nil
		}

		for _, p := range containerscales {
			err := c.containerScalesClientset.SystemautoscalerV1beta1().ContainerScales(namespace).Delete(context.TODO(), p.Name, metav1.DeleteOptions{})
			if err != nil {
				utilruntime.HandleError(fmt.Errorf("error while deleting ContainerScale for Service '%s'", service.GetName()))
				return nil
			}
		}
	}
	return nil
}

// syncService keeps a Service up to date with the corresponding ServiceLevelAgreement
// by creating and deleting the corresponding `ContainerScale` resources. It uses the `Selector`
// to retrive the corresponding `Pod` and `ContainerScale`. The `Pod` resources are used as
// a desired state so `ContainerScale` are changed accordingly.
func (c *Controller) syncService(namespace string, service *corev1.Service, sla *v1beta1.ServiceLevelAgreement) error {
	label := labels.Set(service.Spec.Selector)
	pods, err := c.listers.PodLister.List(label.AsSelector())

	if err != nil {
		utilruntime.HandleError(fmt.Errorf("error while getting Pods for Service '%s'", service.GetName()))
		return nil
	}

	containerscales, err := c.listers.ContainerScaleLister.List(label.AsSelector())

	if err != nil {
		utilruntime.HandleError(fmt.Errorf("error while getting ContainerScales for Service '%s'", service.GetName()))
		return nil
	}

	stateDiff := utils.DiffPods(pods, containerscales)

	for _, pod := range stateDiff.AddList {
		//TODO: change when a policy to handle other QOS class will be discussed
		if pod.Status.QOSClass != corev1.PodQOSGuaranteed {
			c.recorder.Eventf(pod, corev1.EventTypeWarning, QOSNotSupported, "Unsupported QOS for Pod %s/%s: ", pod.Namespace, pod.Name, pod.Status.QOSClass)
			continue
		}
		containerscale := NewContainerScale(pod, sla, label)

		_, err := c.containerScalesClientset.SystemautoscalerV1beta1().ContainerScales(namespace).Create(context.TODO(), containerscale, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			utilruntime.HandleError(fmt.Errorf("error while creating ContainerScale for Pod '%s'", containerscale.GetName()))
			utilruntime.HandleError(err)
			return nil
		}

	}

	for _, containerscale := range stateDiff.DeleteList {

		err := c.containerScalesClientset.SystemautoscalerV1beta1().ContainerScales(namespace).Delete(context.TODO(), containerscale.Name, metav1.DeleteOptions{})
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("error while deleting ContainerScale for Pod '%s'", containerscale.Name))
			utilruntime.HandleError(err)
			return nil
		}
	}

	return nil
}

// NewContainerScale creates a new ContainerScale resource using the corresponding Pod and ServiceLevelAgreement infos.
// The SLA is the resource Owner in order to enable garbage collection on its deletion.
func NewContainerScale(pod *corev1.Pod, sla *v1beta1.ServiceLevelAgreement, selectorLabels labels.Set) *v1beta1.ContainerScale {
	podLabels := make(labels.Set)
	for k, v := range selectorLabels {
		podLabels[k] = v
	}
	podLabels["system.autoscaler/node"] = pod.Spec.NodeName
	return &v1beta1.ContainerScale{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "systemautoscaler.polimi.it/v1beta1",
			Kind:       "ContainerScale",
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
		Spec: v1beta1.ContainerScaleSpec{
			SLARef: v1beta1.SLARef{
				Name:      sla.GetName(),
				Namespace: sla.GetNamespace(),
			},
			PodRef: v1beta1.PodRef{
				Name:      pod.GetName(),
				Namespace: pod.GetNamespace(),
			},
			Container: sla.Spec.Service.Container,
			DesiredResources: sla.Spec.DefaultResources,
		},
		Status: v1beta1.ContainerScaleStatus{
			ActualResources: sla.Spec.DefaultResources,
		},
	}
}
