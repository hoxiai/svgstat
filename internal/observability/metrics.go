package observability

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Metrics struct {
	mu               sync.Mutex
	httpRequests     map[string]uint64
	httpDuration     map[string]float64
	cache            map[string]uint64
	workerFlushes    map[string]uint64
	workerDuration   float64
	apaySyncCount    uint64
	apaySyncDuration float64
	cardinalityDrops map[string]uint64
}

func New() *Metrics {
	return &Metrics{
		httpRequests:     map[string]uint64{},
		httpDuration:     map[string]float64{},
		cache:            map[string]uint64{},
		workerFlushes:    map[string]uint64{},
		cardinalityDrops: map[string]uint64{},
	}
}

func (m *Metrics) ObserveHTTP(route string, status int, duration time.Duration) {
	if route == "" {
		route = "unmatched"
	}
	key := route + "\x00" + strconv.Itoa(status)
	m.mu.Lock()
	m.httpRequests[key]++
	m.httpDuration[route] += duration.Seconds()
	m.mu.Unlock()
}

func (m *Metrics) ObserveRuntimeCache(result string) {
	m.mu.Lock()
	m.cache[result]++
	m.mu.Unlock()
}

func (m *Metrics) ObserveWorkerFlush(duration time.Duration, err error) {
	result := "success"
	if err != nil {
		result = "error"
	}
	m.mu.Lock()
	m.workerFlushes[result]++
	m.workerDuration += duration.Seconds()
	m.mu.Unlock()
}

func (m *Metrics) ObserveAPaySync(duration time.Duration) {
	m.mu.Lock()
	m.apaySyncCount++
	m.apaySyncDuration += duration.Seconds()
	m.mu.Unlock()
}

func (m *Metrics) ObserveCardinalityDrop(kind string) {
	m.mu.Lock()
	m.cardinalityDrops[kind]++
	m.mu.Unlock()
}

func (m *Metrics) WritePrometheus(writer io.Writer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	writeHelp(writer, "svgstat_http_requests_total", "HTTP requests by route and status.")
	for _, key := range sortedKeys(m.httpRequests) {
		parts := strings.SplitN(key, "\x00", 2)
		fmt.Fprintf(writer, "svgstat_http_requests_total{route=%q,status=%q} %d\n", parts[0], parts[1], m.httpRequests[key])
	}
	writeHelp(writer, "svgstat_http_request_duration_seconds_sum", "Total HTTP request duration by route.")
	for _, route := range sortedKeys(m.httpDuration) {
		fmt.Fprintf(writer, "svgstat_http_request_duration_seconds_sum{route=%q} %g\n", route, m.httpDuration[route])
	}
	writeHelp(writer, "svgstat_runtime_cache_total", "Runtime project cache lookups by result.")
	for _, result := range sortedKeys(m.cache) {
		fmt.Fprintf(writer, "svgstat_runtime_cache_total{result=%q} %d\n", result, m.cache[result])
	}
	writeHelp(writer, "svgstat_worker_flush_total", "Worker flushes by result.")
	for _, result := range sortedKeys(m.workerFlushes) {
		fmt.Fprintf(writer, "svgstat_worker_flush_total{result=%q} %d\n", result, m.workerFlushes[result])
	}
	fmt.Fprintf(writer, "svgstat_worker_flush_duration_seconds_sum %g\n", m.workerDuration)
	fmt.Fprintf(writer, "svgstat_apay_sync_duration_seconds_count %d\n", m.apaySyncCount)
	fmt.Fprintf(writer, "svgstat_apay_sync_duration_seconds_sum %g\n", m.apaySyncDuration)
	writeHelp(writer, "svgstat_analytics_cardinality_dropped_total", "Analytics dimensions dropped at configured cardinality limits.")
	for _, kind := range sortedKeys(m.cardinalityDrops) {
		fmt.Fprintf(writer, "svgstat_analytics_cardinality_dropped_total{kind=%q} %d\n", kind, m.cardinalityDrops[kind])
	}
}

func writeHelp(writer io.Writer, name, help string) {
	fmt.Fprintf(writer, "# HELP %s %s\n# TYPE %s counter\n", name, help, name)
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
