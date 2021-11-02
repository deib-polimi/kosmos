package metricsgetter

import (
	"github.com/kubernetes-sigs/custom-metrics-apiserver/pkg/dynamicmapper"
	"github.com/lterrac/system-autoscaler/pkg/metrics-exposer/pkg/metrics"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/rest"
	metricsv1beta2 "k8s.io/metrics/pkg/apis/custom_metrics/v1beta2"
	metricsclient "k8s.io/metrics/pkg/client/custom_metrics"
)

// MetricGetter is used by the recommender to fetch Pod metrics
type MetricGetter interface {
	PodMetrics(p *corev1.Pod, metricType metrics.MetricType) (*metricsv1beta2.MetricValue, error)
	ServiceMetrics(s *corev1.Service, metricType metrics.MetricType) (*metricsv1beta2.MetricValue, error)
}

// DefaultGetter is the standard implementation of MetriGetter
type DefaultGetter struct {
	client metricsclient.CustomMetricsClient
}

// NewDefaultGetter creates a new DefaultGetter
func NewDefaultGetter(cfg *rest.Config, m *dynamicmapper.RegeneratingDiscoveryRESTMapper, ag metricsclient.AvailableAPIsGetter) *DefaultGetter {
	return &DefaultGetter{
		client: metricsclient.NewForConfig(cfg, m, ag),
	}
}

// PodMetrics retrieves the Pod metrics. metrics.All is not supported at the moment by metrics-exposer so don't use it
func (d *DefaultGetter) PodMetrics(p *corev1.Pod, metricType metrics.MetricType) (*metricsv1beta2.MetricValue, error) {
	return d.client.NamespacedMetrics(p.Namespace).GetForObject(corev1.SchemeGroupVersion.WithKind("Pod").GroupKind(), p.Name, metricType.String(), labels.Everything())
}

// PodMetrics retrieves the Pod metrics. metrics.All is not supported at the moment by metrics-exposer so don't use it
func (d *DefaultGetter) ServiceMetrics(s *corev1.Service, metricType metrics.MetricType) (*metricsv1beta2.MetricValue, error) {
	return d.client.NamespacedMetrics(s.Namespace).GetForObject(corev1.SchemeGroupVersion.WithKind("Service").GroupKind(), s.Name, metricType.String(), labels.Everything())
}

// FakeGetter is used to mock the custom metrics api, especially during e2e tests
type FakeGetter struct {
	ResponseTime int64
}

// GetMetrics always return a MetricValue of 5
func (d *FakeGetter) PodMetrics(p *corev1.Pod, metricType metrics.MetricType) (*metricsv1beta2.MetricValue, error) {
	return &metricsv1beta2.MetricValue{
		Value: *resource.NewQuantity(d.ResponseTime, resource.BinarySI),
	}, nil
}

// PodMetrics retrieves the Pod metrics. metrics.All is not supported at the moment by metrics-exposer so don't use it
func (d *FakeGetter) ServiceMetrics(s *corev1.Service, metricType metrics.MetricType) (*metricsv1beta2.MetricValue, error) {
	return &metricsv1beta2.MetricValue{
		Value: *resource.NewQuantity(d.ResponseTime, resource.BinarySI),
	}, nil
}
