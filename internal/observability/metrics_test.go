package observability

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestMetricsWritePrometheus(t *testing.T) {
	metrics := New()
	metrics.ObserveHTTP("/health", 200, 10*time.Millisecond)
	metrics.ObserveRuntimeCache("memory_hit")
	metrics.ObserveWorkerFlush(time.Second, nil)
	metrics.ObserveWorkerFlush(time.Second, errors.New("failed"))
	metrics.ObserveAPaySync(2 * time.Second)
	metrics.ObserveCardinalityDrop("visitor")
	var output bytes.Buffer
	metrics.WritePrometheus(&output)
	for _, expected := range []string{
		`svgstat_http_requests_total{route="/health",status="200"} 1`,
		`svgstat_runtime_cache_total{result="memory_hit"} 1`,
		`svgstat_worker_flush_total{result="error"} 1`,
		`svgstat_apay_sync_duration_seconds_count 1`,
		`svgstat_analytics_cardinality_dropped_total{kind="visitor"} 1`,
	} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("metrics output missing %q:\n%s", expected, output.String())
		}
	}
}
