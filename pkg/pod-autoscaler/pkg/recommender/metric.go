package recommender

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	v1 "k8s.io/api/core/v1"
	"k8s.io/metrics/pkg/apis/custom_metrics"
	"net"
	"net/http"
	"net/url"
	"time"
)

var (
	defaultPort     = 5000
	defaultListPath = "/metrics/list"
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

// GetMetrics returns a list of the the metrics of a pod.
func (c Client) GetMetrics(pod *v1.Pod) (map[string]string, error) {

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
		return nil, err
	}

	// Send the request
	response, err := c.HttpClient.Do(request)
	if err != nil {
		return nil, err
	}

	// Parse the response
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	var metricList custom_metrics.MetricValueList
	err = json.Unmarshal(body, &metricList)
	if err != nil {
		return nil, err
	}

	// Cast it to to map for better handling
	metricMap := MetricListToMap(metricList)

	return metricMap, nil
}

// Given a MetricValueList which is composed by a list of MetricValue
// returns a map key-value, where the key is the name of the metric, and the value is its value.
func MetricListToMap(metrics custom_metrics.MetricValueList) map[string]string {
	metricMap := make(map[string]string)
	for _, item := range metrics.Items {
		metricMap[item.Metric.Name] = item.Value.String()
	}
	return metricMap
}
