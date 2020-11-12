package controller

import (
	"github.com/lterrac/system-autoscaler/pkg/apis/systemautoscaler/v1beta1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

func (c *Controller) handleServiceLevelAgreementAdd(new interface{}) {
	sla, _ := new.(v1beta1.ServiceLevelAgreement)
	klog.Info("Add ServiceLevelAgreement ", sla.Name, " in namespace ", sla.Namespace)
	c.enqueueServiceLevelAgreement(new)
}

func (c *Controller) handleServiceLevelAgreementDeletion(old interface{}) {
	sla, _ := old.(v1beta1.ServiceLevelAgreement)
	klog.Info("Delete ServiceLevelAgreement ", sla.Name, " in namespace ", sla.Namespace)
	c.enqueueServiceLevelAgreement(old)
}
func (c *Controller) handleServiceLevelAgreementUpdate(old, new interface{}) {
	sla, _ := old.(v1beta1.ServiceLevelAgreement)
	klog.Info("Update ServiceLevelAgreement ", sla.Name, " in namespace ", sla.Namespace)
	c.enqueueServiceLevelAgreement(new)
}

// enqueueServiceLevelAgreement takes a ServiceLevelAgreement resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than ServiceLevelAgreement.
func (c *Controller) enqueueServiceLevelAgreement(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.slasworkqueue.Add(key)
}
