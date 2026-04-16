package filter

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type RateLimiter struct {
	buckets  map[string]*bucket
	rate     int
	capacity int
	mu       sync.RWMutex
	stats    struct {
		allowed int64
		dropped int64
	}
}

type bucket struct {
	tokens   float64
	lastTime time.Time
	mu       sync.Mutex
}

func NewRateLimiter(rate, capacity int) *RateLimiter {
	rl := &RateLimiter{
		buckets:  make(map[string]*bucket),
		rate:     rate,
		capacity: capacity,
	}
	return rl
}

func (r *RateLimiter) Allow(key string) bool {
	b := r.getBucket(key)
	if b == nil {
		return false
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastTime).Seconds()
	b.lastTime = now

	b.tokens += elapsed * float64(r.rate)
	if b.tokens > float64(r.capacity) {
		b.tokens = float64(r.capacity)
	}

	if b.tokens >= 1 {
		b.tokens--
		atomic.AddInt64(&r.stats.allowed, 1)
		return true
	}

	atomic.AddInt64(&r.stats.dropped, 1)
	return false
}

func (r *RateLimiter) getBucket(key string) *bucket {
	r.mu.RLock()
	b, ok := r.buckets[key]
	r.mu.RUnlock()

	if ok {
		return b
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if b, ok = r.buckets[key]; ok {
		return b
	}

	b = &bucket{
		tokens:   float64(r.capacity),
		lastTime: time.Now(),
	}
	r.buckets[key] = b

	if len(r.buckets) > 10000 {
		r.cleanupOldBuckets()
	}

	return b
}

func (r *RateLimiter) cleanupOldBuckets() {
	for k := range r.buckets {
		if len(r.buckets) <= 5000 {
			return
		}
		delete(r.buckets, k)
	}
}

func (r *RateLimiter) Process(ctx interface{}) bool {
	key := r.getKey(ctx)
	return r.Allow(key)
}

func (r *RateLimiter) After(ctx interface{}) {}

func (r *RateLimiter) getKey(ctx interface{}) string {
	return fmt.Sprintf("%v", ctx)
}

func (r *RateLimiter) GetStats() (allowed, dropped int64) {
	allowed = atomic.LoadInt64(&r.stats.allowed)
	dropped = atomic.LoadInt64(&r.stats.dropped)
	return
}

type RateLimiterFilter struct {
	limiter *RateLimiter
	keyFunc func(interface{}) string
}

func NewRateLimiterFilter(rate, capacity int, keyFunc func(interface{}) string) *RateLimiterFilter {
	return &RateLimiterFilter{
		limiter: NewRateLimiter(rate, capacity),
		keyFunc: keyFunc,
	}
}

func (f *RateLimiterFilter) Name() string { return "rate-limiter" }

func (f *RateLimiterFilter) Process(ctx interface{}) bool {
	key := f.keyFunc(ctx)
	return f.limiter.Allow(key)
}

func (f *RateLimiterFilter) After(ctx interface{}) {}

func (f *RateLimiterFilter) GetStats() (allowed, dropped int64) {
	return f.limiter.GetStats()
}

type IPSourceKeyFunc func(interface{}) string

func IPKeyFunc(ctx interface{}) string {
	return fmt.Sprintf("%v", ctx)
}
