package handler

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"net/http"
	"time"

	"task-service/task"

	"github.com/gorilla/mux"
)

type TaskHandler struct {
	Manager *task.Manager
}

func (h *TaskHandler) CreateTask(w http.ResponseWriter, r *http.Request) {
	t := h.Manager.CreateTask(func(ctx context.Context) (string, error) {
		// Симуляция долгой операции (3-5 минут)
		delay := time.Duration(rand.Intn(120)+180) * time.Second
		select {
		case <-time.After(delay):
			// Случайная ошибка
			if rand.Intn(10) == 0 {
				return "", errors.New("random error occurred")
			}
			return "Done!", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(t)
}

func (h *TaskHandler) GetTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	t, ok := h.Manager.GetTask(id)
	if !ok {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(t)
}

func (h *TaskHandler) DeleteTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	success := h.Manager.DeleteTask(id)
	if !success {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}