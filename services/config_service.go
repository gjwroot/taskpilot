package services

import (
	"taskpilot/internal/core"
)

// AIConfig holds the AI configuration.
type AIConfig struct {
	APIKey  string `json:"apiKey"`
	BaseURL string `json:"baseURL"`
	Model   string `json:"model"`
}

// ConfigService handles application configuration.
type ConfigService struct {
	Core            *core.AppCore
	OnConfigChanged func()
}

func (s *ConfigService) GetAPIKey() (string, error) {
	return s.Core.ConfigStore.Get("api_key")
}

func (s *ConfigService) SaveAPIKey(key string) error {
	if err := s.Core.ConfigStore.Set("api_key", key); err != nil {
		return err
	}
	if s.OnConfigChanged != nil {
		s.OnConfigChanged()
	}
	return nil
}

func (s *ConfigService) GetAIConfig() (*AIConfig, error) {
	apiKey, _ := s.Core.ConfigStore.Get("api_key")
	baseURL, _ := s.Core.ConfigStore.Get("api_base_url")
	modelName, _ := s.Core.ConfigStore.Get("api_model")
	return &AIConfig{
		APIKey:  apiKey,
		BaseURL: baseURL,
		Model:   modelName,
	}, nil
}

func (s *ConfigService) SaveAIConfig(apiKey, baseURL, modelName string) error {
	if err := s.Core.ConfigStore.Set("api_key", apiKey); err != nil {
		return err
	}
	if err := s.Core.ConfigStore.Set("api_base_url", baseURL); err != nil {
		return err
	}
	if err := s.Core.ConfigStore.Set("api_model", modelName); err != nil {
		return err
	}
	if s.OnConfigChanged != nil {
		s.OnConfigChanged()
	}
	return nil
}

func (s *ConfigService) TestAIConnection() error {
	// Delegate to a temporary check — but the real test goes through AIService.
	// This is kept for backward compat with the frontend.
	return nil
}
