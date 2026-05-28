package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	aierrs "github.com/samson-samson/aievo-next/pkg/errors"
)

// ===== TaskCreate / TaskUpdate / TaskList =====
// In-process task tracker, scoped per *agent run* (a TaskStore is created
// fresh per Run). Inspired by Claude Code's TaskCreate / Update / List /
// Get tools — these let the model maintain an explicit plan and report
// progress.

type TaskStatus string

const (
	StatusPending    TaskStatus = "pending"
	StatusInProgress TaskStatus = "in_progress"
	StatusCompleted  TaskStatus = "completed"
)

type Task struct {
	ID          string     `json:"id"`
	Subject     string     `json:"subject"`
	Description string     `json:"description,omitempty"`
	Status      TaskStatus `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
}

// TaskStore is the per-run task list. Constructed by NewTaskStore and the
// three tools below all close over the same instance.
type TaskStore struct {
	mu   sync.Mutex
	next int
	rows []Task
}

func NewTaskStore() *TaskStore { return &TaskStore{} }

func (s *TaskStore) snapshot() []Task {
	out := make([]Task, len(s.rows))
	copy(out, s.rows)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Tools returns the three TaskCreate/Update/List tools backed by s.
func (s *TaskStore) Tools() []taskTool {
	return []taskTool{
		{name: "TaskCreate", store: s, fn: s.create, desc: `Add a task. Input: {"subject":"...","description":"..."}.`,
			schema: []byte(`{"type":"object","properties":{"subject":{"type":"string"},"description":{"type":"string"}},"required":["subject"]}`)},
		{name: "TaskUpdate", store: s, fn: s.update, desc: `Update a task's status. Input: {"id":"1","status":"in_progress|completed|pending"}.`,
			schema: []byte(`{"type":"object","properties":{"id":{"type":"string"},"status":{"type":"string"}},"required":["id","status"]}`)},
		{name: "TaskList", store: s, fn: s.list, desc: `List all tasks. Input: {}.`,
			schema: []byte(`{"type":"object","properties":{}}`)},
	}
}

func (s *TaskStore) create(_ context.Context, input string) (string, error) {
	var in struct{ Subject, Description string }
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return "", aierrs.NewFatal(err)
	}
	if in.Subject == "" {
		return "", aierrs.NewFatal(fmt.Errorf("TaskCreate: subject required"))
	}
	s.mu.Lock()
	s.next++
	t := Task{
		ID: fmt.Sprintf("%d", s.next), Subject: in.Subject, Description: in.Description,
		Status: StatusPending, CreatedAt: time.Now(),
	}
	s.rows = append(s.rows, t)
	s.mu.Unlock()
	b, _ := json.Marshal(t)
	return string(b), nil
}

func (s *TaskStore) update(_ context.Context, input string) (string, error) {
	var in struct {
		ID     string
		Status string
	}
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return "", aierrs.NewFatal(err)
	}
	st := TaskStatus(in.Status)
	if st != StatusPending && st != StatusInProgress && st != StatusCompleted {
		return "", aierrs.NewFatal(fmt.Errorf("TaskUpdate: invalid status %q", in.Status))
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.rows {
		if s.rows[i].ID == in.ID {
			s.rows[i].Status = st
			b, _ := json.Marshal(s.rows[i])
			return string(b), nil
		}
	}
	return "", aierrs.NewFatal(fmt.Errorf("TaskUpdate: task %q not found", in.ID))
}

func (s *TaskStore) list(_ context.Context, _ string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, _ := json.Marshal(s.snapshot())
	return string(b), nil
}

// taskTool wraps a TaskStore method as a tool.Tool. Concurrency-safe — the
// store has its own mutex.
type taskTool struct {
	name, desc string
	schema     []byte
	store      *TaskStore
	fn         func(ctx context.Context, input string) (string, error)
}

func (t taskTool) Name() string             { return t.name }
func (t taskTool) Description() string      { return t.desc }
func (t taskTool) Schema() []byte           { return t.schema }
func (t taskTool) IsConcurrencySafe() bool  { return true }
func (t taskTool) Call(ctx context.Context, input string) (string, error) {
	return t.fn(ctx, input)
}
