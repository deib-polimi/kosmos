package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

func serverMock() *httptest.Server {
	handler := http.NewServeMux()
	handler.HandleFunc("/", usersMock)

	srv := httptest.NewServer(handler)

	return srv
}

func usersMock(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte(`{"response_time":5.0}`))
}

func TestGetMetrics(t *testing.T) {
	client := NewClient()
	server := serverMock()
	client.Host = server.URL[7:]
	metrics, err := client.GetMetrics(&v1.Pod{})
	if err != nil {
		klog.Error(err)
	}
	require.Equal(t, 5.0, metrics["response_time"])
}
