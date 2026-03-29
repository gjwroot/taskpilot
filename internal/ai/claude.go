package ai

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"taskpilot/internal/logger"
)

const (
	defaultClaudeAPIURL = "https://api.anthropic.com/v1/messages"
	defaultClaudeModel  = "claude-sonnet-4-20250514"
	anthropicVersion    = "2023-06-01"
)

// ContentBlock represents a single block in a multi-block message (text, tool_use, or tool_result).
type ContentBlock struct {
	Type      string                 `json:"type"`                  // "text", "tool_use", "tool_result"
	Text      string                 `json:"text,omitempty"`        // for type="text"
	ID        string                 `json:"id,omitempty"`          // for type="tool_use"
	Name      string                 `json:"name,omitempty"`        // for type="tool_use"
	Input     map[string]interface{} `json:"input,omitempty"`       // for type="tool_use"
	ToolUseID string                 `json:"tool_use_id,omitempty"` // for type="tool_result"
	Content   string                 `json:"content,omitempty"`     // for type="tool_result"
}

// ChatMessage represents a single message in a conversation.
type ChatMessage struct {
	Role          string         `json:"role"`
	Content       string         `json:"content,omitempty"`
	ContentBlocks []ContentBlock `json:"content_blocks,omitempty"`
}

// ToolCall represents a tool invocation requested by the model.
type ToolCall struct {
	ID    string                 `json:"id"`
	Name  string                 `json:"name"`
	Input map[string]interface{} `json:"input"`
}

// ClaudeClient wraps the Claude API.
type ClaudeClient struct {
	apiKey  string
	baseURL string
	model   string
	http    *http.Client
}

// NewClaudeClient creates a new ClaudeClient with the given API key, base URL, and model.
func NewClaudeClient(apiKey, baseURL, model string) *ClaudeClient {
	if baseURL == "" {
		baseURL = defaultClaudeAPIURL
	}
	if model == "" {
		model = defaultClaudeModel
	}
	// Ensure baseURL ends properly for the messages endpoint
	baseURL = strings.TrimRight(baseURL, "/")
	if !strings.HasSuffix(baseURL, "/v1/messages") {
		baseURL = baseURL + "/v1/messages"
	}
	return &ClaudeClient{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		http:    &http.Client{},
	}
}

// SetAPIKey updates the API key.
func (c *ClaudeClient) SetAPIKey(apiKey string) {
	c.apiKey = apiKey
}

// ---------- internal API types ----------

type apiContentBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

type apiMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type apiToolInputSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
	Required   []string               `json:"required,omitempty"`
}

type apiTool struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	InputSchema apiToolInputSchema `json:"input_schema"`
}

type apiRequest struct {
	Model     string       `json:"model"`
	MaxTokens int          `json:"max_tokens"`
	System    string       `json:"system,omitempty"`
	Messages  []apiMessage `json:"messages"`
	Tools     []apiTool    `json:"tools,omitempty"`
}

type apiResponse struct {
	Content    []apiContentBlock `json:"content"`
	StopReason string            `json:"stop_reason"`
}

// ---------- streaming types ----------

type StreamEventType string

const (
	StreamEventStart    StreamEventType = "start"
	StreamEventChunk    StreamEventType = "chunk"
	StreamEventToolCall StreamEventType = "tool_call"
	StreamEventEnd      StreamEventType = "end"
	StreamEventError    StreamEventType = "error"
)

type StreamEvent struct {
	Type      StreamEventType        `json:"type"`
	MessageID string                 `json:"messageId"`
	Text      string                 `json:"text,omitempty"`
	ToolName  string                 `json:"toolName,omitempty"`
	ToolID    string                 `json:"toolId,omitempty"`
	ToolInput map[string]interface{} `json:"toolInput,omitempty"`
}

type apiStreamRequest struct {
	Model     string       `json:"model"`
	MaxTokens int          `json:"max_tokens"`
	System    string       `json:"system,omitempty"`
	Messages  []apiMessage `json:"messages"`
	Tools     []apiTool    `json:"tools,omitempty"`
	Stream    bool         `json:"stream"`
}

type sseMessageStart struct {
	Type    string `json:"type"`
	Message struct {
		ID string `json:"id"`
	} `json:"message"`
}

type sseContentBlockStart struct {
	Type         string `json:"type"`
	Index        int    `json:"index"`
	ContentBlock struct {
		Type string `json:"type"`
		ID   string `json:"id,omitempty"`
		Name string `json:"name,omitempty"`
		Text string `json:"text,omitempty"`
	} `json:"content_block"`
}

type sseContentBlockDelta struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
	Delta struct {
		Type        string `json:"type"`
		Text        string `json:"text,omitempty"`
		PartialJSON string `json:"partial_json,omitempty"`
	} `json:"delta"`
}

// ---------- tool definitions ----------

func chatTools() []apiTool {
	return []apiTool{
		{
			Name:        "create_task",
			Description: "创建一个新任务",
			InputSchema: apiToolInputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"title": map[string]interface{}{
						"type":        "string",
						"description": "任务标题",
					},
					"projectId": map[string]interface{}{
						"type":        "string",
						"description": "所属项目ID",
					},
					"priority": map[string]interface{}{
						"type":        "integer",
						"description": "优先级，0=P0(紧急), 1=P1, 2=P2, 3=P3",
						"minimum":     0,
						"maximum":     3,
					},
					"dueDate": map[string]interface{}{
						"type":        "string",
						"description": "截止日期，ISO格式（可选）",
					},
				},
				Required: []string{"title", "projectId"},
			},
		},
		{
			Name:        "update_task",
			Description: "更新已有任务",
			InputSchema: apiToolInputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "任务ID",
					},
					"title": map[string]interface{}{
						"type":        "string",
						"description": "任务标题",
					},
					"status": map[string]interface{}{
						"type":        "string",
						"description": "任务状态",
						"enum":        []string{"todo", "doing", "done"},
					},
					"priority": map[string]interface{}{
						"type":        "integer",
						"description": "优先级，0-3",
					},
					"dueDate": map[string]interface{}{
						"type":        "string",
						"description": "截止日期，ISO格式",
					},
				},
				Required: []string{"id"},
			},
		},
		{
			Name:        "list_tasks",
			Description: "查询任务列表",
			InputSchema: apiToolInputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"projectId": map[string]interface{}{
						"type":        "string",
						"description": "按项目ID过滤（可选）",
					},
					"status": map[string]interface{}{
						"type":        "string",
						"description": "按状态过滤：todo/doing/done（可选）",
					},
				},
			},
		},
		{
			Name:        "delete_task",
			Description: "删除一个任务",
			InputSchema: apiToolInputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "要删除的任务ID",
					},
				},
				Required: []string{"id"},
			},
		},
	}
}

// ---------- HTTP helper ----------

func (c *ClaudeClient) doRequest(req apiRequest) (*apiResponse, error) {
	logger.Log.Info("AI API request", "model", req.Model, "url", c.baseURL, "messages", len(req.Messages))

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		logger.Log.Error("AI API http error", "error", err)
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		logger.Log.Error("AI API error", "status", resp.StatusCode, "body", string(respBody))
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	logger.Log.Info("AI API response", "status", resp.StatusCode, "bodyLen", len(respBody))

	var apiResp apiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &apiResp, nil
}

func (c *ClaudeClient) extractText(resp *apiResponse) string {
	var result string
	for _, block := range resp.Content {
		if block.Type == "text" {
			result += block.Text
		}
	}
	return result
}

// ---------- public methods ----------

// Chat sends a conversation to Claude and returns the text reply and any tool calls.
func (c *ClaudeClient) Chat(messages []ChatMessage, taskContext string) (string, []ToolCall, error) {
	systemPrompt := "你是 TaskPilot AI 助手，帮助用户管理项目和任务。用户的任务数据会作为上下文提供。你可以使用工具来创建、更新、查询、删除任务。请用中文回复。"
	if taskContext != "" {
		systemPrompt += "\n\n当前任务数据（JSON格式）：\n" + taskContext
	}

	apiMsgs := make([]apiMessage, len(messages))
	for i, m := range messages {
		if len(m.ContentBlocks) > 0 {
			apiMsgs[i] = apiMessage{Role: m.Role, Content: m.ContentBlocks}
		} else {
			apiMsgs[i] = apiMessage{Role: m.Role, Content: m.Content}
		}
	}

	req := apiRequest{
		Model:     c.model,
		MaxTokens: 4096,
		System:    systemPrompt,
		Messages:  apiMsgs,
		Tools:     chatTools(),
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return "", nil, err
	}

	var textContent string
	var toolCalls []ToolCall

	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			textContent += block.Text
		case "tool_use":
			var input map[string]interface{}
			if len(block.Input) > 0 {
				if err := json.Unmarshal(block.Input, &input); err != nil {
					return "", nil, fmt.Errorf("unmarshal tool input: %w", err)
				}
			}
			toolCalls = append(toolCalls, ToolCall{
				ID:    block.ID,
				Name:  block.Name,
				Input: input,
			})
		}
	}

	return textContent, toolCalls, nil
}

// ChatStream sends a conversation to Claude using SSE streaming and calls onEvent for each event.
func (c *ClaudeClient) ChatStream(messages []ChatMessage, taskContext string, onEvent func(StreamEvent)) (string, []ToolCall, error) {
	systemPrompt := `你是 TaskPilot AI 助手，帮助用户管理项目和任务。用户的任务数据会作为上下文提供。你可以使用工具来创建、更新、查询、删除任务。请用中文回复。

当用户的消息看起来是在描述一个任务时（如"明天下午3点前完成设计稿"），你应该：
1. 提取任务标题、截止日期（转为 ISO 8601 格式 YYYY-MM-DD）、优先级（P0-P3 对应 0-3）、所属项目
2. 如果信息不完整，使用合理默认值（优先级默认 1，项目使用上下文中最近活跃的项目）
3. 调用 create_task 工具创建任务
4. 回复确认创建结果，包含解析出的各字段`

	if taskContext != "" {
		systemPrompt += "\n\n当前任务数据（JSON格式）：\n" + taskContext
	}

	apiMsgs := make([]apiMessage, len(messages))
	for i, m := range messages {
		if len(m.ContentBlocks) > 0 {
			apiMsgs[i] = apiMessage{Role: m.Role, Content: m.ContentBlocks}
		} else {
			apiMsgs[i] = apiMessage{Role: m.Role, Content: m.Content}
		}
	}

	req := apiStreamRequest{
		Model:     c.model,
		MaxTokens: 4096,
		System:    systemPrompt,
		Messages:  apiMsgs,
		Tools:     chatTools(),
		Stream:    true,
	}

	logger.Log.Info("AI streaming request", "model", req.Model, "url", c.baseURL, "messages", len(req.Messages))

	body, err := json.Marshal(req)
	if err != nil {
		return "", nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return "", nil, fmt.Errorf("create http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return "", nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		logger.Log.Error("AI streaming error", "status", resp.StatusCode, "body", string(respBody))
		return "", nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	return c.parseSSEStream(resp.Body, onEvent)
}

func (c *ClaudeClient) parseSSEStream(body io.Reader, onEvent func(StreamEvent)) (string, []ToolCall, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var (
		fullText         string
		toolCalls        []ToolCall
		messageID        string
		currentBlockType string
		currentToolID    string
		currentToolName  string
		toolInputJSON    string
	)

	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var raw map[string]interface{}
		if err := json.Unmarshal([]byte(data), &raw); err != nil {
			continue
		}

		eventType, _ := raw["type"].(string)

		switch eventType {
		case "message_start":
			var evt sseMessageStart
			json.Unmarshal([]byte(data), &evt)
			messageID = evt.Message.ID
			onEvent(StreamEvent{
				Type:      StreamEventStart,
				MessageID: messageID,
			})

		case "content_block_start":
			var evt sseContentBlockStart
			json.Unmarshal([]byte(data), &evt)
			currentBlockType = evt.ContentBlock.Type
			if currentBlockType == "tool_use" {
				currentToolID = evt.ContentBlock.ID
				currentToolName = evt.ContentBlock.Name
				toolInputJSON = ""
			}

		case "content_block_delta":
			var evt sseContentBlockDelta
			json.Unmarshal([]byte(data), &evt)

			if evt.Delta.Type == "text_delta" {
				fullText += evt.Delta.Text
				onEvent(StreamEvent{
					Type:      StreamEventChunk,
					MessageID: messageID,
					Text:      evt.Delta.Text,
				})
			} else if evt.Delta.Type == "input_json_delta" {
				toolInputJSON += evt.Delta.PartialJSON
			}

		case "content_block_stop":
			if currentBlockType == "tool_use" {
				var input map[string]interface{}
				if toolInputJSON != "" {
					json.Unmarshal([]byte(toolInputJSON), &input)
				}
				toolCalls = append(toolCalls, ToolCall{
					ID:    currentToolID,
					Name:  currentToolName,
					Input: input,
				})
				onEvent(StreamEvent{
					Type:      StreamEventToolCall,
					MessageID: messageID,
					ToolName:  currentToolName,
					ToolID:    currentToolID,
					ToolInput: input,
				})
			}
			currentBlockType = ""

		case "message_stop":
			onEvent(StreamEvent{
				Type:      StreamEventEnd,
				MessageID: messageID,
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return fullText, toolCalls, fmt.Errorf("read SSE stream: %w", err)
	}

	return fullText, toolCalls, nil
}

// GenerateDailySummary generates a Markdown daily summary from a list of tasks.
func (c *ClaudeClient) GenerateDailySummary(tasks []map[string]interface{}) (string, error) {
	taskJSON, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal tasks: %w", err)
	}

	userContent := "请根据以下任务数据生成一份简洁的每日工作摘要。包括：已完成的工作、进行中的工作、待处理的紧急事项。用中文回复，格式清晰。\n\n任务数据：\n" + string(taskJSON)

	req := apiRequest{
		Model:     c.model,
		MaxTokens: 2048,
		Messages: []apiMessage{
			{Role: "user", Content: userContent},
		},
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return "", err
	}

	return c.extractText(resp), nil
}

// SmartSuggest analyzes existing tasks and suggests new tasks.
func (c *ClaudeClient) SmartSuggest(tasks []map[string]interface{}, projectName string) (string, error) {
	taskJSON, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal tasks: %w", err)
	}

	prompt := fmt.Sprintf(`基于以下项目「%s」的现有任务，分析项目进展并智能推荐 3-5 个可能需要添加的新任务。

对每个建议的任务，请提供：
- **标题**：简洁明确的任务标题
- **优先级**：P0-P3
- **理由**：为什么建议这个任务

请用中文回复，格式清晰。

现有任务数据：
%s`, projectName, string(taskJSON))

	req := apiRequest{
		Model:     c.model,
		MaxTokens: 2048,
		Messages:  []apiMessage{{Role: "user", Content: prompt}},
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return "", err
	}
	return c.extractText(resp), nil
}

// DecomposeTask breaks down a complex task into subtasks.
func (c *ClaudeClient) DecomposeTask(taskTitle, taskDescription string, contextTasks []map[string]interface{}) (string, error) {
	contextJSON, _ := json.MarshalIndent(contextTasks, "", "  ")

	prompt := fmt.Sprintf(`请将以下任务分解为可执行的子任务（3-7 个）。

**任务标题**：%s
**任务描述**：%s

每个子任务需要：
- **标题**：明确可执行的描述
- **优先级**：P0-P3
- **预估时间**：大致时间估算
- **依赖关系**：是否依赖其他子任务

请用中文回复。

项目现有任务上下文：
%s`, taskTitle, taskDescription, string(contextJSON))

	req := apiRequest{
		Model:     c.model,
		MaxTokens: 2048,
		Messages:  []apiMessage{{Role: "user", Content: prompt}},
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return "", err
	}
	return c.extractText(resp), nil
}

// PrioritizeTasks analyzes and suggests priority adjustments.
func (c *ClaudeClient) PrioritizeTasks(tasks []map[string]interface{}) (string, error) {
	taskJSON, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal tasks: %w", err)
	}

	prompt := fmt.Sprintf(`请分析以下任务列表，并给出优先级调整建议。

考虑因素：
1. 截止日期紧迫程度
2. 任务之间的依赖关系
3. 当前进行中任务的数量（建议同时进行不超过 3 个）
4. 优先处理阻塞其他任务的工作

对于每个需要调整的任务，说明：
- 当前优先级 → 建议优先级
- 调整理由

最后给出一个建议的任务执行顺序。请用中文回复。

任务数据：
%s`, string(taskJSON))

	req := apiRequest{
		Model:     c.model,
		MaxTokens: 2048,
		Messages:  []apiMessage{{Role: "user", Content: prompt}},
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return "", err
	}
	return c.extractText(resp), nil
}

// GenerateWeeklyReport generates a weekly progress report.
func (c *ClaudeClient) GenerateWeeklyReport(tasks []map[string]interface{}) (string, error) {
	taskJSON, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal tasks: %w", err)
	}

	prompt := fmt.Sprintf(`请根据以下任务数据生成一份周报。包括：

## 本周完成
列出已完成的任务及成果

## 进行中的工作
列出进行中的任务及当前状态

## 下周计划
根据待办任务和优先级，建议下周的工作重点

## 风险与阻塞
识别可能的风险和阻塞项

## 数据统计
- 完成率
- 各优先级任务分布
- 逾期任务数量

请用中文回复，格式专业简洁。

任务数据：
%s`, string(taskJSON))

	req := apiRequest{
		Model:     c.model,
		MaxTokens: 3000,
		Messages:  []apiMessage{{Role: "user", Content: prompt}},
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return "", err
	}
	return c.extractText(resp), nil
}

// GetProactiveSuggestions analyzes tasks and returns proactive suggestions.
func (c *ClaudeClient) GetProactiveSuggestions(tasks []map[string]interface{}, projectName string) (string, error) {
	taskJSON, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal tasks: %w", err)
	}

	prompt := fmt.Sprintf(`基于项目「%s」的当前任务状态，给出 3-5 条简短的工作建议。

分析维度：
- 逾期未完成的任务（需要立即处理）
- 今日到期的任务
- 超过 3 天未更新的进行中任务（可能被遗忘）
- 优先级调整建议

每条建议用一行，格式：「emoji 建议内容」。简洁有力，不要废话。
如果一切正常，就说"一切顺利"并给出鼓励。

当前任务数据：
%s`, projectName, string(taskJSON))

	req := apiRequest{
		Model:     c.model,
		MaxTokens: 512,
		Messages:  []apiMessage{{Role: "user", Content: prompt}},
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return "", err
	}
	return c.extractText(resp), nil
}

// AutoTagTask uses AI to generate tags for a task.
func (c *ClaudeClient) AutoTagTask(title, description string, existingTags []string) ([]string, error) {
	tagsContext := ""
	if len(existingTags) > 0 {
		tagsJSON, _ := json.Marshal(existingTags)
		tagsContext = fmt.Sprintf("\n项目中已有的标签（尽量复用）：%s", string(tagsJSON))
	}

	prompt := fmt.Sprintf(`为以下任务生成 1-3 个简短的中文标签。
标签应反映任务的类别或领域（如"前端"、"设计"、"修复"、"文档"等）。
仅返回标签，用逗号分隔，不要其他内容。%s

任务标题：%s
任务描述：%s`, tagsContext, title, description)

	req := apiRequest{
		Model:     c.model,
		MaxTokens: 64,
		Messages:  []apiMessage{{Role: "user", Content: prompt}},
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}

	text := strings.TrimSpace(c.extractText(resp))
	if text == "" {
		return nil, nil
	}

	var tags []string
	for _, t := range strings.Split(text, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			tags = append(tags, t)
		}
	}
	return tags, nil
}

// TestConnection tests if the API configuration is working.
func (c *ClaudeClient) TestConnection() error {
	req := apiRequest{
		Model:     c.model,
		MaxTokens: 16,
		Messages: []apiMessage{
			{Role: "user", Content: "Hi"},
		},
	}
	_, err := c.doRequest(req)
	return err
}
