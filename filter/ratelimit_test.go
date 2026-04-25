package filter

import (
	"sync"
	"testing"
	"time"
)

func TestNewRateLimiter(t *testing.T) {
	rl := NewRateLimiter(10, 100)
	if rl == nil {
		t.Fatal("NewRateLimiter returned nil")
	}
	if rl.rate != 10 {
		t.Errorf("expected rate=10, got %d", rl.rate)
	}
	if rl.capacity != 100 {
		t.Errorf("expected capacity=100, got %d", rl.capacity)
	}
}

func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter(10, 10)

	for i := 0; i < 10; i++ {
		if !rl.Allow("key1") {
			t.Errorf("request %d should be allowed", i)
		}
	}

	if rl.Allow("key1") {
		t.Error("11th request should be denied")
	}
}

func TestRateLimiter_Allow_DifferentKeys(t *testing.T) {
	rl := NewRateLimiter(10, 10)

	for i := 0; i < 10; i++ {
		if !rl.Allow("key1") {
			t.Errorf("key1 request %d should be allowed", i)
		}
	}

	for i := 0; i < 5; i++ {
		if !rl.Allow("key2") {
			t.Errorf("key2 request %d should be allowed", i)
		}
	}
}

func TestRateLimiter_Allow_Refill(t *testing.T) {
	rl := NewRateLimiter(1000, 10)

	for i := 0; i < 10; i++ {
		rl.Allow("key1")
	}

	if rl.Allow("key1") {
		t.Error("should be denied after bucket exhausted")
	}

	time.Sleep(20 * time.Millisecond)

	if !rl.Allow("key1") {
		t.Error("should be allowed after refill")
	}
}

func TestRateLimiter_GetStats(t *testing.T) {
	rl := NewRateLimiter(10, 10)

	for i := 0; i < 5; i++ {
		rl.Allow("key1")
	}

	allowed, dropped := rl.GetStats()
	if allowed != 5 {
		t.Errorf("expected allowed=5, got %d", allowed)
	}
	if dropped != 0 {
		t.Errorf("expected dropped=0, got %d", dropped)
	}

	for i := 0; i < 10; i++ {
		rl.Allow("key2")
	}

	allowed2, _ := rl.GetStats()
	if allowed2 != 15 {
		t.Errorf("expected allowed=15, got %d", allowed2)
	}
}

func TestRateLimiter_GetStats_AfterDenial(t *testing.T) {
	rl := NewRateLimiter(10, 5)

	for i := 0; i < 5; i++ {
		rl.Allow("key1")
	}

	rl.Allow("key1")

	_, dropped := rl.GetStats()
	if dropped != 1 {
		t.Errorf("expected dropped=1, got %d", dropped)
	}
}

func TestRateLimiter_Concurrent(t *testing.T) {
	rl := NewRateLimiter(1000, 100)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				rl.Allow("key1")
			}
		}()
	}
	wg.Wait()

	allowed, _ := rl.GetStats()
	t.Logf("allowed: %d", allowed)
}

func TestRateLimiter_CleanupOldBuckets(t *testing.T) {
	rl := NewRateLimiter(10, 10)

	for i := 0; i < 100; i++ {
		rl.Allow(string(rune(i)))
	}

	rl.mu.RLock()
	bucketCount := len(rl.buckets)
	rl.mu.RUnlock()

	t.Logf("bucket count after 100 different keys: %d", bucketCount)
}

func TestNewRateLimiterFilter(t *testing.T) {
	f := NewRateLimiterFilter(10, 10, func(ctx interface{}) string {
		return "fixed-key"
	})
	if f == nil {
		t.Fatal("NewRateLimiterFilter returned nil")
	}
	if f.Name() != "rate-limiter" {
		t.Errorf("expected name='rate-limiter', got '%s'", f.Name())
	}
}

func TestRateLimiterFilter_Process(t *testing.T) {
	f := NewRateLimiterFilter(10, 10, func(ctx interface{}) string {
		return "test-key"
	})

	for i := 0; i < 10; i++ {
		if !f.Process("ctx") {
			t.Errorf("request %d should be allowed", i)
		}
	}

	if f.Process("ctx") {
		t.Error("11th request should be denied")
	}
}

func TestRateLimiterFilter_Process_DifferentKeys(t *testing.T) {
	f := NewRateLimiterFilter(10, 10, func(ctx interface{}) string {
		return ctx.(string)
	})

	for i := 0; i < 10; i++ {
		if !f.Process("key1") {
			t.Errorf("key1 request %d should be allowed", i)
		}
	}

	for i := 0; i < 5; i++ {
		if !f.Process("key2") {
			t.Errorf("key2 request %d should be allowed", i)
		}
	}
}

func TestRateLimiterFilter_GetStats(t *testing.T) {
	f := NewRateLimiterFilter(10, 10, func(ctx interface{}) string {
		return ctx.(string)
	})

	f.Process("key1")
	f.Process("key1")
	f.Process("key2")

	allowed, dropped := f.GetStats()
	if allowed != 3 {
		t.Errorf("expected allowed=3, got %d", allowed)
	}
	if dropped != 0 {
		t.Errorf("expected dropped=0, got %d", dropped)
	}
}

func TestRateLimiterFilter_After(t *testing.T) {
	f := NewRateLimiterFilter(10, 10, nil)
	f.After("ctx")
}

func TestIPKeyFunc(t *testing.T) {
	key := IPKeyFunc("192.168.1.1")
	if key != "192.168.1.1" {
		t.Errorf("expected '192.168.1.1', got '%s'", key)
	}

	key2 := IPKeyFunc(12345)
	if key2 != "12345" {
		t.Errorf("expected '12345', got '%s'", key2)
	}
}

func TestRateLimiter_Process(t *testing.T) {
	rl := NewRateLimiter(10, 10)

	for i := 0; i < 5; i++ {
		if !rl.Process("key1") {
			t.Errorf("request %d should be allowed", i)
		}
	}
}

func TestRateLimiter_After(t *testing.T) {
	rl := NewRateLimiter(10, 10)
	rl.After("key1")
}
