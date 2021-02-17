package main

import (
	"flag"
	"os"
	"time"

	informers2 "github.com/lterrac/system-autoscaler/pkg/informers"
	"github.com/lterrac/system-autoscaler/pkg/signals"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/component-base/logs"
	"k8s.io/klog/v2"

	"github.com/kubernetes-sigs/custom-metrics-apiserver/pkg/apiserver"
	basecmd "github.com/kubernetes-sigs/custom-metrics-apiserver/pkg/cmd"
	"github.com/kubernetes-sigs/custom-metrics-apiserver/pkg/provider"
	generatedopenapi "github.com/lterrac/system-autoscaler/pkg/metrics-exposer/pkg/generated/openapi"
	rtprovider "github.com/lterrac/system-autoscaler/pkg/metrics-exposer/pkg/provider"
	openapinamer "k8s.io/apiserver/pkg/endpoints/openapi"
	genericapiserver "k8s.io/apiserver/pkg/server"
)

var (
	masterURL  string
	kubeconfig string
)

// ResponseTimeMetricsAdapter contains a basic adapter used to serve custom metrics
type ResponseTimeMetricsAdapter struct {
	basecmd.AdapterBase
	informers informers2.Informers
}

func (a *ResponseTimeMetricsAdapter) makeProviderOrDie(informers informers2.Informers, kubeClient kubernetes.Interface) provider.CustomMetricsProvider {
	client, err := a.DynamicClient()
	if err != nil {
		klog.Fatalf("unable to construct dynamic client: %v", err)
	}

	mapper, err := a.RESTMapper()
	if err != nil {
		klog.Fatalf("unable to construct discovery REST mapper: %v", err)
	}

	return rtprovider.NewResponseTimeMetricsProvider(client, mapper, informers, kubeClient)
}

func main() {
	logs.InitLogs()
	defer logs.FlushLogs()
	stopCh := signals.SetupSignalHandler()

	cmd := &ResponseTimeMetricsAdapter{}

	cmd.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(generatedopenapi.GetOpenAPIDefinitions, openapinamer.NewDefinitionNamer(apiserver.Scheme))
	cmd.OpenAPIConfig.Info.Title = "response-time-metrics-adapter"
	cmd.OpenAPIConfig.Info.Version = "0.1.0"

	cmd.Flags().AddGoFlagSet(flag.CommandLine) // make sure we get the klog flags
	cmd.Flags().Parse(os.Args)

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	kubernetesClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building example clientset: %s", err.Error())
	}

	coreInformerFactory := informers.NewSharedInformerFactory(kubernetesClient, time.Second*30)
	coreInformerFactory.Start(stopCh)

	informers := informers2.Informers{
		Pod: coreInformerFactory.Core().V1().Pods(),
	}

	// if ok := cache.WaitForCacheSync(stopCh, coreInformerFactory.Core().V1().Pods().Informer().HasSynced); !ok {
	// 	klog.Error(fmt.Errorf("failed to wait for caches to sync"))
	// 	return
	// }
	// klog.Info("aaa")

	testProvider := cmd.makeProviderOrDie(informers, kubernetesClient)
	cmd.WithCustomMetrics(testProvider)
	klog.Info("aaa")

	if err := cmd.Run(stopCh); err != nil {
		klog.Fatalf("unable to run custom metrics adapter: %v", err)
	}
	klog.Info("aaa")

	<-stopCh
	klog.Info("Shutting down workers")
}
