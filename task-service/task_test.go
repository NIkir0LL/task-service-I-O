package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"task-service/handler"
	"task-service/task"
)

// TaskResponse представляет структуру ответа при создании задачи
type TaskResponse struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// TestMassiveTaskCreation проверяет создание большого количества задач
func TestMassiveTaskCreation(t *testing.T) {
	r := setupRouter()
	const numTasks = 1000
	
	start := time.Now()
	
	var taskIDs []string
	var mu sync.Mutex
	var wg sync.WaitGroup
	var errors int32
	
	wg.Add(numTasks)

	// Создаем задачи параллельно
	for i := 0; i < numTasks; i++ {
		go func(index int) {
			defer wg.Done()
			
			req, _ := http.NewRequest("POST", "/tasks", nil)
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)
			
			if rr.Code != http.StatusOK {
				atomic.AddInt32(&errors, 1)
				return
			}
			
			var taskResp TaskResponse
			if err := json.NewDecoder(rr.Body).Decode(&taskResp); err != nil {
				atomic.AddInt32(&errors, 1)
				return
			}
			
			if taskResp.ID == "" {
				atomic.AddInt32(&errors, 1)
				return
			}
			
			mu.Lock()
			taskIDs = append(taskIDs, taskResp.ID)
			mu.Unlock()
		}(i)
	}

	// Ждем завершения всех горутин
	wg.Wait()
	
	duration := time.Since(start)
	
	t.Logf("Создано %d задач за %v", len(taskIDs), duration)
	t.Logf("Скорость: %.2f задач/сек", float64(len(taskIDs))/duration.Seconds())
	
	if errors > 0 {
		t.Errorf("Произошло %d ошибок при создании задач", errors)
	}

	// Проверяем, что большинство задач созданы
	if len(taskIDs) < numTasks*9/10 { // Допускаем до 10% потерь
		t.Errorf("Создано слишком мало задач: %d из %d", len(taskIDs), numTasks)
	}

	// Проверяем уникальность ID
	idMap := make(map[string]bool)
	duplicates := 0
	for _, id := range taskIDs {
		if idMap[id] {
			duplicates++
		}
		idMap[id] = true
	}
	
	if duplicates > 0 {
		t.Errorf("Найдено %d дублирующихся ID", duplicates)
	}
}

// StatusResponse представляет структуру ответа при получении статуса задачи
type StatusResponse struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	Result    string    `json:"result,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// setupRouter настраивает маршрутизатор для тестов
func setupRouter() *mux.Router {
	manager := task.NewManager()
	taskHandler := &handler.TaskHandler{Manager: manager}
	r := mux.NewRouter()
	r.HandleFunc("/tasks", taskHandler.CreateTask).Methods("POST")
	r.HandleFunc("/tasks/{id}", taskHandler.GetTask).Methods("GET")
	r.HandleFunc("/tasks/{id}", taskHandler.DeleteTask).Methods("DELETE")
	return r
}

// TestCreateTask проверяет создание задачи
func TestCreateTask(t *testing.T) {
	r := setupRouter()
	req, _ := http.NewRequest("POST", "/tasks", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Ожидался статус 200, получен %v", status)
	}

	var taskResp TaskResponse
	if err := json.NewDecoder(rr.Body).Decode(&taskResp); err != nil {
		t.Errorf("Ошибка декодирования ответа: %v", err)
	}

	if taskResp.ID == "" {
		t.Error("Ожидался непустой ID задачи")
	}
	// Задача может быть в статусе "pending" или "running" в зависимости от скорости выполнения
	if taskResp.Status != "pending" && taskResp.Status != "running" {
		t.Errorf("Ожидался статус 'pending' или 'running', получен '%s'", taskResp.Status)
	}
}

// TestGetTaskStatus проверяет получение статуса задачи
func TestGetTaskStatus(t *testing.T) {
	r := setupRouter()

	// Сначала создаем задачу
	req, _ := http.NewRequest("POST", "/tasks", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	var taskResp TaskResponse
	json.NewDecoder(rr.Body).Decode(&taskResp)
	taskID := taskResp.ID

	// Проверяем статус "pending" сразу после создания
	statusReq, _ := http.NewRequest("GET", "/tasks/"+taskID, nil)
	statusRR := httptest.NewRecorder()
	r.ServeHTTP(statusRR, statusReq)
	var statusResp StatusResponse
	json.NewDecoder(statusRR.Body).Decode(&statusResp)
	
	if statusResp.Status != "pending" {
		t.Errorf("Ожидался статус 'pending', получен '%s'", statusResp.Status)
	}

	// Ждем немного и проверяем переход в "running"
	time.Sleep(2 * time.Second)

	statusReq, _ = http.NewRequest("GET", "/tasks/"+taskID, nil)
	statusRR = httptest.NewRecorder()
	r.ServeHTTP(statusRR, statusReq)
	json.NewDecoder(statusRR.Body).Decode(&statusResp)
	
	if statusResp.Status != "running" {
		t.Errorf("Ожидался статус 'running', получен '%s'", statusResp.Status)
	}
	// Проверяем, что время обновления больше времени создания (задача выполняется)
	if !statusResp.UpdatedAt.After(statusResp.CreatedAt) {
		t.Error("UpdatedAt должен быть больше CreatedAt для выполняющейся задачи")
	}

	// Ждем завершения задачи (максимум 300 секунд)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	
	for {
		select {
		case <-ctx.Done():
			t.Errorf("Задача не завершилась в течение 300 секунд")
			return
		default:
			statusReq, _ := http.NewRequest("GET", "/tasks/"+taskID, nil)
			statusRR := httptest.NewRecorder()
			r.ServeHTTP(statusRR, statusReq)
			
			// Если задача не найдена, значит она завершилась и была удалена
			if statusRR.Code == http.StatusNotFound {
				t.Log("Задача завершилась и была удалена из менеджера")
				return
			}
			
			var statusResp StatusResponse
			json.NewDecoder(statusRR.Body).Decode(&statusResp)
			
			if statusResp.Status == "completed" || statusResp.Status == "failed" {
				if statusResp.Status == "completed" {
					if statusResp.Result != "Done!" {
						t.Errorf("Ожидался результат 'Done!', получен '%s'", statusResp.Result)
					}
				} else if statusResp.Status == "failed" {
					if statusResp.Result == "" {
						t.Error("Ожидалась ошибка в поле result, но оно пустое")
					}
				}
				// Проверяем, что время обновления больше времени создания
				if !statusResp.UpdatedAt.After(statusResp.CreatedAt) {
					t.Error("UpdatedAt должен быть больше CreatedAt для завершенной задачи")
				}
				return
			}
			time.Sleep(1 * time.Second)
		}
	}
}

// TestDeleteTask проверяет удаление и отмену задачи
func TestDeleteTask(t *testing.T) {
	r := setupRouter()

	// Создаем задачу
	req, _ := http.NewRequest("POST", "/tasks", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	var taskResp TaskResponse
	json.NewDecoder(rr.Body).Decode(&taskResp)
	taskID := taskResp.ID

	// Ждем, чтобы задача начала выполняться
	time.Sleep(2 * time.Second)

	// Проверяем, что задача в статусе "running"
	statusReq, _ := http.NewRequest("GET", "/tasks/"+taskID, nil)
	statusRR := httptest.NewRecorder()
	r.ServeHTTP(statusRR, statusReq)
	var statusResp StatusResponse
	json.NewDecoder(statusRR.Body).Decode(&statusResp)
	
	if statusResp.Status != "running" {
		t.Logf("Задача в статусе '%s', а не 'running'", statusResp.Status)
	}

	// Удаляем задачу
	deleteReq, _ := http.NewRequest("DELETE", "/tasks/"+taskID, nil)
	deleteRR := httptest.NewRecorder()
	r.ServeHTTP(deleteRR, deleteReq)
	if deleteRR.Code != http.StatusNoContent {
		t.Errorf("Ожидался статус 204, получен %v", deleteRR.Code)
	}

	// Ждем немного, чтобы отмена обработалась
	time.Sleep(1 * time.Second)

	// Проверяем, что задача отменена или удалена
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	taskCancelledOrDeleted := false
	for {
		select {
		case <-ctx.Done():
			if !taskCancelledOrDeleted {
				t.Error("Задача не была отменена или удалена в течение 10 секунд")
			}
			return
		default:
			statusReq, _ := http.NewRequest("GET", "/tasks/"+taskID, nil)
			statusRR := httptest.NewRecorder()
			r.ServeHTTP(statusRR, statusReq)
			
			// Если задача не найдена, значит она была отменена и удалена
			if statusRR.Code == http.StatusNotFound {
				t.Log("Задача была отменена и удалена")
				taskCancelledOrDeleted = true
				return
			}
			
			var statusResp StatusResponse
			json.NewDecoder(statusRR.Body).Decode(&statusResp)
			
			if statusResp.Status == "cancelled" {
				if statusResp.Result != "Task cancelled" {
					t.Errorf("Ожидался результат 'Task cancelled', получен '%s'", statusResp.Result)
				}
				taskCancelledOrDeleted = true
				return
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
}

// TestGetNonExistentTask проверяет получение несуществующей задачи
func TestGetNonExistentTask(t *testing.T) {
	r := setupRouter()
	req, _ := http.NewRequest("GET", "/tasks/nonexistent", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("Ожидался статус 404, получен %v", rr.Code)
	}
}

// TestDeleteNonExistentTask проверяет удаление несуществующей задачи
func TestDeleteNonExistentTask(t *testing.T) {
	r := setupRouter()
	req, _ := http.NewRequest("DELETE", "/tasks/nonexistent", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("Ожидался статус 404, получен %v", rr.Code)
	}
}

// TestConcurrentTaskCreation проверяет конкурентное создание задач
func TestConcurrentTaskCreation(t *testing.T) {
	r := setupRouter()
	var taskIDs []string
	var mu sync.Mutex
	const numTasks = 50

	// WaitGroup для ожидания завершения всех горутин
	var wg sync.WaitGroup
	wg.Add(numTasks)

	// Создаем задачи параллельно
	for i := 0; i < numTasks; i++ {
		go func(index int) {
			defer wg.Done()
			
			req, _ := http.NewRequest("POST", "/tasks", nil)
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)
			
			if rr.Code != http.StatusOK {
				t.Errorf("Задача %d: ожидался статус 200, получен %v", index, rr.Code)
				return
			}
			
			var taskResp TaskResponse
			if err := json.NewDecoder(rr.Body).Decode(&taskResp); err != nil {
				t.Errorf("Задача %d: ошибка декодирования ответа: %v", index, err)
				return
			}
			
			if taskResp.ID == "" {
				t.Errorf("Задача %d: ожидался непустой ID задачи", index)
				return
			}
			if taskResp.Status != "pending" && taskResp.Status != "running" {
				t.Errorf("Задача %d: ожидался статус 'pending' или 'running', получен '%s'", index, taskResp.Status)
				return
			}
			
			mu.Lock()
			taskIDs = append(taskIDs, taskResp.ID)
			mu.Unlock()
			
			t.Logf("Создано задание %d с ID: %s, статус: %s", index, taskResp.ID, taskResp.Status)
		}(i)
	}

	// Ждем завершения всех горутин
	wg.Wait()

	// Проверяем, что все задачи созданы
	if len(taskIDs) != numTasks {
		t.Errorf("Ожидалось %d задач, создано %d", numTasks, len(taskIDs))
	}

	// Проверяем уникальность ID
	idMap := make(map[string]bool)
	for _, id := range taskIDs {
		if idMap[id] {
			t.Errorf("Найден дублирующийся ID: %s", id)
		}
		idMap[id] = true
	}
}