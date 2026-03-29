package services

import (
	"fmt"

	"taskpilot/internal/core"
	"taskpilot/internal/model"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// TaskService handles task CRUD operations.
type TaskService struct {
	Core *core.AppCore
}

func (s *TaskService) CreateTask(title, projectId, description string, priority int, dueDate string) (*model.Task, error) {
	t := model.Task{
		Title:       title,
		ProjectID:   projectId,
		Description: description,
		Priority:    priority,
		DueDate:     dueDate,
		Status:      "todo",
	}
	if err := s.Core.TaskStore.Create(t); err != nil {
		return nil, err
	}
	tasks, err := s.Core.TaskStore.ListAll()
	if err != nil {
		return nil, err
	}
	for i := len(tasks) - 1; i >= 0; i-- {
		if tasks[i].Title == title && tasks[i].ProjectID == projectId {
			result := tasks[i]
			s.emitChange()
			return &result, nil
		}
	}
	return nil, fmt.Errorf("task created but could not be retrieved")
}

func (s *TaskService) UpdateTask(id, title, projectId, description, status string, priority int, dueDate string) error {
	err := s.Core.TaskStore.Update(model.Task{
		ID:          id,
		Title:       title,
		ProjectID:   projectId,
		Description: description,
		Status:      status,
		Priority:    priority,
		DueDate:     dueDate,
	})
	if err == nil {
		s.emitChange()
	}
	return err
}

func (s *TaskService) DeleteTask(id string) error {
	err := s.Core.TaskStore.Delete(id)
	if err == nil {
		s.emitChange()
	}
	return err
}

func (s *TaskService) GetTasksByProject(projectId string) ([]model.Task, error) {
	return s.Core.TaskStore.ListByProject(projectId)
}

func (s *TaskService) GetTodayTasks() ([]model.Task, error) {
	return s.Core.TaskStore.ListTodayTasks()
}

func (s *TaskService) GetAllTasks() ([]model.Task, error) {
	return s.Core.TaskStore.ListAll()
}

func (s *TaskService) emitChange() {
	app := application.Get()
	if app != nil {
		app.Event.Emit("task:changed", nil)
	}
}
