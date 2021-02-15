package replicaupdater

import (
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

type Client struct {
	Host       string
	httpClient http.Client
}

// NewMetricClient returns a new MetricClient representing a metric client.
func NewMetricClient() *Client {
	httpClient := http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: 90 * time.Second,
			}).DialContext,
			// TODO: Some of those value should be tuned
			MaxIdleConns:          50,
			IdleConnTimeout:       90 * time.Second,
			ExpectContinueTimeout: 5 * time.Second,
		},
		Timeout: 5 * time.Second,
	}
	client := &Client{
		httpClient: httpClient,
		Host:       "{pod_address}:{pod_port}",
	}
	return client
}

// getMetrics returns a list of the the metrics of a pod.
func (c Client) getMetrics(pod *v1.Pod) (map[string]interface{}, error) {

	// Retrieve the location of the pod's metrics server
	podAddress := pod.Status.PodIP

	// Compose host and path
	host := c.Host
	host = strings.Replace(host, "{pod_address}", podAddress, -1)
	host = strings.Replace(host, "{pod_port}", "8000", -1)
	path := "metrics/response_time"

	// Create the request
	metricServerURL := url.URL{
		// TODO: http is not a good protocol for polling data
		// grpc can be a good alternative, but the pods should implement a grpc server
		// offerings the metrics
		Scheme: "http",
		Host:   host,
		Path:   path,
	}
	request, err := http.NewRequest(http.MethodGet, metricServerURL.String(), nil)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	// Send the request
	response, err := c.httpClient.Do(request)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	// Parse the response
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	var metricMap map[string]interface{}
	err = json.Unmarshal(body, &metricMap)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	return metricMap, nil
}