package metric

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/metrics/pkg/apis/custom_metrics"
	"net"
	"net/http"
	"net/url"
	"time"
)

var (
	defaultPort      = 5000
	defaultListPath  = "/metrics/list"
	defaultListWatch = "/metrics/watch"
)

type Client struct {
	HttpClient http.Client
}

// NewMetricClient returns a new MetricClient representing a metric client.
func NewMetricClient() Client {
	httpClient := http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: 90 * time.Second,
			}).DialContext,
			MaxIdleConns:          0,
			IdleConnTimeout:       90 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
		Timeout: 90 * time.Second,
	}
	client := Client{
		HttpClient: httpClient,
	}
	return client
}

// ListMetrics returns a list of the the metrics of a pod.
func (c Client) ListMetrics(pod v1.Pod) custom_metrics.MetricValueList {

	// Retrieve the location of the pod's metrics server
	address := pod.Status.PodIP
	port := defaultPort
	path := defaultListPath

	// Create the request
	metricServerURL := url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s:%d", address, port),
		Path:   path,
	}
	request, err := http.NewRequest(http.MethodGet, metricServerURL.String(), nil)
	if err != nil {
		klog.Info(err)
	}

	// Send the request
	response, err := c.HttpClient.Do(request)
	if err != nil {
		klog.Info(err)
	}

	// Parse the response
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		klog.Info(err)
	}
	var metricList custom_metrics.MetricValueList
	err = json.Unmarshal(body, &metricList)
	if err != nil {
		klog.Info(err)
	}

	return metricList
}

// WatchMetrics returns a watch of the metrics of a pod.
func (c Client) WatchMetrics(pod v1.Pod) custom_metrics.MetricValueList {

	// Retrieve the location of the pod's metrics server
	address := pod.Status.PodIP
	port := defaultPort
	path := defaultListWatch

	// Create the request
	metricServerURL := url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s:%d", address, port),
		Path:   path,
	}
	request, err := http.NewRequest(http.MethodGet, metricServerURL.String(), nil)
	if err != nil {
		klog.Info(err)
	}

	// Send the request
	response, err := c.HttpClient.Do(request)
	if err != nil {
		klog.Info(err)
	}

	// Parse the response
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		klog.Info(err)
	}
	var metricList custom_metrics.MetricValueList
	err = json.Unmarshal(body, &metricList)
	if err != nil {
		klog.Info(err)
	}

	return metricList
}
