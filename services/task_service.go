package services

import (
	"fmt"
	"strings"

	"taskpilot/internal/core"
	"taskpilot/internal/logger"
	"taskpilot/internal/model"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// TaskService handles task CRUD operations.
type TaskService struct {
	Core        *core.AppCore
	AutoTagFunc func(title, description string, existingTags []string) ([]string, error)
}

func (s *TaskService) CreateTask(title, projectId, description string, priority int, dueDate string) (*model.Task, error) {
	logger.Log.Info("creating task", "title", title, "projectId", projectId, "priority", priority)
	t := model.Task{
		Title:       title,
		ProjectID:   projectId,
		Description: description,
		Priority:    priority,
		DueDate:     dueDate,
		Status:      "todo",
	}
	if err := s.Core.TaskStore.Create(t); err != nil {
		logger.Log.Error("create task failed", "title", title, "error", err)
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
			go s.autoTag(result.ID, result.Title, result.Description, result.ProjectID)
			return &result, nil
		}
	}
	return nil, fmt.Errorf("task created but could not be retrieved")
}

func (s *TaskService) UpdateTask(id, title, projectId, description, status string, priority int, dueDate string) error {
	logger.Log.Info("updating task", "id", id, "status", status)
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
		go s.autoTag(id, title, description, projectId)
	}
	return err
}

func (s *TaskService) DeleteTask(id string) error {
	logger.Log.Info("deleting task", "id", id)
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

func (s *TaskService) autoTag(taskID, title, description, projectID string) {
	if s.AutoTagFunc == nil {
		return
	}

	tasks, err := s.Core.TaskStore.ListByProject(projectID)
	if err != nil {
		return
	}
	tagSet := make(map[string]bool)
	for _, t := range tasks {
		if t.Tags != "" {
			for _, tag := range strings.Split(t.Tags, ",") {
				tag = strings.TrimSpace(tag)
				if tag != "" {
					tagSet[tag] = true
				}
			}
		}
	}
	var existingTags []string
	for tag := range tagSet {
		existingTags = append(existingTags, tag)
	}

	tags, err := s.AutoTagFunc(title, description, existingTags)
	if err != nil {
		logger.Log.Error("auto tag failed", "taskID", taskID, "error", err)
		return
	}
	if len(tags) == 0 {
		return
	}

	tagsStr := strings.Join(tags, ",")
	logger.Log.Info("auto tag result", "taskID", taskID, "tags", tagsStr)

	task, err := s.Core.TaskStore.GetByID(taskID)
	if err != nil || task == nil {
		return
	}
	task.Tags = tagsStr
	if err := s.Core.TaskStore.Update(*task); err != nil {
		logger.Log.Error("auto tag update failed", "taskID", taskID, "error", err)
		return
	}

	app := application.Get()
	if app != nil {
		app.Event.Emit("task:tags:updated", map[string]string{
			"taskId": taskID,
			"tags":   tagsStr,
		})
		app.Event.Emit("task:changed", nil)
	}
}
