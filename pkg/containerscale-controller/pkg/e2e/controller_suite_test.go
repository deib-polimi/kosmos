package e2e_test

import (
	"github.com/lterrac/system-autoscaler/pkg/informers"
	"testing"
	"time"

	sainformers "github.com/lterrac/system-autoscaler/pkg/generated/informers/externalversions"
	"github.com/lterrac/system-autoscaler/pkg/signals"
	coreinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"

	clientset "github.com/lterrac/system-autoscaler/pkg/generated/clientset/versioned"
	. "github.com/onsi/gomega"

	systemautoscalerv1beta1 "github.com/lterrac/system-autoscaler/pkg/apis/systemautoscaler/v1beta1"
	systemautoscaler "github.com/lterrac/system-autoscaler/pkg/generated/clientset/versioned"
	containerscale "github.com/lterrac/system-autoscaler/pkg/containerscale-controller/pkg/controller"
	. "github.com/onsi/ginkgo"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var cfg *rest.Config
var testEnv *envtest.Environment
var kubeClient *kubernetes.Clientset
var saClient *systemautoscaler.Clientset

var _ = BeforeSuite(func(done Done) {
	useCluster := true

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		UseExistingCluster:       &useCluster,
		AttachControlPlaneOutput: true,
	}

	var err error

	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	err = systemautoscalerv1beta1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	By("bootstrapping clients")

	kubeClient = kubernetes.NewForConfigOrDie(cfg)
	saClient = clientset.NewForConfigOrDie(cfg)

	By("bootstrapping informers")

	crdInformerFactory := sainformers.NewSharedInformerFactory(saClient, time.Second*30)
	coreInformerFactory := coreinformers.NewSharedInformerFactory(kubeClient, time.Second*30)

	By("creating informers")
	informers := informers.Informers{
		Pod:                   coreInformerFactory.Core().V1().Pods(),
		Node:                  coreInformerFactory.Core().V1().Nodes(),
		Service:               coreInformerFactory.Core().V1().Services(),
		ContainerScale:              crdInformerFactory.Systemautoscaler().V1beta1().ContainerScales(),
		ServiceLevelAgreement: crdInformerFactory.Systemautoscaler().V1beta1().ServiceLevelAgreements(),
	}

	By("bootstrapping controller")

	controller := containerscale.NewController(
		kubeClient,
		saClient,
		informers,
	)

	By("starting informers")

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()
	Expect(err).ToNot(HaveOccurred())
	crdInformerFactory.Start(stopCh)
	coreInformerFactory.Start(stopCh)

	By("starting controller")

	go func() {
		err = controller.Run(2, stopCh)
		Expect(err).ToNot(HaveOccurred())
	}()

	close(done)
}, 15)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})
