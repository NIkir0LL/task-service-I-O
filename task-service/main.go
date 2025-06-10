package main

import (
	"log"
	"net/http"

	"task-service/handler"
	"task-service/task"

	"github.com/gorilla/mux"
)

func main() {
	manager := task.NewManager()
	taskHandler := &handler.TaskHandler{Manager: manager}

	r := mux.NewRouter()
	r.HandleFunc("/tasks", taskHandler.CreateTask).Methods("POST")
	r.HandleFunc("/tasks/{id}", taskHandler.GetTask).Methods("GET")
	r.HandleFunc("/tasks/{id}", taskHandler.DeleteTask).Methods("DELETE")

	log.Println("Server running on :8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}