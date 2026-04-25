package scheduler

import (
	"container/heap"
	"sync"
	"testing"
	"time"
)

type mockTaskHandler struct {
	mu      sync.Mutex
	pushUIDCalls     []string
	pushServerCalls []string
}

func (m *mockTaskHandler) HandlePushToUID(uid string, route string, msg any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pushUIDCalls = append(m.pushUIDCalls, uid)
}

func (m *mockTaskHandler) HandlePushToServer(serverID string, route string, msg any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pushServerCalls = append(m.pushServerCalls, serverID)
}

func TestNewScheduler(t *testing.T) {
	s := New(4, 1024)
	if s == nil {
		t.Fatal("New() returned nil")
	}
	if s.workers != 4 {
		t.Errorf("expected workers=4, got %d", s.workers)
	}
	if cap(s.tasks) != 1024 {
		t.Errorf("expected queue size=1024, got %d", cap(s.tasks))
	}
}

func TestNewScheduler_DefaultValues(t *testing.T) {
	s := New(0, 0)
	if s.workers != 4 {
		t.Errorf("expected default workers=4, got %d", s.workers)
	}
	if cap(s.tasks) != 1024 {
		t.Errorf("expected default queue size=1024, got %d", cap(s.tasks))
	}
}

func TestScheduler_StartStop(t *testing.T) {
	s := New(2, 100)
	handler := &mockTaskHandler{}
	s.SetHandler(handler)
	s.Start()
	s.Stop()
}

func TestScheduler_StartStop_Multiple(t *testing.T) {
	s := New(2, 100)
	s.Start()
	s.Start()
	s.Stop()
	s.Stop()
}

func TestScheduler_PushToUID(t *testing.T) {
	s := New(1, 100)
	handler := &mockTaskHandler{}
	s.SetHandler(handler)
	s.Start()
	defer s.Stop()

	s.PushToUID("user1", "chat.push", "hello")
	time.Sleep(10 * time.Millisecond)

	handler.mu.Lock()
	defer handler.mu.Unlock()
	if len(handler.pushUIDCalls) != 1 {
		t.Errorf("expected 1 push call, got %d", len(handler.pushUIDCalls))
	}
	if handler.pushUIDCalls[0] != "user1" {
		t.Errorf("expected uid=user1, got %s", handler.pushUIDCalls[0])
	}
}

func TestScheduler_PushToServer(t *testing.T) {
	s := New(1, 100)
	handler := &mockTaskHandler{}
	s.SetHandler(handler)
	s.Start()
	defer s.Stop()

	s.PushToServer("server-1", "broadcast.push", "hello")
	time.Sleep(10 * time.Millisecond)

	handler.mu.Lock()
	defer handler.mu.Unlock()
	if len(handler.pushServerCalls) != 1 {
		t.Errorf("expected 1 push call, got %d", len(handler.pushServerCalls))
	}
	if handler.pushServerCalls[0] != "server-1" {
		t.Errorf("expected serverID=server-1, got %s", handler.pushServerCalls[0])
	}
}

func TestScheduler_Push_QueueFull(t *testing.T) {
	s := New(1, 2)
	handler := &mockTaskHandler{}
	s.SetHandler(handler)
	s.Start()
	defer s.Stop()

	s.PushToUID("user1", "route", "msg1")
	s.PushToUID("user2", "route", "msg2")
	s.PushToUID("user3", "route", "msg3")

	time.Sleep(10 * time.Millisecond)
}

func TestScheduler_PushAfterStop(t *testing.T) {
	s := New(1, 100)
	s.Start()
	s.Stop()

	s.PushToUID("user1", "route", "msg")
}

func TestNewDispatcher(t *testing.T) {
	d := NewDispatcher(4)
	if d == nil {
		t.Fatal("NewDispatcher() returned nil")
	}
	if d.scheduler == nil {
		t.Error("expected scheduler to be initialized")
	}
}

func TestDispatcher_StartStop(t *testing.T) {
	d := NewDispatcher(2)
	d.Start()
	d.Stop()
}

func TestDispatcher_Push(t *testing.T) {
	d := NewDispatcher(1)
	handler := &mockTaskHandler{}
	d.scheduler.SetHandler(handler)
	d.Start()
	defer d.Stop()

	d.Push("chat.push", "hello", "user1", "user2")
	time.Sleep(20 * time.Millisecond)

	handler.mu.Lock()
	defer handler.mu.Unlock()
	if len(handler.pushUIDCalls) != 2 {
		t.Errorf("expected 2 push calls, got %d", len(handler.pushUIDCalls))
	}
}

func TestDispatcher_Broadcast(t *testing.T) {
	d := NewDispatcher(1)
	handler := &mockTaskHandler{}
	d.scheduler.SetHandler(handler)
	d.Start()
	defer d.Stop()

	d.Broadcast("connector", "broadcast.push", "hello all")
	time.Sleep(20 * time.Millisecond)

	handler.mu.Lock()
	defer handler.mu.Unlock()
	if len(handler.pushServerCalls) != 1 {
		t.Errorf("expected 1 broadcast call, got %d", len(handler.pushServerCalls))
	}
}

func TestDispatcher_GetStats(t *testing.T) {
	d := NewDispatcher(1)
	handler := &mockTaskHandler{}
	d.scheduler.SetHandler(handler)
	d.Start()
	defer d.Stop()

	d.Push("route", "msg", "user1", "user2")
	d.Broadcast("connector", "route", "msg")
	time.Sleep(20 * time.Millisecond)

	total, failed, _ := d.GetStats()
	if total != 2 {
		t.Errorf("expected total=2, got %d", total)
	}
	if failed != 0 {
		t.Errorf("expected failed=0, got %d", failed)
	}
}

func TestNewPriorityScheduler(t *testing.T) {
	p := NewPriorityScheduler(2)
	if p == nil {
		t.Fatal("NewPriorityScheduler() returned nil")
	}
	if p.workers != 2 {
		t.Errorf("expected workers=2, got %d", p.workers)
	}
}

func TestPriorityScheduler_StartStop(t *testing.T) {
	p := NewPriorityScheduler(2)
	p.Start()
	p.Stop()
}

func TestPriorityScheduler_Add(t *testing.T) {
	p := NewPriorityScheduler(1)
	p.Start()
	defer p.Stop()

	task := &TaskExt{
		Task: Task{
			UID:    "user1",
			Route:  "chat.push",
			Message: "hello",
		},
		Priority: 1,
	}

	p.Add(task)
}

func TestPriorityScheduler_Add_WithExpire(t *testing.T) {
	p := NewPriorityScheduler(1)
	p.Start()
	defer p.Stop()

	task := &TaskExt{
		Task: Task{
			UID:    "user1",
			Route:  "chat.push",
			Message: "hello",
		},
		Priority:   1,
		ExpireTime: time.Now().Add(time.Hour),
	}

	p.Add(task)
}

func TestPriorityScheduler_TaskOrdering(t *testing.T) {
	p := NewPriorityScheduler(1)
	p.Start()
	defer p.Stop()

	task1 := &TaskExt{
		Task: Task{UID: "user1"},
		Priority: 1,
	}
	task2 := &TaskExt{
		Task: Task{UID: "user2"},
		Priority: 10,
	}

	p.Add(task1)
	p.Add(task2)
}

func TestTaskHeap_Len(t *testing.T) {
	h := taskHeap{
		{Task: Task{UID: "1"}, Priority: 1},
		{Task: Task{UID: "2"}, Priority: 2},
	}
	if h.Len() != 2 {
		t.Errorf("expected len=2, got %d", h.Len())
	}
}

func TestTaskHeap_Less(t *testing.T) {
	h := taskHeap{
		{Task: Task{UID: "1"}, Priority: 10},
		{Task: Task{UID: "2"}, Priority: 1},
	}
	if !h.Less(0, 1) {
		t.Error("expected task[0] < task[1] (priority 10 > 1, higher priority first)")
	}
}

func TestTaskHeap_Swap(t *testing.T) {
	h := taskHeap{
		{Task: Task{UID: "1"}, Priority: 1},
		{Task: Task{UID: "2"}, Priority: 2},
	}
	h.Swap(0, 1)
	if h[0].Priority != 2 || h[1].Priority != 1 {
		t.Error("Swap did not work correctly")
	}
}

func TestTaskHeap_PushPop(t *testing.T) {
	h := &taskHeap{}
	heap.Push(h, &TaskExt{Task: Task{UID: "1"}, Priority: 1})
	heap.Push(h, &TaskExt{Task: Task{UID: "2"}, Priority: 2})

	if h.Len() != 2 {
		t.Errorf("expected len=2, got %d", h.Len())
	}

	item := heap.Pop(h)
	if item == nil {
		t.Fatal("Pop returned nil")
	}
	task := item.(*TaskExt)
	if task.Priority != 2 {
		t.Errorf("expected priority=2 (higher first), got %d", task.Priority)
	}
}
