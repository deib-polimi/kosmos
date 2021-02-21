package main

import (
	"flag"
	"time"

	"github.com/kubernetes-sigs/custom-metrics-apiserver/pkg/dynamicmapper"

	informers2 "github.com/lterrac/system-autoscaler/pkg/informers"

	"github.com/lterrac/system-autoscaler/pkg/containerscale-controller/pkg/types"
	sainformers "github.com/lterrac/system-autoscaler/pkg/generated/informers/externalversions"
	cm "github.com/lterrac/system-autoscaler/pkg/pod-autoscaler/pkg/contention-manager"
	resupd "github.com/lterrac/system-autoscaler/pkg/pod-autoscaler/pkg/pod-resource-updater"
	metricsclient "k8s.io/metrics/pkg/client/custom_metrics"

	coreinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	clientset "github.com/lterrac/system-autoscaler/pkg/generated/clientset/versioned"
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
		klog.Fatalf("Error building system-autoscaler clientset: %s", err.Error())
	}

	kubernetesClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	//TODO: tune refresh interval
	mapper, err := dynamicmapper.NewRESTMapper(kubernetesClient, time.Second)

	if err != nil {
		klog.Fatalf("Error building REST Mapper: %s", err.Error())
	}

	metricsGetter := recommender.NewDefaultGetter(cfg, mapper, metricsclient.NewAvailableAPIsGetter(kubernetesClient))

	saInformerFactory := sainformers.NewSharedInformerFactory(client, time.Second*30)
	coreInformerFactory := coreinformers.NewSharedInformerFactory(kubernetesClient, time.Second*30)

	// TODO: Check name of this variable
	informers := informers2.Informers{
		Pod:                   coreInformerFactory.Core().V1().Pods(),
		Node:                  coreInformerFactory.Core().V1().Nodes(),
		Service:               coreInformerFactory.Core().V1().Services(),
		ContainerScale:        saInformerFactory.Systemautoscaler().V1beta1().ContainerScales(),
		ServiceLevelAgreement: saInformerFactory.Systemautoscaler().V1beta1().ServiceLevelAgreements(),
	}

	//TODO: should be renamed
	//TODO: we should try without buffer
	recommenderOut := make(chan types.NodeScales, 100)
	contentionManagerOut := make(chan types.NodeScales, 100)

	// TODO: adjust arguments to recommender
	recommenderController := recommender.NewController(
		kubernetesClient,
		client,
		metricsGetter,
		informers,
		recommenderOut,
	)

	contentionManagerController := cm.NewController(
		kubernetesClient,
		client,
		informers,
		recommenderOut,
		contentionManagerOut,
	)

	updaterController := resupd.NewController(
		kubernetesClient,
		client,
		informers,
		contentionManagerOut,
	)

	// notice that there is no need to run Start methods in a separate goroutine. (i.e. go kubeInformerFactory.Start(stopCh)
	// Start method is non-blocking and runs all registered safactory in a dedicated goroutine.
	saInformerFactory.Start(stopCh)
	coreInformerFactory.Start(stopCh)

	if err = recommenderController.Run(1, stopCh); err != nil {
		klog.Fatalf("Error running recommender: %s", err.Error())
	}
	defer recommenderController.Shutdown()

	//c := recommender.NewMetricClient()
	//server := serverMock()
	//c.Host = server.URL[7:]
	//recommenderController.MetricClient = c

	if err = contentionManagerController.Run(2, stopCh); err != nil {
		klog.Fatalf("Error running update controller: %s", err.Error())
	}
	defer contentionManagerController.Shutdown()

	if err = updaterController.Run(2, stopCh); err != nil {
		klog.Fatalf("Error running update controller: %s", err.Error())
	}
	defer updaterController.Shutdown()

	<-stopCh
	klog.Info("Shutting down workers")

}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
}
