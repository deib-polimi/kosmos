package main

import (
	"fmt"
	"log"
	"math"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"

	"github.com/asecurityteam/rolling"
)

//
var target = &url.URL{}
var window = &rolling.TimePolicy{}

// Environment
var address string
var port string
var windowSize string
var windowGranularity string

func main() {
	mux := http.NewServeMux()

	mux.Handle("/metrics/response_time", http.HandlerFunc(ResponseTime))
	mux.Handle("/metrics/throughput", http.HandlerFunc(Throughput))
	mux.Handle("/", http.HandlerFunc(ForwardRequest))

	address = os.Getenv("ADDRESS")
	port = os.Getenv("PORT")
	windowSize = os.Getenv("WINDOW_SIZE")
	windowGranularity = os.Getenv("WINDOW_GRANULARITY")
	log.Println("Reading environment variables")

	srv := &http.Server{
		Addr:    ":8000",
		Handler: mux,
	}
	target, _ = url.Parse("http://" + address + ":" + port)
	log.Println("Forwarding all requests to:", target)

	ws, _ := time.ParseDuration(windowSize)
	wg, _ := time.ParseDuration(windowGranularity)
	window = rolling.NewTimePolicy(rolling.NewWindow(int(ws.Nanoseconds()/wg.Nanoseconds())), time.Millisecond)
	log.Println("Time window initialized with size:", ws, " and granularity:", wg)

	// output error and quit if ListenAndServe fails
	log.Fatal(srv.ListenAndServe())

}

func ForwardRequest(res http.ResponseWriter, req *http.Request) {
	// Update the headers to allow for SSL redirection
	//req.URL.Host = target.Host
	//req.URL.Scheme = target.Scheme
	//req.Header.Set("X-Forwarded-Host", req.Header.Get("Host"))
	//req.Host = target.Host
	requestTime := time.Now()
	httputil.NewSingleHostReverseProxy(target).ServeHTTP(res, req)
	responseTime := time.Now()
	delta := responseTime.Sub(requestTime)
	window.Append(float64(delta.Milliseconds()))
}

func ResponseTime(res http.ResponseWriter, req *http.Request) {
	responseTime := window.Reduce(rolling.Avg)
	if math.IsNaN(responseTime) {
		responseTime = 0
	}
	fmt.Fprintf(res, `{"response_time": %f}`, responseTime)
}

func Throughput(res http.ResponseWriter, req *http.Request) {
	throughput := window.Reduce(rolling.Count)
	fmt.Fprintf(res, `{"throughput": %f}`, throughput)
}
