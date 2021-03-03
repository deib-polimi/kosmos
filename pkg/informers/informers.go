package informers

import (
	sainformers "github.com/lterrac/system-autoscaler/pkg/generated/informers/externalversions/systemautoscaler/v1beta1"
	salisters "github.com/lterrac/system-autoscaler/pkg/generated/listers/systemautoscaler/v1beta1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
)

type Informers struct {
	Pod                   coreinformers.PodInformer
	Node                  coreinformers.NodeInformer
	Service               coreinformers.ServiceInformer
	PodScale              sainformers.PodScaleInformer
	ServiceLevelAgreement sainformers.ServiceLevelAgreementInformer
}

func (i *Informers) GetListers() Listers {
	return Listers{
		i.Pod.Lister(),
		i.Node.Lister(),
		i.Service.Lister(),
		i.PodScale.Lister(),
		i.ServiceLevelAgreement.Lister(),
	}
}

type Listers struct {
	corelisters.PodLister
	corelisters.NodeLister
	corelisters.ServiceLister
	salisters.PodScaleLister
	salisters.ServiceLevelAgreementLister
}
