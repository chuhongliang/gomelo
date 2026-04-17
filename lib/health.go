package lib

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type HealthChecker interface {
	Health() error
}

type HealthStatus struct {
	Status string                 `json:"status"`
	Time   string                 `json:"time"`
	Checks map[string]CheckResult `json:"checks,omitempty"`
}

type CheckResult struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Latency string `json:"latency,omitempty"`
}

type HealthCheck struct {
	name    string
	checker HealthChecker
}

type HealthServer struct {
	addr    string
	checks  map[string]*HealthCheck
	timeout time.Duration
	mu      sync.RWMutex
	server  *http.Server
}

func NewHealthServer(addr string) *HealthServer {
	return &HealthServer{
		addr:    addr,
		checks:  make(map[string]*HealthCheck),
		timeout: 5 * time.Second,
	}
}

func (hs *HealthServer) Register(name string, checker HealthChecker) {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	hs.checks[name] = &HealthCheck{
		name:    name,
		checker: checker,
	}
}

func (hs *HealthServer) Unregister(name string) {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	delete(hs.checks, name)
}

func (hs *HealthServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	hs.mu.RLock()
	checks := hs.checks
	hs.mu.RUnlock()

	results := make(map[string]CheckResult)
	allHealthy := true
	var mu sync.Mutex
	var wg sync.WaitGroup

	for name, check := range checks {
		wg.Add(1)
		go func(name string, check *HealthCheck) {
			defer wg.Done()

			start := time.Now()
			err := check.checker.Health()
			latency := time.Since(start)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				allHealthy = false
				results[name] = CheckResult{
					Status:  "unhealthy",
					Message: err.Error(),
					Latency: latency.String(),
				}
			} else {
				results[name] = CheckResult{
					Status:  "healthy",
					Latency: latency.String(),
				}
			}
		}(name, check)
	}

	wg.Wait()

	status := HealthStatus{
		Status: "healthy",
		Time:   time.Now().Format(time.RFC3339),
		Checks: results,
	}

	if !allHealthy {
		status.Status = "unhealthy"
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (hs *HealthServer) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", hs.ServeHTTP)
	mux.HandleFunc("/health/", hs.ServeHTTP)
	mux.HandleFunc("/ready", hs.ServeHTTP)
	mux.HandleFunc("/live", hs.ServeLiveness)

	hs.server = &http.Server{
		Addr:         hs.addr,
		Handler:      mux,
		ReadTimeout:  hs.timeout,
		WriteTimeout: hs.timeout,
	}

	return hs.server.ListenAndServe()
}

func (hs *HealthServer) ServeLiveness(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "ok")
}

func (hs *HealthServer) Stop() error {
	if hs.server != nil {
		return hs.server.Close()
	}
	return nil
}

func (hs *HealthServer) CheckAll() (map[string]CheckResult, bool) {
	hs.mu.RLock()
	checks := hs.checks
	hs.mu.RUnlock()

	results := make(map[string]CheckResult)
	allHealthy := true

	for name, check := range checks {
		start := time.Now()
		err := check.checker.Health()
		latency := time.Since(start)

		if err != nil {
			allHealthy = false
			results[name] = CheckResult{
				Status:  "unhealthy",
				Message: err.Error(),
				Latency: latency.String(),
			}
		} else {
			results[name] = CheckResult{
				Status:  "healthy",
				Latency: latency.String(),
			}
		}
	}

	return results, allHealthy
}

type CompositeChecker struct {
	name   string
	checks []HealthChecker
}

func NewCompositeChecker(name string, checks ...HealthChecker) *CompositeChecker {
	return &CompositeChecker{
		name:   name,
		checks: checks,
	}
}

func (cc *CompositeChecker) Add(check HealthChecker) {
	cc.checks = append(cc.checks, check)
}

func (cc *CompositeChecker) Health() error {
	for _, check := range cc.checks {
		if err := check.Health(); err != nil {
			return fmt.Errorf("%s: %w", cc.name, err)
		}
	}
	return nil
}

func (cc *CompositeChecker) AddTo(hs *HealthServer) {
	hs.Register(cc.name, cc)
}

type FuncChecker struct {
	name string
	fn   func() error
}

func (fc *FuncChecker) Health() error {
	return fc.fn()
}

func NewFuncChecker(name string, fn func() error) *FuncChecker {
	return &FuncChecker{name: name, fn: fn}
}
