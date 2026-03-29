package services

import (
	"encoding/json"
	"fmt"

	"github.com/wailsapp/wails/v3/pkg/application"
	"taskpilot/internal/ai"
	"taskpilot/internal/core"
	"taskpilot/internal/logger"
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
		logger.Log.Info("AI client reloaded", "model", modelName, "baseURL", baseURL)
	}
}

// GetAIClient returns the underlying Claude client.
func (s *AIService) GetAIClient() *ai.ClaudeClient {
	return s.aiClient
}

// StreamChatWithAI starts a streaming chat session and emits Wails events for each chunk.
func (s *AIService) StreamChatWithAI(message string, projectID string) error {
	if s.aiClient == nil {
		return fmt.Errorf("AI 未配置 – 请先在设置中配置 API Key")
	}

	logger.Log.Info("stream chat request", "messageLen", len(message), "projectID", projectID)

	tasks, err := s.Core.TaskStore.ListAll()
	if err != nil {
		tasks = []model.Task{}
	}
	taskJSON, _ := json.Marshal(tasks)

	s.chatHistory = append(s.chatHistory, ai.ChatMessage{
		Role:    "user",
		Content: message,
	})

	// Persist user message
	s.Core.ChatStore.Save(projectID, "user", message, "[]")

	app := application.Get()
	if app == nil {
		return fmt.Errorf("application not available")
	}

	go func() {
		var allToolResults []ToolCallResult
		const maxToolRounds = 5

		for round := 0; round < maxToolRounds; round++ {
			text, toolCalls, err := s.aiClient.ChatStream(s.chatHistory, string(taskJSON), func(evt ai.StreamEvent) {
				switch evt.Type {
				case ai.StreamEventStart:
					app.Event.Emit("ai:stream:start", map[string]string{"messageId": evt.MessageID})
				case ai.StreamEventChunk:
					app.Event.Emit("ai:stream:chunk", map[string]string{"messageId": evt.MessageID, "text": evt.Text})
				case ai.StreamEventToolCall:
					app.Event.Emit("ai:stream:tool_call", map[string]interface{}{"messageId": evt.MessageID, "name": evt.ToolName, "input": evt.ToolInput})
				case ai.StreamEventEnd:
					// Will be emitted after all rounds complete
				case ai.StreamEventError:
					app.Event.Emit("ai:stream:error", map[string]string{"messageId": evt.MessageID, "error": evt.Text})
				}
			})

			if err != nil {
				logger.Log.Error("stream chat failed", "error", err)
				app.Event.Emit("ai:stream:error", map[string]string{"messageId": "", "error": fmt.Sprintf("AI 对话失败: %v", err)})
				return
			}

			// 如果没有工具调用，这是最终回复
			if len(toolCalls) == 0 {
				s.chatHistory = append(s.chatHistory, ai.ChatMessage{Role: "assistant", Content: text})
				break
			}

			// 构建 assistant 消息的 content blocks（text + tool_use blocks）
			var assistantBlocks []ai.ContentBlock
			if text != "" {
				assistantBlocks = append(assistantBlocks, ai.ContentBlock{Type: "text", Text: text})
			}
			for _, tc := range toolCalls {
				assistantBlocks = append(assistantBlocks, ai.ContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Name,
					Input: tc.Input,
				})
			}
			s.chatHistory = append(s.chatHistory, ai.ChatMessage{
				Role:          "assistant",
				ContentBlocks: assistantBlocks,
			})

			// 执行工具调用并构建 tool_result blocks
			var toolResultBlocks []ai.ContentBlock
			for _, tc := range toolCalls {
				result := s.executeToolCall(tc)
				allToolResults = append(allToolResults, result)
				app.Event.Emit("ai:stream:tool_result", map[string]interface{}{
					"messageId": "", "name": tc.Name, "result": result, "success": result.Success,
				})

				resultJSON, _ := json.Marshal(result)
				toolResultBlocks = append(toolResultBlocks, ai.ContentBlock{
					Type:      "tool_result",
					ToolUseID: tc.ID,
					Content:   string(resultJSON),
				})
			}
			s.chatHistory = append(s.chatHistory, ai.ChatMessage{
				Role:          "user",
				ContentBlocks: toolResultBlocks,
			})

			logger.Log.Info("tool use round completed", "round", round+1, "toolCalls", len(toolCalls))
		}

		// 保存最终的 assistant 文本回复
		finalText := ""
		if len(s.chatHistory) > 0 {
			last := s.chatHistory[len(s.chatHistory)-1]
			if last.Role == "assistant" {
				finalText = last.Content
				if finalText == "" {
					for _, b := range last.ContentBlocks {
						if b.Type == "text" {
							finalText += b.Text
						}
					}
				}
			}
		}

		toolResultsJSON := "[]"
		if len(allToolResults) > 0 {
			b, _ := json.Marshal(allToolResults)
			toolResultsJSON = string(b)
		}
		s.Core.ChatStore.Save(projectID, "assistant", finalText, toolResultsJSON)

		app.Event.Emit("ai:stream:end", map[string]string{"messageId": ""})

		logger.Log.Info("stream chat completed", "textLen", len(finalText), "toolCalls", len(allToolResults))
	}()

	return nil
}

// GetChatHistory retrieves persisted chat messages for a project.
func (s *AIService) GetChatHistory(projectID string, limit, offset int) ([]map[string]interface{}, error) {
	msgs, err := s.Core.ChatStore.GetMessages(projectID, limit, offset)
	if err != nil {
		return nil, err
	}
	var result []map[string]interface{}
	for _, m := range msgs {
		msg := map[string]interface{}{
			"id":        m.ID,
			"role":      m.Role,
			"content":   m.Content,
			"createdAt": m.CreatedAt,
		}
		if m.ToolResults != "" && m.ToolResults != "[]" {
			var tr []ToolCallResult
			if json.Unmarshal([]byte(m.ToolResults), &tr) == nil && len(tr) > 0 {
				msg["toolResults"] = tr
			}
		}
		result = append(result, msg)
	}
	return result, nil
}

// ClearProjectChatHistory clears in-memory and DB chat history for a specific project.
func (s *AIService) ClearProjectChatHistory(projectID string) error {
	s.chatHistory = nil
	return s.Core.ChatStore.DeleteByProject(projectID)
}

// GetProactiveSuggestions returns AI-generated suggestions for the given project.
func (s *AIService) GetProactiveSuggestions(projectID string) (string, error) {
	if s.aiClient == nil {
		return "", fmt.Errorf("AI 未配置")
	}
	logger.Log.Info("getting proactive suggestions", "projectID", projectID)

	var tasks []model.Task
	var err error
	if projectID != "" {
		tasks, err = s.Core.TaskStore.ListByProject(projectID)
	} else {
		tasks, err = s.Core.TaskStore.ListAll()
	}
	if err != nil {
		return "", err
	}

	projectName := "所有项目"
	if projectID != "" {
		projects, _ := s.Core.ProjectStore.List()
		for _, p := range projects {
			if p.ID == projectID {
				projectName = p.Name
				break
			}
		}
	}

	result, err := s.aiClient.GetProactiveSuggestions(tasksToMaps(tasks), projectName)
	if err != nil {
		logger.Log.Error("proactive suggestions failed", "error", err)
		return "", err
	}
	return result, nil
}

func (s *AIService) ChatWithAI(message string) (*ChatResponse, error) {
	if s.aiClient == nil {
		return nil, fmt.Errorf("AI 未配置 – 请先在设置中配置 API Key")
	}

	logger.Log.Info("chat request", "messageLen", len(message))

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
		logger.Log.Error("chat failed", "error", err)
		return nil, fmt.Errorf("AI 对话失败: %w", err)
	}

	logger.Log.Info("chat response", "textLen", len(text), "toolCalls", len(toolCalls))

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
	logger.Log.Info("executing tool call", "tool", tc.Name)

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

	var result ToolCallResult

	emitTaskChanged := func() {
		app := application.Get()
		if app != nil {
			app.Event.Emit("task:changed", nil)
		}
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
			result = ToolCallResult{Action: tc.Name, Success: false, Message: err.Error()}
		} else {
			emitTaskChanged()
			result = ToolCallResult{Action: tc.Name, Success: true, Message: fmt.Sprintf("任务 '%s' 已创建", title)}
		}

	case "update_task":
		id := getStr("id")
		existing, err := s.Core.TaskStore.GetByID(id)
		if err != nil {
			result = ToolCallResult{Action: tc.Name, Success: false, Message: err.Error()}
		} else {
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
				result = ToolCallResult{Action: tc.Name, Success: false, Message: err.Error()}
			} else {
				emitTaskChanged()
				result = ToolCallResult{Action: tc.Name, Success: true, Message: fmt.Sprintf("任务 '%s' 已更新", id)}
			}
		}

	case "delete_task":
		id := getStr("id")
		if err := s.Core.TaskStore.Delete(id); err != nil {
			result = ToolCallResult{Action: tc.Name, Success: false, Message: err.Error()}
		} else {
			emitTaskChanged()
			result = ToolCallResult{Action: tc.Name, Success: true, Message: fmt.Sprintf("任务 '%s' 已删除", id)}
		}

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
			result = ToolCallResult{Action: tc.Name, Success: false, Message: err.Error()}
		} else {
			summary := fmt.Sprintf("找到 %d 个任务", len(tasks))
			if len(tasks) > 0 {
				taskData, _ := json.Marshal(tasksToMaps(tasks))
				summary += "\n" + string(taskData)
			}
			result = ToolCallResult{Action: tc.Name, Success: true, Message: summary}
		}

	default:
		result = ToolCallResult{Action: tc.Name, Success: false, Message: fmt.Sprintf("unknown action: %s", tc.Name)}
	}

	logger.Log.Info("tool call result", "tool", tc.Name, "success", result.Success, "message", result.Message)
	return result
}

func (s *AIService) GetDailySummary() (string, error) {
	if s.aiClient == nil {
		return "", fmt.Errorf("AI 未配置 – 请先在设置中配置 API Key")
	}
	logger.Log.Info("generating daily summary")
	tasks, err := s.Core.TaskStore.ListTodayTasks()
	if err != nil {
		return "", fmt.Errorf("could not fetch today's tasks: %w", err)
	}
	result, err := s.aiClient.GenerateDailySummary(tasksToMaps(tasks))
	if err != nil {
		logger.Log.Error("daily summary failed", "error", err)
		return "", err
	}
	logger.Log.Info("daily summary generated", "resultLen", len(result))
	return result, nil
}

func (s *AIService) SmartSuggestTasks(projectId string) (string, error) {
	if s.aiClient == nil {
		return "", fmt.Errorf("AI 未配置 – 请先在设置中配置 API Key")
	}
	logger.Log.Info("smart suggest tasks", "projectId", projectId)
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
	result, err := s.aiClient.SmartSuggest(tasksToMaps(tasks), projectName)
	if err != nil {
		logger.Log.Error("smart suggest failed", "error", err)
		return "", err
	}
	logger.Log.Info("smart suggest completed", "projectName", projectName)
	return result, nil
}

func (s *AIService) DecomposeTask(taskId string) (string, error) {
	if s.aiClient == nil {
		return "", fmt.Errorf("AI 未配置 – 请先在设置中配置 API Key")
	}
	logger.Log.Info("decompose task", "taskId", taskId)
	task, err := s.Core.TaskStore.GetByID(taskId)
	if err != nil {
		return "", err
	}
	allTasks, _ := s.Core.TaskStore.ListByProject(task.ProjectID)
	result, err := s.aiClient.DecomposeTask(task.Title, task.Description, tasksToMaps(allTasks))
	if err != nil {
		logger.Log.Error("decompose task failed", "taskId", taskId, "error", err)
		return "", err
	}
	logger.Log.Info("decompose task completed", "taskId", taskId)
	return result, nil
}

func (s *AIService) PrioritizeTasks(projectId string) (string, error) {
	if s.aiClient == nil {
		return "", fmt.Errorf("AI 未配置 – 请先在设置中配置 API Key")
	}
	logger.Log.Info("prioritize tasks", "projectId", projectId)
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
	result, err := s.aiClient.PrioritizeTasks(tasksToMaps(tasks))
	if err != nil {
		logger.Log.Error("prioritize tasks failed", "error", err)
		return "", err
	}
	logger.Log.Info("prioritize tasks completed", "taskCount", len(tasks))
	return result, nil
}

func (s *AIService) GenerateWeeklyReport() (string, error) {
	if s.aiClient == nil {
		return "", fmt.Errorf("AI 未配置 – 请先在设置中配置 API Key")
	}
	logger.Log.Info("generating weekly report")
	tasks, err := s.Core.TaskStore.ListAll()
	if err != nil {
		return "", err
	}
	result, err := s.aiClient.GenerateWeeklyReport(tasksToMaps(tasks))
	if err != nil {
		logger.Log.Error("weekly report failed", "error", err)
		return "", err
	}
	logger.Log.Info("weekly report generated", "resultLen", len(result))
	return result, nil
}

func (s *AIService) TestAIConnection() error {
	if s.aiClient == nil {
		return fmt.Errorf("AI 未配置")
	}
	return s.aiClient.TestConnection()
}

func (s *AIService) ClearChatHistory() {
	s.chatHistory = nil
	s.Core.ChatStore.DeleteAll()
}

func tasksToMaps(tasks []model.Task) []map[string]interface{} {
	var result []map[string]interface{}
	for _, t := range tasks {
		result = append(result, map[string]interface{}{
			"id": t.ID, "title": t.Title, "status": t.Status,
			"priority": t.Priority, "dueDate": t.DueDate,
			"projectId": t.ProjectID, "description": t.Description,
			"tags": t.Tags,
		})
	}
	return result
}
