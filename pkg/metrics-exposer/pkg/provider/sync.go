package provider

import (
	"fmt"

	"github.com/kubernetes-sigs/custom-metrics-apiserver/pkg/provider"
	"github.com/lterrac/system-autoscaler/pkg/metrics-exposer/pkg/metrics"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

// Metrics is the wrapper for Kubernetes resource metrics
type Metrics struct {
	ResponseTime *resource.Quantity
	RequestCount *resource.Quantity
	Throughput   *resource.Quantity
}

// updateMetrics updates the map of metrics
// for now, it updates the metrics for pods and services
func (p *responseTimeMetricsProvider) updateMetrics() {

	var podMetrics *Metrics
	var err error

	// TODO: retrieve the all the pods
	containerScales, err := p.informers.ContainerScale.Lister().List(labels.Everything())
	if err != nil {
		klog.Error("failed to retrieve to retrieve the container scales")
		return
	}

	serviceMetricsMap := make(map[string]map[string][]*Metrics)

	for _, containerScale := range containerScales {

		podNamespace := containerScale.Spec.PodRef.Namespace
		podName := containerScale.Spec.PodRef.Name

		serviceNamespace := containerScale.Spec.ServiceRef.Namespace
		serviceName := containerScale.Spec.ServiceRef.Name

		pod, err := p.informers.Pod.Lister().Pods(podNamespace).Get(podName)
		if err != nil {
			klog.Errorf("failed to retrieve the pod with name %s and namespace %s", podName, podNamespace)
			continue
		}

		podMetrics, err = p.PodMetrics(pod)

		if err != nil {
			klog.Errorf("failed to retrieve metrics for pod with name %s and namespace %s", podName, podNamespace)
			continue
		}

		err = p.updatePodMetric(podName, podNamespace, metrics.ResponseTime, *podMetrics.ResponseTime)

		if err != nil {
			klog.Errorf("error while updating response time for pod with name %s and namespace %s", podName, podNamespace)
			continue
		}

		err = p.updatePodMetric(podName, podNamespace, metrics.RequestCount, *podMetrics.RequestCount)

		if err != nil {
			klog.Errorf("error while updating request count for pod with name %s and namespace %s", podName, podNamespace)
			continue
		}

		err = p.updatePodMetric(podName, podNamespace, metrics.Throughput, *podMetrics.Throughput)

		if err != nil {
			klog.Errorf("error while updating throughput for pod with name %s and namespace %s", podName, podNamespace)
			continue
		}

		if _, ok := serviceMetricsMap[serviceNamespace]; !ok {
			serviceMetricsMap[serviceNamespace] = make(map[string][]*Metrics)
		}

		// group metrics by service
		serviceMetrics, ok := serviceMetricsMap[serviceNamespace][serviceName]

		if !ok {
			serviceMetrics = make([]*Metrics, 0)
		}

		serviceMetricsMap[serviceNamespace][serviceName] = append(serviceMetrics, podMetrics)

	}

	for namespace, nestedMap := range serviceMetricsMap {
		for name, serviceMetrics := range nestedMap {
			// Compute average
			responseTimeSum := 0
			requestCountSum := 0
			throughputSum := 0

			for _, metric := range serviceMetrics {
				requests := metric.RequestCount.Value()
				responseTimeSum += int(metric.ResponseTime.MilliValue()) * int(requests)
				requestCountSum += int(requests)
				throughputSum += int(metric.Throughput.MilliValue()) * int(requests)
			}
			var metricsValue *Metrics

			if requestCountSum == 0 {
				metricsValue = &Metrics{
					ResponseTime: resource.NewQuantity(0, resource.BinarySI),
					RequestCount: resource.NewQuantity(0, resource.BinarySI),
					Throughput:   resource.NewQuantity(0, resource.BinarySI),
				}
			} else {

				averageResponseTime := resource.NewMilliQuantity(int64(responseTimeSum/requestCountSum), resource.BinarySI)
				averageRequestCount := resource.NewQuantity(int64(requestCountSum), resource.BinarySI)
				averageThroughput := resource.NewMilliQuantity(int64(throughputSum/requestCountSum), resource.BinarySI)

				metricsValue = &Metrics{
					ResponseTime: averageResponseTime,
					RequestCount: averageRequestCount,
					Throughput:   averageThroughput,
				}
			}
			p.updateServiceMetric(name, namespace, metrics.ResponseTime, *metricsValue.ResponseTime)
			p.updateServiceMetric(name, namespace, metrics.RequestCount, *metricsValue.RequestCount)
			p.updateServiceMetric(name, namespace, metrics.Throughput, *metricsValue.Throughput)

		}
	}
}

func (p *responseTimeMetricsProvider) PodMetrics(pod *v1.Pod) (*Metrics, error) {
	value, err := p.metricClient.AllMetrics(pod)

	if err != nil {
		return nil, fmt.Errorf("failed to retrieve all metrics for pod with name %s and namespace %s, error: %v", pod.Name, pod.Namespace, err)
	}

	return &Metrics{
		ResponseTime: resource.NewMilliQuantity(int64(value[string(metrics.ResponseTime)].(float64)), resource.BinarySI),
		RequestCount: resource.NewQuantity(int64(value[string(metrics.RequestCount)].(float64)), resource.BinarySI),
		Throughput:   resource.NewMilliQuantity(int64(value[string(metrics.Throughput)].(float64)), resource.BinarySI),
	}, nil
}

// setMetrics saves the metrics in the provider cache
func (p *responseTimeMetricsProvider) setMetrics(metricInfo CustomMetricResource, value metricValue) {
	p.cacheLock.RLock()
	defer p.cacheLock.RUnlock()
	p.cache[metricInfo] = value
}

func (p *responseTimeMetricsProvider) updatePodMetric(pod, namespace string, metric metrics.Metrics, quantity resource.Quantity) error {

	groupResource := schema.ParseGroupResource("pod")

	info := provider.CustomMetricInfo{
		GroupResource: groupResource,
		Metric:        string(metric),
		Namespaced:    true,
	}

	info, _, err := info.Normalized(p.mapper)

	if err != nil {
		return fmt.Errorf("Error normalizing info: %s", err)
	}

	namespacedName := types.NamespacedName{
		Name:      pod,
		Namespace: namespace,
	}

	metricInfo := CustomMetricResource{
		CustomMetricInfo: info,
		NamespacedName:   namespacedName,
	}

	metricValue := metricValue{
		labels: labels.Set{},
		value:  quantity,
	}

	p.setMetrics(metricInfo, metricValue)

	return nil
}

func (p *responseTimeMetricsProvider) updateServiceMetric(service, namespace string, metric metrics.Metrics, quantity resource.Quantity) error {
	groupResource := schema.ParseGroupResource("service")

	info := provider.CustomMetricInfo{
		GroupResource: groupResource,
		Metric:        string(metric),
		Namespaced:    true,
	}

	info, _, err := info.Normalized(p.mapper)

	if err != nil {
		return fmt.Errorf("Error normalizing info: %s", err)
	}

	namespacedName := types.NamespacedName{
		Name:      service,
		Namespace: namespace,
	}

	metricInfo := CustomMetricResource{
		CustomMetricInfo: info,
		NamespacedName:   namespacedName,
	}

	metricsValue := metricValue{
		labels: labels.Set{},
		value:  quantity,
	}

	p.setMetrics(metricInfo, metricsValue)

	return nil
}
