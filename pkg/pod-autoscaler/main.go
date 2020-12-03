package main

import (
	"flag"
	"github.com/lterrac/system-autoscaler/pkg/podscale-controller/pkg/types"
	"k8s.io/client-go/kubernetes"
	"time"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	clientset "github.com/lterrac/system-autoscaler/pkg/generated/clientset/versioned"
	informers "github.com/lterrac/system-autoscaler/pkg/generated/informers/externalversions"
	"github.com/lterrac/system-autoscaler/pkg/pod-autoscaler/pkg/recommender"
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

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	client, err := clientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building example clientset: %s", err.Error())
	}
	kubernetesClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building example clientset: %s", err.Error())
	}

	crdInformerFactory := informers.NewSharedInformerFactory(client, time.Second*30)

	out := make(chan types.NodeScales, 10)

	// TODO: adjust arguments to recommender
	controller := recommender.NewController(
		kubernetesClient,
		client,
		crdInformerFactory.Systemautoscaler().V1beta1().PodScales(),
		crdInformerFactory.Systemautoscaler().V1beta1().ServiceLevelAgreements(),
		out,
	)

	// notice that there is no need to run Start methods in a separate goroutine. (i.e. go kubeInformerFactory.Start(stopCh)
	// Start method is non-blocking and runs all registered informers in a dedicated goroutine.
	crdInformerFactory.Start(stopCh)

	if err = controller.Run(1, stopCh); err != nil {
		klog.Fatalf("Error running controller: %s", err.Error())
	}
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
}
