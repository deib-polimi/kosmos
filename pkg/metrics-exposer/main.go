package main

import (
	"flag"
	"os"
	"time"

	sainformers "github.com/lterrac/system-autoscaler/pkg/informers"
	"github.com/lterrac/system-autoscaler/pkg/signals"

	coreinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
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
	informers sainformers.Informers
}

func (a *ResponseTimeMetricsAdapter) makeProviderOrDie(informers sainformers.Informers) provider.CustomMetricsProvider {
	client, err := a.DynamicClient()
	if err != nil {
		klog.Fatalf("unable to construct dynamic client: %v", err)
	}

	mapper, err := a.RESTMapper()
	if err != nil {
		klog.Fatalf("unable to construct discovery REST mapper: %v", err)
	}

	return rtprovider.NewResponseTimeMetricsProvider(client, mapper, informers)
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

	coreInformerFactory := coreinformers.NewSharedInformerFactory(kubernetesClient, time.Second*30)

	informers := sainformers.Informers{
		Pod: coreInformerFactory.Core().V1().Pods(),
	}

	coreInformerFactory.Start(stopCh)

	go informers.Pod.Informer().Run(stopCh)

	if ok := cache.WaitForCacheSync(stopCh, informers.Pod.Informer().HasSynced); !ok {
		klog.Fatalf("failed to wait for caches to sync")
	}

	responseTimeMetricsProvider := cmd.makeProviderOrDie(informers)
	cmd.WithCustomMetrics(responseTimeMetricsProvider)

	if err := cmd.Run(stopCh); err != nil {
		klog.Fatalf("unable to run custom metrics adapter: %v", err)
	}
}
