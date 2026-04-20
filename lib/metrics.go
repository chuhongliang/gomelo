package lib

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
)

type Counter struct {
	name   string
	labels map[string]string
	value  uint64
	mu     sync.RWMutex
}

func NewCounter(name string) *Counter {
	return &Counter{name: name}
}

func (c *Counter) WithLabels(labels map[string]string) *Counter {
	return &Counter{name: c.name, labels: labels}
}

func (c *Counter) Add(v uint64) {
	if len(c.labels) == 0 {
		atomic.AddUint64(&c.value, v)
		return
	}
	key := makeKey(c.name, c.labels)
	GlobalRegistry().GetLabeledCounter(key).Add(v)
}

func makeKey(name string, labels map[string]string) string {
	if len(labels) == 0 {
		return name
	}
	parts := make([]string, 0, len(labels)*2+1)
	parts = append(parts, name)
	for k, v := range labels {
		parts = append(parts, k, v)
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += "\x00" + parts[i]
	}
	return result
}

func (c *Counter) Value() uint64 {
	if len(c.labels) == 0 {
		return atomic.LoadUint64(&c.value)
	}
	key := makeKey(c.name, c.labels)
	return GlobalRegistry().GetLabeledCounter(key).Value()
}

func (c *Counter) Name() string {
	return c.name
}

func (c *Counter) Labels() map[string]string {
	return c.labels
}

type Gauge struct {
	name   string
	labels map[string]string
	value  int64
}

func NewGauge(name string) *Gauge {
	return &Gauge{name: name}
}

func (g *Gauge) WithLabels(labels map[string]string) *Gauge {
	return &Gauge{name: g.name, labels: labels}
}

func (g *Gauge) Set(v int64) {
	if len(g.labels) == 0 {
		atomic.StoreInt64(&g.value, v)
		return
	}
	key := makeKey(g.name, g.labels)
	GlobalRegistry().GetLabeledGauge(key).Set(v)
}

func (g *Gauge) Inc() {
	if len(g.labels) == 0 {
		atomic.AddInt64(&g.value, 1)
		return
	}
	key := makeKey(g.name, g.labels)
	GlobalRegistry().GetLabeledGauge(key).Inc()
}

func (g *Gauge) Dec() {
	if len(g.labels) == 0 {
		atomic.AddInt64(&g.value, -1)
		return
	}
	key := makeKey(g.name, g.labels)
	GlobalRegistry().GetLabeledGauge(key).Dec()
}

func (g *Gauge) Add(v int64) {
	if len(g.labels) == 0 {
		atomic.AddInt64(&g.value, v)
		return
	}
	key := makeKey(g.name, g.labels)
	GlobalRegistry().GetLabeledGauge(key).Add(v)
}

func (g *Gauge) Value() int64 {
	if len(g.labels) == 0 {
		return atomic.LoadInt64(&g.value)
	}
	key := makeKey(g.name, g.labels)
	return GlobalRegistry().GetLabeledGauge(key).Value()
}

func (g *Gauge) Name() string {
	return g.name
}

func (g *Gauge) Labels() map[string]string {
	return g.labels
}

type Histogram struct {
	name    string
	labels  map[string]string
	buckets []int64
	counts  []uint64
	sum     int64
	count   uint64
	mu      sync.RWMutex
}

func NewHistogram(name string, buckets ...int64) *Histogram {
	if len(buckets) == 0 {
		buckets = []int64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 5000}
	}
	return &Histogram{
		name:    name,
		buckets: buckets,
		counts:  make([]uint64, len(buckets)+1),
	}
}

func (h *Histogram) WithLabels(labels map[string]string) *Histogram {
	if len(labels) == 0 {
		return h
	}
	return &Histogram{
		name:    h.name,
		labels:  labels,
		buckets: h.buckets,
		counts:  make([]uint64, len(h.buckets)+1),
	}
}

func (h *Histogram) Observe(v int64) {
	if len(h.labels) == 0 {
		atomic.AddInt64(&h.sum, v)
		atomic.AddUint64(&h.count, 1)
		for i, bound := range h.buckets {
			if v <= bound {
				atomic.AddUint64(&h.counts[i], 1)
			}
		}
		atomic.AddUint64(&h.counts[len(h.counts)-1], 1)
		return
	}
	key := makeKey(h.name, h.labels)
	GlobalRegistry().GetLabeledHistogram(key).Observe(v)
}

func (h *Histogram) Sum() int64 {
	if len(h.labels) == 0 {
		return atomic.LoadInt64(&h.sum)
	}
	key := makeKey(h.name, h.labels)
	return GlobalRegistry().GetLabeledHistogram(key).Sum()
}

func (h *Histogram) Count() uint64 {
	if len(h.labels) == 0 {
		return atomic.LoadUint64(&h.count)
	}
	key := makeKey(h.name, h.labels)
	return GlobalRegistry().GetLabeledHistogram(key).Count()
}

func (h *Histogram) Name() string {
	return h.name
}

func (h *Histogram) Labels() map[string]string {
	return h.labels
}

type MetricsRegistry struct {
	counters          map[string]*Counter
	gauges            map[string]*Gauge
	histograms        map[string]*Histogram
	labeledCounters   map[string]*Counter
	labeledGauges     map[string]*Gauge
	labeledHistograms map[string]*Histogram
	mu                sync.RWMutex
}

func NewMetricsRegistry() *MetricsRegistry {
	return &MetricsRegistry{
		counters:          make(map[string]*Counter),
		gauges:            make(map[string]*Gauge),
		histograms:        make(map[string]*Histogram),
		labeledCounters:   make(map[string]*Counter),
		labeledGauges:     make(map[string]*Gauge),
		labeledHistograms: make(map[string]*Histogram),
	}
}

func (r *MetricsRegistry) RegisterCounter(name string) *Counter {
	r.mu.Lock()
	defer r.mu.Unlock()

	if c, ok := r.counters[name]; ok {
		return c
	}

	c := &Counter{name: name}
	r.counters[name] = c
	return c
}

func (r *MetricsRegistry) RegisterGauge(name string) *Gauge {
	r.mu.Lock()
	defer r.mu.Unlock()

	if g, ok := r.gauges[name]; ok {
		return g
	}

	g := &Gauge{name: name}
	r.gauges[name] = g
	return g
}

func (r *MetricsRegistry) GetLabeledCounter(key string) *Counter {
	r.mu.RLock()
	c, ok := r.labeledCounters[key]
	r.mu.RUnlock()
	if ok {
		return c
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok = r.labeledCounters[key]; ok {
		return c
	}
	c = &Counter{name: key}
	r.labeledCounters[key] = c
	return c
}

func (r *MetricsRegistry) GetLabeledGauge(key string) *Gauge {
	r.mu.RLock()
	g, ok := r.labeledGauges[key]
	r.mu.RUnlock()
	if ok {
		return g
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if g, ok = r.labeledGauges[key]; ok {
		return g
	}
	g = &Gauge{name: key}
	r.labeledGauges[key] = g
	return g
}

func (r *MetricsRegistry) GetLabeledHistogram(key string) *Histogram {
	r.mu.RLock()
	h, ok := r.labeledHistograms[key]
	r.mu.RUnlock()
	if ok {
		return h
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if h, ok = r.labeledHistograms[key]; ok {
		return h
	}
	h = &Histogram{name: key}
	r.labeledHistograms[key] = h
	return h
}

func (r *MetricsRegistry) Counter(name string) *Counter {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.counters[name]
}

func (r *MetricsRegistry) Gauge(name string) *Gauge {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.gauges[name]
}

func (r *MetricsRegistry) Histogram(name string) *Histogram {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.histograms[name]
}

func (r *MetricsRegistry) RegisterHistogram(name string, buckets ...int64) *Histogram {
	r.mu.Lock()
	defer r.mu.Unlock()

	if h, ok := r.histograms[name]; ok {
		return h
	}

	h := NewHistogram(name, buckets...)
	r.histograms[name] = h
	return h
}

func (r *MetricsRegistry) Export() []MetricFamily {
	r.mu.RLock()
	defer r.mu.RUnlock()

	families := make([]MetricFamily, 0)

	for _, c := range r.counters {
		families = append(families, MetricFamily{
			Name:   c.Name(),
			Type:   "counter",
			Labels: c.Labels(),
			Value:  float64(c.Value()),
		})
	}

	for _, c := range r.labeledCounters {
		families = append(families, MetricFamily{
			Name:   c.Name(),
			Type:   "counter",
			Labels: c.Labels(),
			Value:  float64(c.Value()),
		})
	}

	for _, g := range r.gauges {
		families = append(families, MetricFamily{
			Name:   g.Name(),
			Type:   "gauge",
			Labels: g.Labels(),
			Value:  float64(g.Value()),
		})
	}

	for _, g := range r.labeledGauges {
		families = append(families, MetricFamily{
			Name:   g.Name(),
			Type:   "gauge",
			Labels: g.Labels(),
			Value:  float64(g.Value()),
		})
	}

	for _, h := range r.histograms {
		families = append(families, MetricFamily{
			Name:   h.Name(),
			Type:   "histogram",
			Labels: h.Labels(),
			Sum:    float64(h.Sum()),
			Count:  float64(h.Count()),
		})
	}

	for _, h := range r.labeledHistograms {
		families = append(families, MetricFamily{
			Name:   h.Name(),
			Type:   "histogram",
			Labels: h.Labels(),
			Sum:    float64(h.Sum()),
			Count:  float64(h.Count()),
		})
	}

	return families
}

type MetricFamily struct {
	Name   string            `json:"name"`
	Type   string            `json:"type"`
	Labels map[string]string `json:"labels,omitempty"`
	Value  float64           `json:"value,omitempty"`
	Sum    float64           `json:"sum,omitempty"`
	Count  float64           `json:"count,omitempty"`
}

type MetricsServer struct {
	addr     string
	registry *MetricsRegistry
	server   *http.Server
}

func NewMetricsServer(addr string, registry *MetricsRegistry) *MetricsServer {
	return &MetricsServer{
		addr:     addr,
		registry: registry,
	}
}

func (ms *MetricsServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	metrics := ms.registry.Export()

	format := r.URL.Query().Get("format")
	if format == "prometheus" {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		for _, m := range metrics {
			fmt.Fprintf(w, "%s", m.Name)
			if len(m.Labels) > 0 {
				fmt.Fprint(w, "{")
				first := true
				for k, v := range m.Labels {
					if !first {
						fmt.Fprint(w, ",")
					}
					fmt.Fprintf(w, `%s="%s"`, k, v)
					first = false
				}
				fmt.Fprint(w, "}")
			}
			switch m.Type {
			case "counter", "gauge":
				fmt.Fprintf(w, " %f\n", m.Value)
			case "histogram":
				fmt.Fprintf(w, "_sum %f\n", m.Sum)
				fmt.Fprintf(w, "_count %f\n", m.Count)
			}
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

func (ms *MetricsServer) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", ms.ServeHTTP)

	ms.server = &http.Server{
		Addr:    ms.addr,
		Handler: mux,
	}

	return ms.server.ListenAndServe()
}

func (ms *MetricsServer) Stop() error {
	if ms.server != nil {
		return ms.server.Close()
	}
	return nil
}

var (
	globalRegistry      = NewMetricsRegistry()
	globalMetricsServer *MetricsServer
)

func GlobalRegistry() *MetricsRegistry {
	return globalRegistry
}

func IncCounter(name string) {
	globalRegistry.RegisterCounter(name)
	if c := globalRegistry.Counter(name); c != nil {
		c.Add(1)
	}
}

func SetGauge(name string, v int64) {
	globalRegistry.RegisterGauge(name)
	if g := globalRegistry.Gauge(name); g != nil {
		g.Set(v)
	}
}

func ObserveHistogram(name string, v int64) {
	globalRegistry.RegisterHistogram(name)
	if h := globalRegistry.Histogram(name); h != nil {
		h.Observe(v)
	}
}

func StartMetricsServer(addr string) error {
	globalMetricsServer = NewMetricsServer(addr, globalRegistry)
	return globalMetricsServer.Start()
}

func StopMetricsServer() error {
	if globalMetricsServer != nil {
		return globalMetricsServer.Stop()
	}
	return nil
}
