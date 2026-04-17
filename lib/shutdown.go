package lib

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type GracefulShutdown struct {
	timeout    time.Duration
	components []Component
	onShutdown func() error
}

func NewGracefulShutdown(timeout time.Duration) *GracefulShutdown {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &GracefulShutdown{timeout: timeout}
}

func (gs *GracefulShutdown) WithComponents(comps ...Component) *GracefulShutdown {
	gs.components = append(gs.components, comps...)
	return gs
}

func (gs *GracefulShutdown) OnShutdown(fn func() error) *GracefulShutdown {
	gs.onShutdown = fn
	return gs
}

func (gs *GracefulShutdown) WaitAndShutdown() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigCh
	log.Printf("received signal %v, starting graceful shutdown...", sig)

	ctx, cancel := context.WithTimeout(context.Background(), gs.timeout)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- gs.shutdown(ctx)
	}()

	select {
	case <-ctx.Done():
		log.Printf("shutdown timed out after %v, forcing exit", gs.timeout)
		os.Exit(1)
	case err := <-done:
		if err != nil {
			log.Printf("shutdown completed with error: %v", err)
			os.Exit(1)
		}
		log.Printf("shutdown completed successfully")
		os.Exit(0)
	}
}

func (gs *GracefulShutdown) shutdown(ctx context.Context) error {
	if gs.onShutdown != nil {
		if err := gs.onShutdown(); err != nil {
			return fmt.Errorf("onShutdown: %w", err)
		}
	}

	for i, comp := range gs.components {
		log.Printf("stopping component %d/%d: %T", i+1, len(gs.components), comp)
		comp.Stop()
	}

	return nil
}

func (gs *GracefulShutdown) Shutdown() error {
	return gs.shutdown(context.Background())
}

func (gs *GracefulShutdown) ShutdownWithTimeout(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return gs.shutdown(ctx)
}

type ShutdownManager struct {
	timeout        time.Duration
	lifecycleHooks []Component
	cleanupFuncs   []func() error
	forceOnTimeout bool
}

func NewShutdownManager(timeout time.Duration) *ShutdownManager {
	return &ShutdownManager{
		timeout:        timeout,
		lifecycleHooks: make([]Component, 0),
		cleanupFuncs:   make([]func() error, 0),
		forceOnTimeout: true,
	}
}

func (sm *ShutdownManager) AddLifecycle(hook Component) {
	sm.lifecycleHooks = append(sm.lifecycleHooks, hook)
}

func (sm *ShutdownManager) AddCleanup(fn func() error) {
	sm.cleanupFuncs = append(sm.cleanupFuncs, fn)
}

func (sm *ShutdownManager) SetForceOnTimeout(force bool) {
	sm.forceOnTimeout = force
}

func (sm *ShutdownManager) Run() error {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigCh
	log.Printf("received signal %v", sig)

	ctx, cancel := context.WithTimeout(context.Background(), sm.timeout)
	defer cancel()

	errCh := make(chan error, len(sm.cleanupFuncs)+len(sm.lifecycleHooks))
	var wg sync.WaitGroup
	wg.Add(len(sm.cleanupFuncs) + len(sm.lifecycleHooks))

	for _, fn := range sm.cleanupFuncs {
		go func(f func() error) {
			defer wg.Done()
			if err := f(); err != nil {
				select {
				case errCh <- err:
				default:
				}
			}
		}(fn)
	}

	for _, hook := range sm.lifecycleHooks {
		go func(h Component) {
			defer wg.Done()
			h.Stop()
		}(hook)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	var errs []error
	for {
		select {
		case <-done:
			if len(errs) > 0 {
				return errs[0]
			}
			return nil
		case <-ctx.Done():
			return context.DeadlineExceeded
		case err := <-errCh:
			errs = append(errs, err)
		}
	}
}
