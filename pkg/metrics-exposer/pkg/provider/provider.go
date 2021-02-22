package provider

import (
	"errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"sync"
	"time"

	apierr "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/metrics/pkg/apis/custom_metrics"

	"github.com/kubernetes-sigs/custom-metrics-apiserver/pkg/provider"
	"github.com/kubernetes-sigs/custom-metrics-apiserver/pkg/provider/helpers"
	"github.com/lterrac/system-autoscaler/pkg/informers"
	"github.com/lterrac/system-autoscaler/pkg/pod-autoscaler/pkg/recommender"
)

// CustomMetricResource wraps provider.CustomMetricInfo in a struct which stores the Name and Namespace of the resource
// So that we can accurately store and retrieve the metric as if this were an actual metrics server.
type CustomMetricResource struct {
	provider.CustomMetricInfo
	types.NamespacedName
}

type metricValue struct {
	labels labels.Set
	value  resource.Quantity
}

// responseTimeMetricsProvider is a sample implementation of provider.MetricsProvider which stores a map of fake metrics
type responseTimeMetricsProvider struct {
	client       dynamic.Interface
	mapper       apimeta.RESTMapper
	metricClient *recommender.Client
	informers    informers.Informers
	cacheLock    sync.RWMutex
	cache        map[CustomMetricResource]metricValue
}

// NewResponseTimeMetricsProvider returns an instance of responseTimeMetricsProvider
func NewResponseTimeMetricsProvider(client dynamic.Interface, mapper apimeta.RESTMapper, informers informers.Informers, stopCh <-chan struct{}) provider.CustomMetricsProvider {
	p := &responseTimeMetricsProvider{
		client:       client,
		mapper:       mapper,
		metricClient: recommender.NewMetricClient(),
		informers:    informers,
		cache:        make(map[CustomMetricResource]metricValue),
	}

	go wait.Until(p.updateMetrics, time.Second, stopCh)

	return p
}

// valueFor is a helper function to get just the value of a specific metric
func (p *responseTimeMetricsProvider) valueFor(info provider.CustomMetricInfo, namespacedName types.NamespacedName, metricSelector labels.Selector) (resource.Quantity, error) {

	info, _, err := info.Normalized(p.mapper)
	if err != nil {
		return resource.Quantity{}, err
	}

	metricInfo := CustomMetricResource{
		CustomMetricInfo: info,
		NamespacedName:   namespacedName,
	}

	value, ok := p.cache[metricInfo]
	if !ok {
		return resource.Quantity{}, errors.New("metric not in cache, failed to retrieve metrics")
	}

	return value.value, nil
}

// metricFor is a helper function which formats a value, metric, and object info into a MetricValue which can be returned by the metrics API
func (p *responseTimeMetricsProvider) metricFor(value resource.Quantity, name types.NamespacedName, selector labels.Selector, info provider.CustomMetricInfo, metricSelector labels.Selector) (*custom_metrics.MetricValue, error) {
	objRef, err := helpers.ReferenceFor(p.mapper, name, info)
	if err != nil {
		return nil, err
	}

	metric := &custom_metrics.MetricValue{
		DescribedObject: objRef,
		Metric: custom_metrics.MetricIdentifier{
			Name: info.Metric,
		},
		Timestamp: metav1.Time{Time: time.Now()},
		Value:     value,
	}

	if len(metricSelector.String()) > 0 {
		sel, err := metav1.ParseToLabelSelector(metricSelector.String())
		if err != nil {
			return nil, err
		}
		metric.Metric.Selector = sel
	}

	return metric, nil
}

// metricsFor is a wrapper used by GetMetricBySelector to format several metrics which match a resource selector
func (p *responseTimeMetricsProvider) metricsFor(namespace string, selector labels.Selector, info provider.CustomMetricInfo, metricSelector labels.Selector) (*custom_metrics.MetricValueList, error) {
	names, err := helpers.ListObjectNames(p.mapper, p.client, namespace, selector, info)
	if err != nil {
		return nil, err
	}

	res := make([]custom_metrics.MetricValue, 0, len(names))
	for _, name := range names {
		namespacedName := types.NamespacedName{Name: name, Namespace: namespace}
		value, err := p.valueFor(info, namespacedName, metricSelector)
		if err != nil {
			if apierr.IsNotFound(err) {
				continue
			}
			return nil, err
		}

		metric, err := p.metricFor(value, namespacedName, selector, info, metricSelector)
		if err != nil {
			return nil, err
		}
		res = append(res, *metric)
	}

	return &custom_metrics.MetricValueList{
		Items: res,
	}, nil
}

func (p *responseTimeMetricsProvider) GetMetricByName(name types.NamespacedName, info provider.CustomMetricInfo, metricSelector labels.Selector) (*custom_metrics.MetricValue, error) {

	p.cacheLock.RLock()
	defer p.cacheLock.RUnlock()

	value, err := p.valueFor(info, name, metricSelector)
	if err != nil {
		return nil, err
	}
	return p.metricFor(value, name, labels.Everything(), info, metricSelector)
}

func (p *responseTimeMetricsProvider) GetMetricBySelector(namespace string, selector labels.Selector, info provider.CustomMetricInfo, metricSelector labels.Selector) (*custom_metrics.MetricValueList, error) {

	p.cacheLock.RLock()
	defer p.cacheLock.RUnlock()

	return p.metricsFor(namespace, selector, info, metricSelector)
}

func (p *responseTimeMetricsProvider) ListAllMetrics() []provider.CustomMetricInfo {

	p.cacheLock.RLock()
	defer p.cacheLock.RUnlock()

	// Get unique CustomMetricInfos from wrapper CustomMetricResources
	infos := make(map[provider.CustomMetricInfo]struct{})
	for r := range p.cache {
		infos[r.CustomMetricInfo] = struct{}{}
	}

	// Build slice of CustomMetricInfos to be returns
	metrics := make([]provider.CustomMetricInfo, 0, len(infos))
	for info := range infos {
		metrics = append(metrics, info)
	}

	return metrics
}
