package services

import (
	"encoding/json"
	"fmt"

	"taskpilot/internal/ai"
	"taskpilot/internal/core"
	"taskpilot/internal/model"
)

// ChatResponse is returned from ChatWithAI to the frontend.
type ChatResponse struct {
	Text      string           `json:"text"`
	ToolCalls []ToolCallResult `json:"toolCalls"`
}

// ToolCallResult represents the outcome of a single AI tool call.
type ToolCallResult struct {
	Action  string `json:"action"`
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// AIService handles AI chat and analysis features.
type AIService struct {
	Core        *core.AppCore
	aiClient    *ai.ClaudeClient
	chatHistory []ai.ChatMessage
}

// ReloadClient re-initialises the AI client from stored config.
func (s *AIService) ReloadClient() {
	apiKey, _ := s.Core.ConfigStore.Get("api_key")
	baseURL, _ := s.Core.ConfigStore.Get("api_base_url")
	modelName, _ := s.Core.ConfigStore.Get("api_model")
	if apiKey != "" {
		s.aiClient = ai.NewClaudeClient(apiKey, baseURL, modelName)
	}
}

func (s *AIService) ChatWithAI(message string) (*ChatResponse, error) {
	if s.aiClient == nil {
		return nil, fmt.Errorf("AI 未配置 – 请先在设置中配置 API Key")
	}

	tasks, err := s.Core.TaskStore.ListAll()
	if err != nil {
		tasks = []model.Task{}
	}
	taskJSON, _ := json.Marshal(tasks)

	s.chatHistory = append(s.chatHistory, ai.ChatMessage{
		Role:    "user",
		Content: message,
	})

	text, toolCalls, err := s.aiClient.Chat(s.chatHistory, string(taskJSON))
	if err != nil {
		return nil, fmt.Errorf("AI 对话失败: %w", err)
	}

	s.chatHistory = append(s.chatHistory, ai.ChatMessage{
		Role:    "assistant",
		Content: text,
	})

	var toolResults []ToolCallResult
	for _, tc := range toolCalls {
		toolResults = append(toolResults, s.executeToolCall(tc))
	}

	return &ChatResponse{
		Text:      text,
		ToolCalls: toolResults,
	}, nil
}

func (s *AIService) executeToolCall(tc ai.ToolCall) ToolCallResult {
	getStr := func(key string) string {
		if v, ok := tc.Input[key]; ok {
			if str, ok := v.(string); ok {
				return str
			}
		}
		return ""
	}
	getInt := func(key string) int {
		if v, ok := tc.Input[key]; ok {
			if f, ok := v.(float64); ok {
				return int(f)
			}
		}
		return 0
	}

	switch tc.Name {
	case "create_task":
		title := getStr("title")
		err := s.Core.TaskStore.Create(model.Task{
			Title:     title,
			ProjectID: getStr("projectId"),
			Priority:  getInt("priority"),
			DueDate:   getStr("dueDate"),
			Status:    "todo",
		})
		if err != nil {
			return ToolCallResult{Action: tc.Name, Success: false, Message: err.Error()}
		}
		return ToolCallResult{Action: tc.Name, Success: true, Message: fmt.Sprintf("任务 '%s' 已创建", title)}

	case "update_task":
		id := getStr("id")
		existing, err := s.Core.TaskStore.GetByID(id)
		if err != nil {
			return ToolCallResult{Action: tc.Name, Success: false, Message: err.Error()}
		}
		if str := getStr("title"); str != "" {
			existing.Title = str
		}
		if str := getStr("status"); str != "" {
			existing.Status = str
		}
		if _, ok := tc.Input["priority"]; ok {
			existing.Priority = getInt("priority")
		}
		if str := getStr("dueDate"); str != "" {
			existing.DueDate = str
		}
		if err := s.Core.TaskStore.Update(*existing); err != nil {
			return ToolCallResult{Action: tc.Name, Success: false, Message: err.Error()}
		}
		return ToolCallResult{Action: tc.Name, Success: true, Message: fmt.Sprintf("任务 '%s' 已更新", id)}

	case "delete_task":
		id := getStr("id")
		if err := s.Core.TaskStore.Delete(id); err != nil {
			return ToolCallResult{Action: tc.Name, Success: false, Message: err.Error()}
		}
		return ToolCallResult{Action: tc.Name, Success: true, Message: fmt.Sprintf("任务 '%s' 已删除", id)}

	case "list_tasks":
		var tasks []model.Task
		var err error
		if pid := getStr("projectId"); pid != "" {
			tasks, err = s.Core.TaskStore.ListByProject(pid)
		} else if status := getStr("status"); status != "" {
			tasks, err = s.Core.TaskStore.ListByStatus(status)
		} else {
			tasks, err = s.Core.TaskStore.ListAll()
		}
		if err != nil {
			return ToolCallResult{Action: tc.Name, Success: false, Message: err.Error()}
		}
		return ToolCallResult{Action: tc.Name, Success: true, Message: fmt.Sprintf("找到 %d 个任务", len(tasks))}

	default:
		return ToolCallResult{Action: tc.Name, Success: false, Message: fmt.Sprintf("unknown action: %s", tc.Name)}
	}
}

func (s *AIService) GetDailySummary() (string, error) {
	if s.aiClient == nil {
		return "", fmt.Errorf("AI 未配置 – 请先在设置中配置 API Key")
	}
	tasks, err := s.Core.TaskStore.ListTodayTasks()
	if err != nil {
		return "", fmt.Errorf("could not fetch today's tasks: %w", err)
	}
	return s.aiClient.GenerateDailySummary(tasksToMaps(tasks))
}

func (s *AIService) SmartSuggestTasks(projectId string) (string, error) {
	if s.aiClient == nil {
		return "", fmt.Errorf("AI 未配置 – 请先在设置中配置 API Key")
	}
	tasks, err := s.Core.TaskStore.ListByProject(projectId)
	if err != nil {
		return "", err
	}
	projects, err := s.Core.ProjectStore.List()
	if err != nil {
		return "", err
	}
	projectName := "未知项目"
	for _, p := range projects {
		if p.ID == projectId {
			projectName = p.Name
			break
		}
	}
	return s.aiClient.SmartSuggest(tasksToMaps(tasks), projectName)
}

func (s *AIService) DecomposeTask(taskId string) (string, error) {
	if s.aiClient == nil {
		return "", fmt.Errorf("AI 未配置 – 请先在设置中配置 API Key")
	}
	task, err := s.Core.TaskStore.GetByID(taskId)
	if err != nil {
		return "", err
	}
	allTasks, _ := s.Core.TaskStore.ListByProject(task.ProjectID)
	return s.aiClient.DecomposeTask(task.Title, task.Description, tasksToMaps(allTasks))
}

func (s *AIService) PrioritizeTasks(projectId string) (string, error) {
	if s.aiClient == nil {
		return "", fmt.Errorf("AI 未配置 – 请先在设置中配置 API Key")
	}
	var tasks []model.Task
	var err error
	if projectId != "" {
		tasks, err = s.Core.TaskStore.ListByProject(projectId)
	} else {
		tasks, err = s.Core.TaskStore.ListAll()
	}
	if err != nil {
		return "", err
	}
	return s.aiClient.PrioritizeTasks(tasksToMaps(tasks))
}

func (s *AIService) GenerateWeeklyReport() (string, error) {
	if s.aiClient == nil {
		return "", fmt.Errorf("AI 未配置 – 请先在设置中配置 API Key")
	}
	tasks, err := s.Core.TaskStore.ListAll()
	if err != nil {
		return "", err
	}
	return s.aiClient.GenerateWeeklyReport(tasksToMaps(tasks))
}

func (s *AIService) TestAIConnection() error {
	if s.aiClient == nil {
		return fmt.Errorf("AI 未配置")
	}
	return s.aiClient.TestConnection()
}

func (s *AIService) ClearChatHistory() {
	s.chatHistory = nil
}

func tasksToMaps(tasks []model.Task) []map[string]interface{} {
	var result []map[string]interface{}
	for _, t := range tasks {
		result = append(result, map[string]interface{}{
			"id": t.ID, "title": t.Title, "status": t.Status,
			"priority": t.Priority, "dueDate": t.DueDate,
			"projectId": t.ProjectID, "description": t.Description,
		})
	}
	return result
}
