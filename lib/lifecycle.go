package lib

import (
	"fmt"
	"sync"
)

type LifecycleManager struct {
	loaded     []Component
	components map[string]any
	mu         sync.RWMutex
}

func NewLifecycleManager() *LifecycleManager {
	return &LifecycleManager{
		loaded:     make([]Component, 0),
		components: make(map[string]any),
	}
}

func (lm *LifecycleManager) Register(name string, comp Component) {
	lm.mu.Lock()
	lm.components[name] = comp
	lm.loaded = append(lm.loaded, comp)
	lm.mu.Unlock()
}

func (lm *LifecycleManager) GetComponent(name string) (Component, bool) {
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	c, ok := lm.components[name]
	if !ok {
		return nil, false
	}
	comp, ok2 := c.(Component)
	return comp, ok2
}

func (lm *LifecycleManager) Load(name string, comp Component) {
	if name == "" {
		name = comp.Name()
	}
	lm.mu.Lock()
	if lm.components[name] != nil {
		lm.mu.Unlock()
		return
	}
	lm.loaded = append(lm.loaded, comp)
	lm.components[name] = comp
	lm.mu.Unlock()
}

func (lm *LifecycleManager) StartAll(app *App, cb func(err error)) {
	lm.mu.Lock()
	components := make([]Component, len(lm.loaded))
	copy(components, lm.loaded)
	lm.mu.Unlock()

	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	started := make([]Component, 0)

	wg.Add(len(components))
	for _, c := range components {
		go func(comp Component) {
			defer wg.Done()
			if err := comp.Start(app); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return
			}
			mu.Lock()
			started = append(started, comp)
			mu.Unlock()
		}(c)
	}
	wg.Wait()

	if firstErr != nil {
		for _, comp := range started {
			comp.Stop()
		}
		cb(firstErr)
		return
	}
	cb(nil)
}

func (lm *LifecycleManager) StopAll() error {
	lm.mu.Lock()
	components := make([]Component, len(lm.loaded))
	copy(components, lm.loaded)
	lm.mu.Unlock()

	var errs []error
	for i := len(components) - 1; i >= 0; i-- {
		if err := components[i].Stop(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("stop errors: %v", errs)
	}
	return nil
}
