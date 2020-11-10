package metric

type Informer struct {
	metricClient Client
}

// NewMetricInformer returns a new MetricInformer representing a metric informer.
func NewMetricInformer(client Client) Informer {
	return Informer{
		metricClient: client,
	}
}
