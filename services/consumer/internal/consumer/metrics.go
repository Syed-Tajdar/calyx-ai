package consumer

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

// Metrics holds Prometheus-format counters/gauges for the consumer (visibility only).
type Metrics struct {
	poisonTotal               atomic.Int64
	dlqTotal                  atomic.Int64
	flushFailuresTotal        atomic.Int64
	retriesTotal              atomic.Int64
	bufferedMessages          atomic.Int64
	lastSuccessfulFlushUnixMs atomic.Int64
}

func (m *Metrics) IncPoison()        { m.poisonTotal.Add(1) }
func (m *Metrics) IncDLQ()           { m.dlqTotal.Add(1) }
func (m *Metrics) IncFlushFailure()  { m.flushFailuresTotal.Add(1) }
func (m *Metrics) IncRetries(n int)  { m.retriesTotal.Add(int64(n)) }
func (m *Metrics) SetBuffered(n int) { m.bufferedMessages.Store(int64(n)) }
func (m *Metrics) MarkFlushOK()      { m.lastSuccessfulFlushUnixMs.Store(time.Now().UnixMilli()) }

func (m *Metrics) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = fmt.Fprintf(w, "# HELP poison_total Messages classified as poison.\n")
		_, _ = fmt.Fprintf(w, "# TYPE poison_total counter\n")
		_, _ = fmt.Fprintf(w, "poison_total %d\n", m.poisonTotal.Load())
		_, _ = fmt.Fprintf(w, "# HELP dlq_total Messages successfully written to the DLQ.\n")
		_, _ = fmt.Fprintf(w, "# TYPE dlq_total counter\n")
		_, _ = fmt.Fprintf(w, "dlq_total %d\n", m.dlqTotal.Load())
		_, _ = fmt.Fprintf(w, "# HELP flush_failures_total Failed batch or per-message flush attempts.\n")
		_, _ = fmt.Fprintf(w, "# TYPE flush_failures_total counter\n")
		_, _ = fmt.Fprintf(w, "flush_failures_total %d\n", m.flushFailuresTotal.Load())
		_, _ = fmt.Fprintf(w, "# HELP retries_total Transient insert retries.\n")
		_, _ = fmt.Fprintf(w, "# TYPE retries_total counter\n")
		_, _ = fmt.Fprintf(w, "retries_total %d\n", m.retriesTotal.Load())
		_, _ = fmt.Fprintf(w, "# HELP buffered_messages Current in-memory batch size.\n")
		_, _ = fmt.Fprintf(w, "# TYPE buffered_messages gauge\n")
		_, _ = fmt.Fprintf(w, "buffered_messages %d\n", m.bufferedMessages.Load())
		_, _ = fmt.Fprintf(w, "# HELP last_successful_flush_timestamp Unix ms of last successful flush.\n")
		_, _ = fmt.Fprintf(w, "# TYPE last_successful_flush_timestamp gauge\n")
		_, _ = fmt.Fprintf(w, "last_successful_flush_timestamp %d\n", m.lastSuccessfulFlushUnixMs.Load())
	}
}
