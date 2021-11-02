package db

import (
"context"
"github.com/jackc/pgx/v4"
"github.com/jackc/pgx/v4/pgxpool"
"k8s.io/klog/v2"
)

var (
	columns = []string{
		"timestamp",
		"function",
		"node",
		"namespace",
		"community",
		"gpu",
		"latency",
	}
)

const (
	// InsertMetricQuery is the prepare statement for inserting metrics.
	InsertMetricQuery = "INSERT INTO proxy_metric (timestamp, function, node, namespace, community, gpu, latency) VALUES ($1, $2, $3, $4, $5, $6, $7);"
	batchSize         = 1000
	table             = "proxy_metric"
)

// MetricsPersistor receives metrics from the load balancer and persists them to a backend.
// The initial implementation is a simple client that connects to a TimescaleDB backend.
type MetricsPersistor struct {
	pool        *pgxpool.Pool
	metrichChan <-chan RawResponseTime
	opts        Options
	ctx         context.Context
}

// NewMetricsPersistor creates a new MetricsPersistor.
func NewMetricsPersistor(opts Options, rawMetricChan <-chan RawResponseTime) *MetricsPersistor {
	return &MetricsPersistor{
		opts:        opts,
		metrichChan: rawMetricChan,
	}
}

// SetupDBConnection creates a new connection to the database using the provided options.
func (p *MetricsPersistor) SetupDBConnection() error {
	var config *pgxpool.Config
	var err error

	config, err = pgxpool.ParseConfig(p.opts.ConnString())

	if err != nil {
		return err
	}

	p.pool, err = pgxpool.ConnectConfig(context.Background(), config)

	if err != nil {
		return err
	}

	return nil
}

// Stop closes the connection to the database.
func (p *MetricsPersistor) Stop() {
	p.ctx.Done()
	p.pool.Close()
}

// PollMetrics receives metrics from the load balancer and persists them to a backend until the chan is closed.
// It spawns the first polling goroutine which always listen for new data.
func (p *MetricsPersistor) PollMetrics() {
	var cancel context.CancelFunc

	// use context to terminate all routines
	p.ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	klog.Infof("start metrics collection")

	p.batchData(false)

	klog.Infof("stop metrics collection")

}

// batchData receives metrics from the load balancer and persists them to a backend.
// It spawns new goroutines if the metrics arrival rate cannot be handled buy a single routine.
func (p *MetricsPersistor) batchData(terminate bool) {
	var batch []RawResponseTime
	var err error
	batch = make([]RawResponseTime, 0, batchSize)

	for {
		select {
		case <-p.ctx.Done():
			return
		case m, ok := <-p.metrichChan:
			if !ok {
				p.Stop()
				return
			}

			batch = append(batch, m)

			if !terminate && len(p.metrichChan) > batchSize/4 {
				klog.Infof("create new polling routine")
				go p.batchData(true)
			}

			if len(batch) == batchSize || len(p.metrichChan) == 0 {
				err = p.save(batch)
				if err != nil {
					klog.Errorf("failed to persist metric %v error: %s\n", m, err)
				}
				if terminate {
					break
				}
				batch = make([]RawResponseTime, 0, batchSize)
			}


		}

	}
}

// Save insert a new metric into the database.
func (p *MetricsPersistor) save(batch []RawResponseTime) error {

	_, err := p.pool.CopyFrom(context.TODO(), pgx.Identifier{table}, columns, pgx.CopyFromSlice(len(batch), func(i int) ([]interface{}, error) {
		return batch[i].AsCopy(), nil
	}))

	return err
}
