package provider

import (
	"errors"
	"fmt"

	"github.com/kubernetes-sigs/custom-metrics-apiserver/pkg/provider"
	"github.com/lterrac/system-autoscaler/pkg/metrics-exposer/pkg/metrics"
	corev1 "k8s.io/api/core/v1"
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

func (m Metrics) metric(metric string) (resource.Quantity, error) {

	switch metric {
	case string(metrics.ResponseTime):
		return *m.ResponseTime, nil
	case string(metrics.RequestCount):
		return *m.RequestCount, nil
	case string(metrics.Throughput):
		return *m.Throughput, nil
	default:
		return resource.Quantity{}, errors.New("error while parsing metric. Non existing metric " + metric)
	}

}

// updateMetrics updates the map of metrics
// for now, it updates the metrics for pods and services
func (p *responseTimeMetricsProvider) updateMetrics() {

	var responseTimeMetric *resource.Quantity
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
			klog.Errorf("failed to retrieve to retrieve the pod with name %s and namespace %s", podName, podNamespace)
			continue
		}

		responseTimeMetric, err = p.getResponseTime(pod)
		if err != nil {
			klog.Error("failed to retrieve the metrics for pod with name %s and namespace %s", podName, podNamespace)
			continue
		}

		requestCountMetric, err := p.getRequestCount(pod)
		if err != nil {
			klog.Error("failed to retrieve the metrics for pod with name %s and namespace %s", podName, podNamespace)
			continue
		}

		throughputMetric, err := p.getThroughput(pod)
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

		metrics := Metrics{
			ResponseTime: responseTimeMetric,
			RequestCount: requestCountMetric,
			Throughput:   throughputMetric,
		}

		p.setMetrics(metricInfo, metrics)

		if _, ok := serviceMetricsMap[serviceNamespace]; !ok {
			serviceMetricsMap[serviceNamespace] = make(map[string][]*Metrics)
		}

		// group metrics by service
		serviceMetrics, ok := serviceMetricsMap[serviceNamespace][serviceName]

		if !ok {
			serviceMetrics = make([]*Metrics, 0)
		}

		serviceMetricsMap[serviceNamespace][serviceName] = append(serviceMetrics, &metrics)

	}

	for namespace, nestedMap := range serviceMetricsMap {
		for name, metrics := range nestedMap {

			// Compute average
			responseTimeSum := 0
			requestCountSum := 0
			throughputSum := 0
			weights := 0

			for _, metric := range metrics {
				requests, ok := metric.RequestCount.AsInt64()
				if !ok {
					continue
				}
				responseTimeSum += int(metric.ResponseTime.MilliValue()) * int(requests)
				requestCountSum += int(metric.RequestCount.MilliValue()) * int(requests)
				throughputSum += int(metric.Throughput.MilliValue()) * int(requests)
				weights += int(requests)
			}

			averageResponseTime := resource.NewMilliQuantity(int64(responseTimeSum/(len(metrics)*weights)), resource.BinarySI)
			averageRequestCount := resource.NewMilliQuantity(int64(requestCountSum/(len(metrics)*weights)), resource.BinarySI)
			averageThroughput := resource.NewMilliQuantity(int64(throughputSum/(len(metrics)*weights)), resource.BinarySI)

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

			metrics := Metrics{
				ResponseTime: averageResponseTime,
				RequestCount: averageRequestCount,
				Throughput:   averageThroughput,
			}

			p.setMetrics(metricInfo, metrics)

		}
	}

}

// getResponseTime retrieve the metrics of a pod
func (p *responseTimeMetricsProvider) getResponseTime(pod *corev1.Pod) (*resource.Quantity, error) {

	// Retrieve the metrics through the HTTP client
	value, err := p.metricClient.ResponseTime(pod)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve %s for pod with name %s and namespace %s, error: %v", string(metrics.ResponseTime), pod.Name, pod.Namespace, err)
	}
	return resource.NewMilliQuantity(int64(value[string(metrics.ResponseTime)].(float64)), resource.BinarySI), nil
}

// getResponseTimeForPod retrieve the metrics of a pod
func (p *responseTimeMetricsProvider) getRequestCount(pod *corev1.Pod) (*resource.Quantity, error) {

	// Retrieve the metrics through the HTTP client
	value, err := p.metricClient.RequestCount(pod)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve %s for pod with name %s and namespace %s, error: %v", string(metrics.ResponseTime), pod.Name, pod.Namespace, err)
	}
	return resource.NewMilliQuantity(int64(value[string(metrics.RequestCount)].(float64)), resource.BinarySI), nil

}

// getResponseTimeForPod retrieve the metrics of a pod
func (p *responseTimeMetricsProvider) getThroughput(pod *corev1.Pod) (*resource.Quantity, error) {

	// Retrieve the metrics through the HTTP client
	value, err := p.metricClient.ResponseTime(pod)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve %s for pod with name %s and namespace %s, error: %v", string(metrics.ResponseTime), pod.Name, pod.Namespace, err)
	}
	return resource.NewMilliQuantity(int64(value[string(metrics.Throughput)].(float64)), resource.BinarySI), nil

}

// setMetrics saves the metrics in the provider cache
func (p *responseTimeMetricsProvider) setMetrics(metricInfo CustomMetricResource, value Metrics) {
	p.cacheLock.RLock()
	defer p.cacheLock.RUnlock()
	p.cache[metricInfo] = value
}
