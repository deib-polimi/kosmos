package db

import (
	"time"
)

// RawResponseTime is the response time for a single http request
type RawResponseTime struct {
	Timestamp time.Time
	Function  string
	Node      string
	Namespace string
	Community string
	Gpu       bool
	Latency   int
}

func (r RawResponseTime) AsCopy() []interface{} {
	return []interface{}{
		r.Timestamp,
		r.Node,
		r.Function,
		r.Namespace,
		r.Community,
		r.Gpu,
		r.Latency,
	}
}
