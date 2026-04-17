package lib

import (
	"testing"
)

func TestCounter(t *testing.T) {
	c := NewCounter("test_counter")
	c.Add(10)

	if c.Value() != 10 {
		t.Errorf("expected 10, got %d", c.Value())
	}

	c.Add(5)
	if c.Value() != 15 {
		t.Errorf("expected 15, got %d", c.Value())
	}
}

func TestGauge(t *testing.T) {
	g := NewGauge("test_gauge")

	g.Set(100)
	if g.Value() != 100 {
		t.Errorf("expected 100, got %d", g.Value())
	}

	g.Inc()
	if g.Value() != 101 {
		t.Errorf("expected 101, got %d", g.Value())
	}

	g.Dec()
	if g.Value() != 100 {
		t.Errorf("expected 100, got %d", g.Value())
	}

	g.Add(50)
	if g.Value() != 150 {
		t.Errorf("expected 150, got %d", g.Value())
	}
}

func TestHistogram(t *testing.T) {
	h := NewHistogram("test_histogram", 1, 5, 10, 50, 100)

	h.Observe(3)
	h.Observe(7)
	h.Observe(15)

	if h.Count() != 3 {
		t.Errorf("expected count 3, got %d", h.Count())
	}

	if h.Sum() != 25 {
		t.Errorf("expected sum 25, got %d", h.Sum())
	}
}

func TestMetricsRegistry(t *testing.T) {
	registry := NewMetricsRegistry()

	c := registry.RegisterCounter("requests")
	c.Add(5)

	g := registry.RegisterGauge("connections")
	g.Set(10)

	h := registry.RegisterHistogram("latency", 1, 10, 100)
	h.Observe(50)

	if registry.Counter("requests") == nil {
		t.Error("counter not found")
	}

	if registry.Gauge("connections") == nil {
		t.Error("gauge not found")
	}

	if registry.Histogram("latency") == nil {
		t.Error("histogram not found")
	}
}

func TestGlobalRegistry(t *testing.T) {
	IncCounter("global_test")
	SetGauge("global_gauge", 42)
	ObserveHistogram("global_hist", 100)

	if GlobalRegistry().Counter("global_test") == nil {
		t.Error("global counter not registered")
	}
}
