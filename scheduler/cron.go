package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"gomelo/loader"

	"github.com/robfig/cron/v3"
)

type CronScheduler struct {
	serverType  string
	cron        *cron.Cron
	configPath  string
	configs     map[string]map[string]string
	ldr         *loader.Loader
	mu          sync.RWMutex
	methods     map[string]map[string]*CronMethodInfo
	initialized bool
	stopCtx     context.Context
	stopCancel  context.CancelFunc
}

type CronMethodInfo struct {
	Method reflect.Method
	Cron   any
}

func NewCronScheduler(serverType string, configPath string, ldr *loader.Loader) *CronScheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &CronScheduler{
		serverType: serverType,
		configPath: configPath,
		ldr:        ldr,
		cron:       cron.New(cron.WithSeconds()),
		configs:    make(map[string]map[string]string),
		methods:    make(map[string]map[string]*CronMethodInfo),
		stopCtx:    ctx,
		stopCancel: cancel,
	}
}

func (cs *CronScheduler) LoadConfig() error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.configPath == "" {
		return fmt.Errorf("cron config path is empty")
	}

	data, err := os.ReadFile(cs.configPath)
	if err != nil {
		return fmt.Errorf("read cron config: %w", err)
	}

	var configs map[string]map[string]string
	if err := json.Unmarshal(data, &configs); err != nil {
		return fmt.Errorf("parse cron config: %w", err)
	}

	cs.configs = configs
	return nil
}

func (cs *CronScheduler) RegisterMethods() {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.ldr == nil {
		return
	}

	crons := cs.ldr.GetAllCrons()
	serverCrons, ok := crons[cs.serverType]
	if !ok {
		return
	}

	for key, cm := range serverCrons {
		parts := splitKey(key)
		if len(parts) != 2 {
			continue
		}
		cronName := parts[0]
		methodName := parts[1]

		if _, exists := cs.configs[cronName]; !exists {
			continue
		}
		if _, specExists := cs.configs[cronName][methodName]; !specExists {
			continue
		}

		if cs.methods[cronName] == nil {
			cs.methods[cronName] = make(map[string]*CronMethodInfo)
		}
		cs.methods[cronName][methodName] = &CronMethodInfo{
			Method: cm.Method,
			Cron:   cm.Cron,
		}
	}
}

func splitKey(key string) []string {
	return strings.Split(key, ".")
}

func (cs *CronScheduler) Start() error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.initialized {
		return nil
	}

	cs.cron.Start()

	for cronName, methods := range cs.methods {
		for methodName, info := range methods {
			spec, ok := cs.configs[cronName][methodName]
			if !ok {
				continue
			}

			if !isValidCronSpec(spec) {
				log.Printf("Invalid cron spec for %s.%s: %s", cronName, methodName, spec)
				continue
			}

			spec = convertToCronV3Spec(spec)

			method := info.Method
			cron := info.Cron

			_, err := cs.cron.AddFunc(spec, func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("cron panic recovered: %v", r)
					}
				}()

				select {
				case <-cs.stopCtx.Done():
					return
				default:
				}
				args := []reflect.Value{
					reflect.ValueOf(cron),
					reflect.ValueOf(cs.stopCtx),
				}
				method.Func.Call(args)
			})
			if err != nil {
				log.Printf("Failed to add cron %s.%s: %v", cronName, methodName, err)
				continue
			}

			log.Printf("Registered cron: %s.%s (%s)", cronName, methodName, spec)
		}
	}

	cs.initialized = true
	return nil
}

func (cs *CronScheduler) Stop() {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.stopCancel()

	if cs.cron != nil {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		ctx := cs.cron.Stop()
		select {
		case <-ctx.Done():
		case <-stopCtx.Done():
			log.Printf("cron stop timed out")
		}
	}
	cs.initialized = false
}

func (cs *CronScheduler) IsInitialized() bool {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.initialized
}

func convertToCronV3Spec(spec string) string {
	parts := strings.Fields(spec)
	if len(parts) == 5 {
		return "0 " + spec
	}
	return spec
}

func isValidCronSpec(spec string) bool {
	parts := strings.Fields(spec)
	return len(parts) == 5 || len(parts) == 6
}

type CronManager struct {
	schedulers map[string]*CronScheduler
	mu         sync.RWMutex
	basePath   string
	ldr        *loader.Loader
}

func NewCronManager(basePath string, ldr *loader.Loader) *CronManager {
	return &CronManager{
		schedulers: make(map[string]*CronScheduler),
		basePath:   basePath,
		ldr:        ldr,
	}
}

func (cm *CronManager) StartAll() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.ldr.Load()

	crons := cm.ldr.GetAllCrons()
	for serverType := range crons {
		configPath := filepath.Join(cm.basePath, "..", "config", serverType, "cron.json")
		scheduler := NewCronScheduler(serverType, configPath, cm.ldr)

		if err := scheduler.LoadConfig(); err != nil {
			log.Printf("Warning: failed to load cron config for %s: %v", serverType, err)
			continue
		}

		scheduler.RegisterMethods()
		if err := scheduler.Start(); err != nil {
			log.Printf("Warning: failed to start cron scheduler for %s: %v", serverType, err)
			continue
		}

		cm.schedulers[serverType] = scheduler
	}

	return nil
}

func (cm *CronManager) StopAll() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for _, scheduler := range cm.schedulers {
		scheduler.Stop()
	}
	cm.schedulers = make(map[string]*CronScheduler)
}
