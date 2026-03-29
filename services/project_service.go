package services

import (
	"fmt"

	"taskpilot/internal/core"
	"taskpilot/internal/model"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// ProjectService handles project CRUD operations.
type ProjectService struct {
	Core *core.AppCore
}

func (s *ProjectService) CreateProject(name, description, color string) (*model.Project, error) {
	p := model.Project{
		Name:        name,
		Description: description,
		Color:       color,
	}
	if err := s.Core.ProjectStore.Create(p); err != nil {
		return nil, err
	}
	projects, err := s.Core.ProjectStore.List()
	if err != nil {
		return nil, err
	}
	for i := len(projects) - 1; i >= 0; i-- {
		if projects[i].Name == name && projects[i].Color == color {
			result := projects[i]
			s.emitChange()
			return &result, nil
		}
	}
	return nil, fmt.Errorf("project created but could not be retrieved")
}

func (s *ProjectService) UpdateProject(id, name, description, color string) error {
	err := s.Core.ProjectStore.Update(model.Project{
		ID:          id,
		Name:        name,
		Description: description,
		Color:       color,
	})
	if err == nil {
		s.emitChange()
	}
	return err
}

func (s *ProjectService) DeleteProject(id string) error {
	err := s.Core.ProjectStore.Delete(id)
	if err == nil {
		s.emitChange()
	}
	return err
}

func (s *ProjectService) GetProjects() ([]model.Project, error) {
	return s.Core.ProjectStore.List()
}

func (s *ProjectService) emitChange() {
	app := application.Get()
	if app != nil {
		app.Event.Emit("project:changed", nil)
	}
}
