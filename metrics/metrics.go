package metrics

import (
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	once     sync.Once
	instance *Metrics
)

type Metrics struct {
	registry *prometheus.Registry

	connections   prometheus.Gauge
	activeSessions prometheus.Gauge
	messageTotal  *prometheus.CounterVec
	messageSize   *prometheus.HistogramVec
	handlerDuration *prometheus.HistogramVec
	rpcLatency    *prometheus.HistogramVec
	rpcRequests   *prometheus.CounterVec
	rpcErrors     *prometheus.CounterVec
	poolCapacity  *prometheus.GaugeVec
	poolInUse     *prometheus.GaugeVec
	queueSize     *prometheus.GaugeVec
	serverUptime  prometheus.Counter
	memoryUsage   prometheus.Gauge
	goroutines    prometheus.Gauge
}

func New() *Metrics {
	m := &Metrics{
		registry: prometheus.NewRegistry(),
	}

	m.connections = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "gomelo",
		Name:      "active_connections",
		Help:      "Current number of active connections",
	})

	m.activeSessions = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "gomelo",
		Name:      "active_sessions",
		Help:      "Current number of active sessions",
	})

	m.messageTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "gomelo",
		Name:      "messages_total",
		Help:      "Total number of messages processed",
	}, []string{"type", "route"})

	m.messageSize = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "gomelo",
		Name:      "message_size_bytes",
		Help:      "Size of messages in bytes",
		Buckets:   []float64{64, 256, 512, 1024, 4096, 16384, 65536},
	}, []string{"type"})

	m.handlerDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "gomelo",
		Name:      "handler_duration_seconds",
		Help:      "Handler execution duration in seconds",
		Buckets:   []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
	}, []string{"route", "status"})

	m.rpcLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "gomelo",
		Name:      "rpc_latency_seconds",
		Help:      "RPC call latency in seconds",
		Buckets:   []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
	}, []string{"service", "method"})

	m.rpcRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "gomelo",
		Name:      "rpc_requests_total",
		Help:      "Total number of RPC requests",
	}, []string{"service", "method", "status"})

	m.rpcErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "gomelo",
		Name:      "rpc_errors_total",
		Help:      "Total number of RPC errors",
	}, []string{"service", "method", "type"})

	m.poolCapacity = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "gomelo",
		Name:      "pool_capacity",
		Help:      "Pool capacity by name",
	}, []string{"name"})

	m.poolInUse = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "gomelo",
		Name:      "pool_in_use",
		Help:      "Pool connections in use by name",
	}, []string{"name"})

	m.queueSize = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "gomelo",
		Name:      "queue_size",
		Help:      "Queue size by name",
	}, []string{"name"})

	m.serverUptime = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "gomelo",
		Name:      "server_uptime_seconds",
		Help:      "Server uptime in seconds",
	})

	m.memoryUsage = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "gomelo",
		Name:      "memory_usage_bytes",
		Help:      "Memory usage in bytes",
	})

	m.goroutines = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "gomelo",
		Name:      "goroutines",
		Help:      "Number of goroutines",
	})

	m.registry.MustRegister(
		m.connections,
		m.activeSessions,
		m.messageTotal,
		m.messageSize,
		m.handlerDuration,
		m.rpcLatency,
		m.rpcRequests,
		m.rpcErrors,
		m.poolCapacity,
		m.poolInUse,
		m.queueSize,
		m.serverUptime,
		m.memoryUsage,
		m.goroutines,
	)

	return m
}

func Global() *Metrics {
	once.Do(func() {
		instance = New()
	})
	return instance
}

func (m *Metrics) IncConnections() {
	m.connections.Inc()
}

func (m *Metrics) DecConnections() {
	m.connections.Dec()
}

func (m *Metrics) SetConnections(n float64) {
	m.connections.Set(n)
}

func (m *Metrics) IncSessions() {
	m.activeSessions.Inc()
}

func (m *Metrics) DecSessions() {
	m.activeSessions.Dec()
}

func (m *Metrics) SetSessions(n float64) {
	m.activeSessions.Set(n)
}

func (m *Metrics) IncMessages(msgType, route string) {
	m.messageTotal.WithLabelValues(msgType, route).Inc()
}

func (m *Metrics) ObserveMessageSize(msgType string, size float64) {
	m.messageSize.WithLabelValues(msgType).Observe(size)
}

func (m *Metrics) ObserveHandlerDuration(route string, status string, seconds float64) {
	m.handlerDuration.WithLabelValues(route, status).Observe(seconds)
}

func (m *Metrics) IncRPCRequest(service, method, status string) {
	m.rpcRequests.WithLabelValues(service, method, status).Inc()
}

func (m *Metrics) ObserveRPCLatency(service, method string, seconds float64) {
	m.rpcLatency.WithLabelValues(service, method).Observe(seconds)
}

func (m *Metrics) IncRPCError(service, method, errType string) {
	m.rpcErrors.WithLabelValues(service, method, errType).Inc()
}

func (m *Metrics) SetPoolCapacity(name string, n float64) {
	m.poolCapacity.WithLabelValues(name).Set(n)
}

func (m *Metrics) SetPoolInUse(name string, n float64) {
	m.poolInUse.WithLabelValues(name).Set(n)
}

func (m *Metrics) SetQueueSize(name string, n float64) {
	m.queueSize.WithLabelValues(name).Set(n)
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

type Counter struct {
	value atomic.Int64
}

func NewCounter() *Counter {
	return &Counter{}
}

func (c *Counter) Inc() {
	c.value.Add(1)
}

func (c *Counter) Add(n int64) {
	c.value.Add(n)
}

func (c *Counter) Value() int64 {
	return c.value.Load()
}

type Gauge struct {
	value int64
}

func NewGauge() *Gauge {
	return &Gauge{}
}

func (g *Gauge) Set(n float64) {
	atomic.StoreInt64(&g.value, int64(n))
}

func (g *Gauge) Inc() {
	atomic.AddInt64(&g.value, 1)
}

func (g *Gauge) Dec() {
	atomic.AddInt64(&g.value, -1)
}

func (g *Gauge) Value() float64 {
	return float64(atomic.LoadInt64(&g.value))
}

type Histogram struct {
	counts  [6]atomic.Int64
	buckets []float64
	sum     int64
	count   atomic.Int64
	mu      sync.Mutex
}

func NewHistogram(buckets []float64) *Histogram {
	if buckets == nil {
		buckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5}
	}
	return &Histogram{buckets: buckets}
}

func (h *Histogram) Observe(v float64) {
	atomic.AddInt64(&h.sum, int64(v*1000000))
	h.count.Add(1)

	h.mu.Lock()
	defer h.mu.Unlock()

	for i, b := range h.buckets {
		if v <= b {
			h.counts[i].Add(1)
			return
		}
	}
	h.counts[len(h.buckets)-1].Add(1)
}

func (h *Histogram) Percentile(p float64) float64 {
	total := h.count.Load()
	if total == 0 {
		return 0
	}

	threshold := float64(total) * p
	var cum int64

	h.mu.Lock()
	defer h.mu.Unlock()

	for i := range h.buckets {
		cum += h.counts[i].Load()
		if float64(cum) >= threshold {
			return h.buckets[i]
		}
	}
	return h.buckets[len(h.buckets)-1]
}

func (h *Histogram) Sum() float64 {
	return float64(atomic.LoadInt64(&h.sum)) / 1000000
}

type Timer struct {
	start time.Time
}

func NewTimer() *Timer {
	return &Timer{start: time.Now()}
}

func (t *Timer) Reset() {
	t.start = time.Now()
}

func (t *Timer) Elapsed() time.Duration {
	return time.Since(t.start)
}

func (t *Timer) ElapsedSeconds() float64 {
	return t.Elapsed().Seconds()
}

type MetricsMiddleware struct {
	metrics *Metrics
}

func NewMiddleware(m *Metrics) *MetricsMiddleware {
	return &MetricsMiddleware{metrics: m}
}

func (m *MetricsMiddleware) RecordHandler(route string, status string, start time.Time) {
	duration := time.Since(start).Seconds()
	m.metrics.ObserveHandlerDuration(route, status, duration)
}

func (m *MetricsMiddleware) RecordMessage(msgType, route string, size int) {
	m.metrics.IncMessages(msgType, route)
	m.metrics.ObserveMessageSize(msgType, float64(size))
}

type ConnectionTracker struct {
	connections atomic.Int64
	maxConnections int64
}

func NewConnectionTracker() *ConnectionTracker {
	return &ConnectionTracker{}
}

func (t *ConnectionTracker) Add() int64 {
	return t.connections.Add(1)
}

func (t *ConnectionTracker) Remove() int64 {
	return t.connections.Add(-1)
}

func (t *ConnectionTracker) Current() int64 {
	return t.connections.Load()
}

func (t *ConnectionTracker) SetMax(n int64) {
	atomic.StoreInt64(&t.maxConnections, n)
}

func (t *ConnectionTracker) Max() int64 {
	return atomic.LoadInt64(&t.maxConnections)
}

type PoolStats struct {
	Name      string
	Capacity  int64
	InUse     int64
	Idle      int64
	WaitCount int64
}

type PoolReporter struct {
	metrics *Metrics
	pools   map[string]*PoolStats
	mu      sync.RWMutex
}

func NewPoolReporter(m *Metrics) *PoolReporter {
	return &PoolReporter{
		metrics: m,
		pools:   make(map[string]*PoolStats),
	}
}

func (r *PoolReporter) Register(name string, capacity int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pools[name] = &PoolStats{Name: name, Capacity: capacity}
}

func (r *PoolReporter) Update(name string, inUse, idle, waitCount int64) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if stats, ok := r.pools[name]; ok {
		stats.InUse = inUse
		stats.Idle = idle
		stats.WaitCount = waitCount
		r.metrics.SetPoolCapacity(name, float64(stats.Capacity))
		r.metrics.SetPoolInUse(name, float64(inUse))
	}
}

func (r *PoolReporter) Report() []PoolStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]PoolStats, 0, len(r.pools))
	for _, s := range r.pools {
		result = append(result, *s)
	}
	return result
}

func FormatDuration(ms float64) string {
	if ms < 1 {
		return strconv.FormatFloat(ms*1000, 'f', 2, 64) + "ms"
	}
	return strconv.FormatFloat(ms, 'f', 2, 64) + "s"
}