package main

import (
	"fmt"
	"k8s.io/klog/v2"
	"log"
	"math"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/asecurityteam/rolling"
	"github.com/lterrac/system-autoscaler/pkg/http-metrics/pkg/db"
	"github.com/lterrac/system-autoscaler/pkg/metrics-exposer/pkg/metrics"
)

var target = &url.URL{}
var window = &rolling.TimePolicy{}

// Environment
var address string
var port string
var windowSize time.Duration
var windowGranularity time.Duration

// MetricsDB
var database *db.MetricsPersistor
var metricChan chan db.RawResponseTime
var proxy *httputil.ReverseProxy

// Useful information
var node string
var function string
var community string
var namespace string
var gpu bool

func main() {

	var err error

	mux := http.NewServeMux()

	mux.Handle("/metric/response_time", http.HandlerFunc(ResponseTime))
	mux.Handle("/metric/request_count", http.HandlerFunc(RequestCount))
	mux.Handle("/metric/throughput", http.HandlerFunc(Throughput))
	mux.Handle("/metrics/", http.HandlerFunc(AllMetrics))
	mux.Handle("/", http.HandlerFunc(ForwardRequest))

	address = getenv("ADDRESS", "localhost")
	port = getenv("APP_PORT", "8080")
	windowSizeString := getenv("WINDOW_SIZE", "30s")
	windowGranularityString := getenv("WINDOW_GRANULARITY", "1m")

	node = getenv("NODE", "")
	function = getenv("FUNCTION", "")
	community = getenv("COMMUNITY", "")
	namespace = getenv("NAMESPACE", "")
	gpu, err = strconv.ParseBool(getenv("GPU", "false"))

	if err != nil {
		klog.Fatal("failed to parse GPU environment variable. It should be a boolean")
	}

	metricChan = make(chan db.RawResponseTime, 10000)

	database = db.NewMetricsPersistor(db.NewDBOptions(), metricChan)
	err = database.SetupDBConnection()
	go database.PollMetrics()

	if err != nil {
		klog.Fatal(err)
	}

	srv := &http.Server{
		Addr:    ":8000",
		Handler: mux,
	}
	target, _ = url.Parse("http://" + address + ":" + port)
	log.Println("Forwarding all requests to:", target)

	windowSize, err = time.ParseDuration(windowSizeString)

	proxy = httputil.NewSingleHostReverseProxy(target)

	if err != nil {
		log.Fatalf("Failed to parse windows size. Error: %v", err)
	}

	windowGranularity, err = time.ParseDuration(windowGranularityString)

	if err != nil {
		log.Fatalf("Failed to parse windows granularity. Error: %v", err)
	}

	window = rolling.NewTimePolicy(rolling.NewWindow(int(windowSize.Nanoseconds()/windowGranularity.Nanoseconds())), time.Millisecond)
	log.Println("Time window initialized with size:", windowSizeString, " and granularity:", windowGranularityString)

	// output error and quit if ListenAndServe fails
	log.Fatal(srv.ListenAndServe())

}

// ForwardRequest send all the request the the pod except for the ones having metrics/ in the path
func ForwardRequest(res http.ResponseWriter, req *http.Request) {

	klog.Infof("Received request %v", req)

	requestTime := time.Now()

	if proxy != nil {
		proxy.ServeHTTP(res, req)
	}

	klog.Infof("Response forwarded back")

	if req.URL.Path != "health" {
		responseTime := time.Now()
		delta := responseTime.Sub(requestTime)
		metricChan <- db.RawResponseTime{
			Timestamp: time.Now(),
			Function:  function,
			Node:      node,
			Namespace: namespace,
			Community: community,
			Gpu:       gpu,
			Latency:   int(delta.Milliseconds()),
		}
		window.Append(float64(delta.Milliseconds()))
	}
}

// ResponseTime return the pod average response time
func ResponseTime(res http.ResponseWriter, req *http.Request) {
	responseTime := window.Reduce(rolling.Avg)
	if math.IsNaN(responseTime) {
		responseTime = 0
	}
	_, _ = fmt.Fprintf(res, `{"%s": %f}`, metrics.ResponseTime.String(), responseTime)
}

// RequestCount return the current number of request sent to the pod
func RequestCount(res http.ResponseWriter, req *http.Request) {
	requestCount := window.Reduce(rolling.Count)
	if math.IsNaN(requestCount) {
		requestCount = 0
	}
	_, _ = fmt.Fprintf(res, `{"%s": %f}`, metrics.RequestCount.String(), requestCount)
}

// Throughput returns the pod throughput in request per second
func Throughput(res http.ResponseWriter, req *http.Request) {
	throughput := window.Reduce(rolling.Count) / windowSize.Seconds()
	_, _ = fmt.Fprintf(res, `{"%s": %f}`, metrics.Throughput.String(), throughput)
}

// AllMetrics returns all the metrics available for the pod
func AllMetrics(res http.ResponseWriter, req *http.Request) {

	responseTime := window.Reduce(rolling.Avg)
	if math.IsNaN(responseTime) {
		responseTime = 0
	}

	requestCount := window.Reduce(rolling.Count)
	if math.IsNaN(requestCount) {
		requestCount = 0
	}

	throughput := window.Reduce(rolling.Count) / windowSize.Seconds()
	// TODO: maybe we should wrap this into an helper function of metrics struct
	klog.Info(fmt.Sprintf(`{"%s": %f,"%s": %f,"%s": %f}`, metrics.ResponseTime.String(), responseTime, metrics.RequestCount.String(), requestCount, metrics.Throughput.String(), throughput))
	_, _ = fmt.Fprintf(res, `{"%s": %f,"%s": %f,"%s": %f}`, metrics.ResponseTime.String(), responseTime, metrics.RequestCount.String(), requestCount, metrics.Throughput.String(), throughput)
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		klog.Warningf("failed parsing environment variable %s, setting it to default value %s", key, fallback)
		return fallback
	}
	klog.Infof("parsed environment variable %s with value %s", key, value)
	return value
}
