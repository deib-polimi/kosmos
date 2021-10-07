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
	"time"

	"github.com/asecurityteam/rolling"
	"github.com/lterrac/system-autoscaler/pkg/metrics-exposer/pkg/metrics"
)

var target = &url.URL{}
var window = &rolling.TimePolicy{}

// Environment
var address string
var port string
var windowSize time.Duration
var windowGranularity time.Duration

func main() {
	mux := http.NewServeMux()

	mux.Handle("/metric/response_time", http.HandlerFunc(ResponseTime))
	mux.Handle("/metric/request_count", http.HandlerFunc(RequestCount))
	mux.Handle("/metric/throughput", http.HandlerFunc(Throughput))
	mux.Handle("/metrics/", http.HandlerFunc(AllMetrics))
	mux.Handle("/", http.HandlerFunc(ForwardRequest))

	address = os.Getenv("ADDRESS")
	port = os.Getenv("PORT")
	windowSizeString := os.Getenv("WINDOW_SIZE")
	windowGranularityString := os.Getenv("WINDOW_GRANULARITY")

	var err error
	log.Println("Reading environment variables")

	srv := &http.Server{
		Addr:    ":8000",
		Handler: mux,
	}
	target, _ = url.Parse("http://" + address + ":" + port)
	log.Println("Forwarding all requests to:", target)

	windowSize, err = time.ParseDuration(windowSizeString)

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
	requestTime := time.Now()
	httputil.NewSingleHostReverseProxy(target).ServeHTTP(res, req)
	responseTime := time.Now()
	delta := responseTime.Sub(requestTime)
	window.Append(float64(delta.Milliseconds()))
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

	klog.Infof(`{"%s": %f,"%s": %f,"%s": %f}`, metrics.ResponseTime.String(), responseTime, metrics.RequestCount.String(), requestCount, metrics.Throughput.String(), throughput)
	_, _ = fmt.Fprintf(res, `{"%s": %f,"%s": %f,"%s": %f}`, metrics.ResponseTime.String(), responseTime, metrics.RequestCount.String(), requestCount, metrics.Throughput.String(), throughput)
}
