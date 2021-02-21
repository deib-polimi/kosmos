package recommender

import (
	"github.com/kubernetes-sigs/custom-metrics-apiserver/pkg/dynamicmapper"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/rest"
	metricsv1beta2 "k8s.io/metrics/pkg/apis/custom_metrics/v1beta2"
	metricsclient "k8s.io/metrics/pkg/client/custom_metrics"
)

// MetricGetter is used by the recommender to fetch Pod metrics
type MetricGetter interface {
	GetMetrics(p *corev1.Pod) (*metricsv1beta2.MetricValue, error)
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

// GetMetrics retrieves the Pod metrics
func (d *DefaultGetter) GetMetrics(p *corev1.Pod) (*metricsv1beta2.MetricValue, error) {
	return d.client.NamespacedMetrics(p.Namespace).GetForObject(corev1.SchemeGroupVersion.WithKind("Pod").GroupKind(), p.Name, responseTime, labels.Everything())
}

// FakeGetter is used to mock the custom metrics api, especially during e2e tests
type FakeGetter struct{}

// GetMetrics always return a MetricValue of 5
func (d *FakeGetter) GetMetrics(p *corev1.Pod) (*metricsv1beta2.MetricValue, error) {
	return &metricsv1beta2.MetricValue{
		Value: *resource.NewQuantity(5, resource.BinarySI),
	}, nil
}
