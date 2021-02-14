package controller

func (c *Controller) handleServiceLevelAgreementAdd(new interface{}) {
	c.slasworkqueue.Enqueue(new)
}

func (c *Controller) handleServiceLevelAgreementDeletion(old interface{}) {
	c.slasworkqueue.Enqueue(old)
}

func (c *Controller) handleServiceLevelAgreementUpdate(old, new interface{}) {
	c.slasworkqueue.Enqueue(new)
}
