package provider

import (
	"fmt"

	"github.com/kubernetes-sigs/custom-metrics-apiserver/pkg/provider"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

// updateMetrics updates the map of metrics
// for now, it updates the metrics for pods and services
func (p *responseTimeMetricsProvider) updateMetrics() {

	// TODO: retrieve the all the pods
	containerScales, err := p.informers.ContainerScale.Lister().List(labels.Everything())
	if err != nil {
		klog.Error("failed to retrieve to retrieve the container scales")
		return
	}

	// serviceMetricsMap[{namespace}][{name}] -> metrics
	serviceMetricsMap := make(map[string]map[string][]*resource.Quantity)

	for _, containerScale := range containerScales {

		podNamespace := containerScale.Spec.PodRef.Namespace
		podName := containerScale.Spec.PodRef.Name

		serviceNamespace := containerScale.Spec.ServiceRef.Namespace
		serviceName := containerScale.Spec.ServiceRef.Name

		pod, err := p.informers.Pod.Lister().Pods(podNamespace).Get(podName)
		if err != nil {
			klog.Errorf("failed to retrieve to retrieve the pod with name %s and namespace %s", podName, podNamespace)
			continue
		}

		metric, err := p.getMetricsForPod(pod)
		if err != nil {
			klog.Error("failed to retrieve the metrics for pod with name %s and namespace %s", podName, podNamespace)
			continue
		}

		groupResource := schema.ParseGroupResource("pod")

		info := provider.CustomMetricInfo{
			GroupResource: groupResource,
			Metric:        "response-time",
			Namespaced:    true,
		}

		info, _, err = info.Normalized(p.mapper)
		if err != nil {
			klog.Errorf("Error normalizing info: %s", err)
			continue
		}

		namespacedName := types.NamespacedName{
			Name:      podName,
			Namespace: podNamespace,
		}

		metricInfo := CustomMetricResource{
			CustomMetricInfo: info,
			NamespacedName:   namespacedName,
		}

		metricLabels := labels.Set{}

		value := metricValue{
			labels: metricLabels,
			value:  *metric,
		}

		p.setMetrics(metricInfo, value)

		// group metrics by service
		serviceMetrics := serviceMetricsMap[serviceNamespace][serviceName]
		if serviceMetrics == nil {
			serviceMetrics = make([]*resource.Quantity, 0)
		}

		if serviceMetricsMap[serviceNamespace] == nil {
			serviceMetricsMap[serviceNamespace] = make(map[string][]*resource.Quantity)
		}

		serviceMetricsMap[serviceNamespace][serviceName] = append(serviceMetrics, metric)

	}

	for namespace, nestedMap := range serviceMetricsMap {
		for name, metrics := range nestedMap {

			// Compute average
			sum := 0
			for _, metric := range metrics {
				sum += int(metric.MilliValue())
			}
			averageMetric := resource.NewMilliQuantity(int64(sum/len(metrics)), resource.BinarySI)

			groupResource := schema.ParseGroupResource("service")

			info := provider.CustomMetricInfo{
				GroupResource: groupResource,
				Metric:        "response-time",
				Namespaced:    true,
			}

			info, _, err = info.Normalized(p.mapper)
			if err != nil {
				klog.Errorf("Error normalizing info: %s", err)
				continue
			}

			namespacedName := types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			}

			metricInfo := CustomMetricResource{
				CustomMetricInfo: info,
				NamespacedName:   namespacedName,
			}

			metricLabels := labels.Set{}

			value := metricValue{
				labels: metricLabels,
				value:  *averageMetric,
			}

			p.setMetrics(metricInfo, value)

		}
	}

}

// getMetricsForPod retrieve the metrics of a pod
// TODO: Should we handle also other metrics besides the response time?
func (p *responseTimeMetricsProvider) getMetricsForPod(pod *corev1.Pod) (*resource.Quantity, error) {

	// Retrieve the metrics through the HTTP client
	value, err := p.metricClient.ResponseTime(pod)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve metrics for pod with name %s and namespace %s, error: %v", pod.Name, pod.Namespace, err)
	}
	return resource.NewMilliQuantity(int64(value["response_time"].(float64)), resource.BinarySI), nil

}

// setMetrics saves the metrics in the provider cache
func (p *responseTimeMetricsProvider) setMetrics(metricInfo CustomMetricResource, value metricValue) {
	p.cacheLock.RLock()
	defer p.cacheLock.RUnlock()
	p.cache[metricInfo] = value
}
