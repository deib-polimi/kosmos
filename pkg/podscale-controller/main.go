package main

import (
	"flag"
	"time"

	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	clientset "github.com/lterrac/system-autoscaler/pkg/generated/clientset/versioned"
	informers "github.com/lterrac/system-autoscaler/pkg/generated/informers/externalversions"
	podScaleController "github.com/lterrac/system-autoscaler/pkg/podscale-controller/pkg/controller"
	"github.com/lterrac/system-autoscaler/pkg/signals"
)

var (
	masterURL  string
	kubeconfig string
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	var cfg *rest.Config
	var err error

	if kubeconfig != "" {
		cfg, err = clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	} else {
		cfg, err = rest.InClusterConfig()
	}

	// creates the in-cluster config
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	systemAutoscalerClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building example clientset: %s", err.Error())
	}

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	systemAutoscalerInformerFactory := informers.NewSharedInformerFactory(systemAutoscalerClient, time.Second*30)

	// TODO: adjust arguments to recommender
	controller := podScaleController.NewController(
		kubeClient,
		systemAutoscalerClient,
		systemAutoscalerInformerFactory.Systemautoscaler().V1beta1().ServiceLevelAgreements(),
		systemAutoscalerInformerFactory.Systemautoscaler().V1beta1().PodScales(),
		kubeInformerFactory.Core().V1().Services(),
		kubeInformerFactory.Core().V1().Pods(),
	)

	// notice that there is no need to run Start methods in a separate goroutine. (i.e. go kubeInformerFactory.Start(stopCh)
	// Start method is non-blocking and runs all registered informers in a dedicated goroutine.
	kubeInformerFactory.Start(stopCh)
	systemAutoscalerInformerFactory.Start(stopCh)

	if err = controller.Run(2, stopCh); err != nil {
		klog.Fatalf("Error running controller: %s", err.Error())
	}
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
}
