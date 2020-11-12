package controller

import (
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
)

func (c *Controller) handleServiceLevelAgreementAdd(new interface{}) {
	c.enqueueServiceLevelAgreement(new)
}

func (c *Controller) handleServiceLevelAgreementDeletion(old interface{}) {
	c.enqueueServiceLevelAgreement(old)
}

func (c *Controller) handleServiceLevelAgreementUpdate(old, new interface{}) {
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
