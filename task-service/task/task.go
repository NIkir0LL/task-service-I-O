package task

import (
	"context"
	"time"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled" // Новый статус
)

type Task struct {
	ID        string        `json:"id"`
	Status    Status        `json:"status"`
	Result    string        `json:"result,omitempty"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
	ctx       context.Context
	cancel    context.CancelFunc
	Run       func(ctx context.Context) (string, error) `json:"-"`
}