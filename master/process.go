package master

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type ProcessState int

const (
	ProcessStarting ProcessState = iota
	ProcessRunning
	ProcessStopping
	ProcessStopped
	ProcessCrashed
)

func (s ProcessState) String() string {
	switch s {
	case ProcessStarting:
		return "starting"
	case ProcessRunning:
		return "running"
	case ProcessStopping:
		return "stopping"
	case ProcessStopped:
		return "stopped"
	case ProcessCrashed:
		return "crashed"
	default:
		return "unknown"
	}
}

type ProcessInfo struct {
	ID         string
	ServerType string
	Host       string
	Port       int
	PID        int
	State      ProcessState
	StartTime  time.Time
	ExitCode   int
}

type Process interface {
	Info() *ProcessInfo
	Start() error
	Stop() error
	Stopped() <-chan struct{}
	Wait() (int, error)
}

type localProcess struct {
	info *ProcessInfo
	cmd  *exec.Cmd
	done chan struct{}
	mu   sync.Mutex
}

func newLocalProcess(id, serverType, exePath string, args []string, env []string) (Process, error) {
	cmd := exec.Command(exePath, args...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	info := &ProcessInfo{
		ID:         id,
		ServerType: serverType,
		State:      ProcessStopping,
		StartTime:  time.Now(),
	}

	for _, e := range env {
		if strings.HasPrefix(e, "GOMELO_HOST=") {
			info.Host = strings.TrimPrefix(e, "GOMELO_HOST=")
		} else if strings.HasPrefix(e, "GOMELO_PORT=") {
			if port, err := strconv.Atoi(strings.TrimPrefix(e, "GOMELO_PORT=")); err == nil {
				info.Port = port
			}
		}
	}

	return &localProcess{
		info: info,
		cmd:  cmd,
		done: make(chan struct{}),
	}, nil
}

func (p *localProcess) Info() *ProcessInfo {
	p.mu.Lock()
	defer p.mu.Unlock()
	info := *p.info
	info.PID = p.getPID()
	return &info
}

func (p *localProcess) getPID() int {
	if p.cmd != nil && p.cmd.Process != nil {
		return p.cmd.Process.Pid
	}
	return 0
}

func (p *localProcess) Start() error {
	p.mu.Lock()
	if p.info.State == ProcessRunning || p.info.State == ProcessStarting {
		p.mu.Unlock()
		return fmt.Errorf("process already running")
	}
	p.info.State = ProcessStarting
	p.info.StartTime = time.Now()
	p.mu.Unlock()

	if err := p.cmd.Start(); err != nil {
		p.mu.Lock()
		p.info.State = ProcessCrashed
		p.mu.Unlock()
		return fmt.Errorf("start process %s: %w", p.info.ID, err)
	}

	p.mu.Lock()
	p.info.PID = p.cmd.Process.Pid
	p.info.State = ProcessRunning
	p.done = make(chan struct{})
	p.mu.Unlock()

	return nil
}

func (p *localProcess) Stop() error {
	p.mu.Lock()
	if p.info.State != ProcessRunning && p.info.State != ProcessStarting {
		p.mu.Unlock()
		return nil
	}
	p.info.State = ProcessStopping
	p.mu.Unlock()

	if p.cmd != nil && p.cmd.Process != nil {
		p.cmd.Process.Signal(syscall.SIGTERM)
		go func() {
			p.cmd.Wait()
			close(p.done)
		}()
	}

	return nil
}

func (p *localProcess) Stopped() <-chan struct{} {
	return p.done
}

func (p *localProcess) Wait() (int, error) {
	err := p.cmd.Wait()
	p.mu.Lock()
	defer p.mu.Unlock()

	select {
	case <-p.done:
	default:
		close(p.done)
	}

	if err == nil {
		p.info.State = ProcessStopped
		p.info.ExitCode = 0
		return 0, nil
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		p.info.State = ProcessCrashed
		if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			p.info.ExitCode = ws.ExitStatus()
			return p.info.ExitCode, nil
		}
	}

	p.info.State = ProcessCrashed
	return -1, err
}

type ProcessManager interface {
	Spawn(id, serverType, exePath string, args, env []string) (Process, error)
	Kill(pid int) error
	List() []*ProcessInfo
	Find(id string) *ProcessInfo
	FindByPID(pid int) *ProcessInfo
	Watch(process Process, ch chan ProcessEvent)
	Close()
}

type ProcessEvent struct {
	ServerID   string
	ServerType string
	Host       string
	Port       int
	PID        int
	Event      string
	ExitCode   int
	Timestamp  time.Time
}

type localProcessManager struct {
	processes map[int]Process
	byID      map[string]Process
	mu        sync.RWMutex
}

func NewProcessManager() ProcessManager {
	return &localProcessManager{
		processes: make(map[int]Process),
		byID:      make(map[string]Process),
	}
}

func (pm *localProcessManager) Spawn(id, serverType, exePath string, args, env []string) (Process, error) {
	p, err := newLocalProcess(id, serverType, exePath, args, env)
	if err != nil {
		return nil, err
	}

	if err := p.Start(); err != nil {
		return nil, err
	}

	pm.mu.Lock()
	pm.processes[p.Info().PID] = p
	pm.byID[id] = p
	pm.mu.Unlock()

	return p, nil
}

func (pm *localProcessManager) Kill(pid int) error {
	pm.mu.RLock()
	p, ok := pm.processes[pid]
	pm.mu.RUnlock()
	if !ok {
		return fmt.Errorf("process not found: %d", pid)
	}
	return p.Stop()
}

func (pm *localProcessManager) List() []*ProcessInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make([]*ProcessInfo, 0, len(pm.processes))
	for _, p := range pm.processes {
		result = append(result, p.Info())
	}
	return result
}

func (pm *localProcessManager) Find(id string) *ProcessInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	if p, ok := pm.byID[id]; ok {
		return p.Info()
	}
	return nil
}

func (pm *localProcessManager) FindByPID(pid int) *ProcessInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	if p, ok := pm.processes[pid]; ok {
		return p.Info()
	}
	return nil
}

func (pm *localProcessManager) Watch(process Process, ch chan ProcessEvent) {
	go func() {
		info := process.Info()
		pid := info.PID
		exitCode, _ := process.Wait()

		event := ProcessEvent{
			ServerID:   info.ID,
			ServerType: info.ServerType,
			Host:       info.Host,
			Port:       info.Port,
			PID:        pid,
			Timestamp:  time.Now(),
		}

		if exitCode == 0 {
			event.Event = "stopped"
		} else {
			event.Event = "crashed"
			event.ExitCode = exitCode
		}

		pm.mu.Lock()
		delete(pm.processes, pid)
		delete(pm.byID, info.ID)
		pm.mu.Unlock()

		for i := 0; i < 10; i++ {
			select {
			case ch <- event:
				return
			case <-time.After(time.Millisecond):
			}
		}
	}()
}

func (pm *localProcessManager) Close() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for _, p := range pm.processes {
		p.Stop()
	}
	pm.processes = make(map[int]Process)
	pm.byID = make(map[string]Process)
}
