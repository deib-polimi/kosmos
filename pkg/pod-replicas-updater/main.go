package main

import (
	"flag"
	"time"

	"github.com/kubernetes-sigs/custom-metrics-apiserver/pkg/dynamicmapper"
	metricsgetter "github.com/lterrac/system-autoscaler/pkg/pod-autoscaler/pkg/metrics"
	metricsclient "k8s.io/metrics/pkg/client/custom_metrics"

	clientset "github.com/lterrac/system-autoscaler/pkg/generated/clientset/versioned"
	sainformers "github.com/lterrac/system-autoscaler/pkg/generated/informers/externalversions"
	informers2 "github.com/lterrac/system-autoscaler/pkg/informers"
	replicaupdater "github.com/lterrac/system-autoscaler/pkg/pod-replicas-updater/pkg"
	"github.com/lterrac/system-autoscaler/pkg/signals"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
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

	//TODO: tune refresh interval
	mapper, err := dynamicmapper.NewRESTMapper(kubernetesClient, time.Second)

	if err != nil {
		klog.Fatalf("Error building REST Mapper: %s", err.Error())
	}

	metricsGetter := metricsgetter.NewDefaultGetter(cfg, mapper, metricsclient.NewAvailableAPIsGetter(kubernetesClient))

	saInformerFactory := sainformers.NewSharedInformerFactory(client, time.Second*30)
	coreInformerFactory := informers.NewSharedInformerFactory(kubernetesClient, time.Second*30)

	// TODO: check name of this variable
	informers := informers2.Informers{
		Pod:                   coreInformerFactory.Core().V1().Pods(),
		Node:                  coreInformerFactory.Core().V1().Nodes(),
		Service:               coreInformerFactory.Core().V1().Services(),
		PodScale:              saInformerFactory.Systemautoscaler().V1beta1().PodScales(),
		ServiceLevelAgreement: saInformerFactory.Systemautoscaler().V1beta1().ServiceLevelAgreements(),
	}

	// TODO: adjust arguments to recommender
	replicaUpdater := replicaupdater.NewController(
		kubernetesClient,
		client,
		informers,
		metricsGetter,
	)

	// notice that there is no need to run Start methods in a separate goroutine. (i.e. go kubeInformerFactory.Start(stopCh)
	// Start method is non-blocking and runs all registered sainformers in a dedicated goroutine.
	saInformerFactory.Start(stopCh)
	coreInformerFactory.Start(stopCh)

	if err = replicaUpdater.Run(1, stopCh); err != nil {
		klog.Fatalf("Error running recommender: %s", err.Error())
	}
	defer replicaUpdater.Shutdown()

	<-stopCh
	klog.Info("Shutting down workers")

}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
}
