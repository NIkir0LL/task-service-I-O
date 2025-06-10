package task

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Manager struct {
	mu    sync.RWMutex
	tasks map[string]*Task
}

func NewManager() *Manager {
	return &Manager{
		tasks: make(map[string]*Task),
	}
}

func (m *Manager) CreateTask(runFunc func(ctx context.Context) (string, error)) *Task {
	id := uuid.New().String()
	ctx, cancel := context.WithCancel(context.Background())
	task := &Task{
		ID:        id,
		Status:    StatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		ctx:       ctx,
		cancel:    cancel,
		Run:       runFunc,
	}

	m.mu.Lock()
	m.tasks[id] = task
	m.mu.Unlock()

	go m.runTask(task)

	return task
}

func (m *Manager) runTask(task *Task) {
	m.updateStatus(task.ID, StatusRunning)
	result, err := task.Run(task.ctx)
	m.mu.Lock()
	defer m.mu.Unlock()
	if err != nil {
		if err == context.Canceled {
			task.Status = StatusCancelled
			task.Result = "Task cancelled"
		} else {
			task.Status = StatusFailed
			task.Result = err.Error()
		}
	} else {
		task.Status = StatusCompleted
		task.Result = result
	}
	task.UpdatedAt = time.Now()
	// Удаляем задачу после завершения
	delete(m.tasks, task.ID)
}

func (m *Manager) GetTask(id string) (*Task, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	task, ok := m.tasks[id]
	return task, ok
}

func (m *Manager) DeleteTask(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	task, ok := m.tasks[id]
	if !ok {
		return false
	}
	task.cancel() // Отменяем задачу
	// Не удаляем сразу, дадим runTask обработать
	return true
}

func (m *Manager) updateStatus(id string, status Status) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if task, ok := m.tasks[id]; ok {
		task.Status = status
		task.UpdatedAt = time.Now()
	}
}