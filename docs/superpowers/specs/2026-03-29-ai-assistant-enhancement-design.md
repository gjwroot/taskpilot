# AI 助手增强设计文档

> 日期：2026-03-29
> 状态：待实现

## 概述

对 TaskPilot 的 AI 助手进行全面升级：引入真实流式输出、Streamdown Markdown 渲染、打字机效果，并新增自然语言创建任务、对话持久化、AI 主动建议、智能自动标签四项功能。

## 1. 流式输出架构

### 1.1 传输通道

采用 Wails v3 Events 作为流式通道，Go 后端调用 Claude Streaming API（SSE），逐 token 通过事件推送给前端。

```
Claude API (SSE) → Go AIService (goroutine) → Wails Events → React useAIStream hook → Streamdown
```

### 1.2 事件协议

| 事件名 | 数据结构 | 说明 |
|--------|----------|------|
| `ai:stream:start` | `{ messageId: string }` | 开始新回复 |
| `ai:stream:chunk` | `{ messageId: string, text: string }` | 文本片段 |
| `ai:stream:tool_call` | `{ messageId: string, name: string, input: object }` | AI 发起工具调用 |
| `ai:stream:tool_result` | `{ messageId: string, name: string, result: object, success: bool }` | 工具执行结果 |
| `ai:stream:end` | `{ messageId: string }` | 回复完成 |
| `ai:stream:error` | `{ messageId: string, error: string }` | 出错 |

### 1.3 后端改动

**`internal/ai/claude.go`：**
- 新增 `ChatStream(messages []Message, taskContext string, onEvent func(event StreamEvent))` 方法
- 使用 Claude Messages API 的 `stream: true` 参数
- 解析 SSE 事件（`message_start`、`content_block_delta`、`content_block_stop`、`message_stop`）
- 遇到 `tool_use` 类型的 content block 时，通过 `onEvent` 回调通知调用方

**`services/ai_service.go`：**
- 新增 `StreamChatWithAI(message string)` 方法
- 启动 goroutine 处理流式响应
- 收到文本 delta 时 emit `ai:stream:chunk`
- 收到 tool_use 时暂停流、执行工具（复用现有 `executeToolCall`）、emit `ai:stream:tool_result`、继续流
- 保留 `ChatWithAI()` 作为降级方案

### 1.4 前端改动

**新增 `hooks/useAIStream.ts`：**

```typescript
type StreamStatus = 'idle' | 'streaming' | 'tool_calling' | 'error';

interface UseAIStreamReturn {
  status: StreamStatus;
  streamingContent: string;       // 当前流式消息的累积内容
  sendMessage: (msg: string) => Promise<void>;
  cancel: () => void;             // 取消当前流
}
```

- 监听 `ai:stream:*` 系列事件
- 逐 chunk 拼接 `streamingContent`
- 流结束后将完整消息追加到 `chatMessages`

**`ChatPanel.tsx` 改动：**
- 替换 `await chatWithAI(message)` 为 `useAIStream` 的 `sendMessage`
- 流式消息实时渲染，历史消息静态渲染
- 工具调用期间显示执行状态指示

## 2. Streamdown Markdown 渲染

### 2.1 依赖

```json
{
  "streamdown": "latest",
  "@streamdown/code": "latest",
  "@streamdown/cjk": "latest"
}
```

不安装 `@streamdown/math` 和 `@streamdown/mermaid`（任务管理场景不需要）。

### 2.2 替换范围

| 文件 | 现有方案 | 替换为 |
|------|----------|--------|
| `ChatPanel.tsx` | `renderMarkdown()` + `renderInline()` | `<Streamdown>` 组件 |
| `DailySummary.tsx` | 手写 Markdown 解析 | `<Streamdown isAnimating={false}>` |

### 2.3 组件封装

新建 `components/MarkdownRenderer.tsx`：

```tsx
import { Streamdown } from "streamdown";
import { code, cjk } from "@streamdown/code";

interface Props {
  content: string;
  isStreaming?: boolean;
}

export function MarkdownRenderer({ content, isStreaming = false }: Props) {
  return (
    <Streamdown
      plugins={{ code, cjk }}
      isAnimating={isStreaming}
    >
      {content}
    </Streamdown>
  );
}
```

### 2.4 样式适配

- Streamdown 内置 Tailwind 样式，与现有 Tailwind v4 兼容
- 在 `style.css` 中微调 Streamdown CSS 变量，匹配 TaskPilot 暗色主题
- 代码块使用 Shiki 的 `one-dark-pro` 主题

## 3. 自然语言快速创建任务

### 3.1 方案

增强现有 Claude 系统提示词，添加明确指令：当用户输入看起来像任务描述时，主动提取结构化信息并调用 `create_task`。

### 3.2 系统提示词增强

在现有系统提示词中追加：

```
当用户的消息看起来是在描述一个任务时（如"明天下午3点前完成设计稿"），你应该：
1. 提取任务标题、截止日期（转为 ISO 8601）、优先级（P0-P3）、所属项目
2. 如果信息不完整，使用合理默认值（优先级默认 P1，项目使用当前选中项目）
3. 调用 create_task 工具创建任务
4. 回复确认创建结果，包含解析出的各字段
```

### 3.3 新增工具

无需新增工具。现有 `create_task` 已支持所有字段，只需通过提示词引导 AI 更积极地使用它。

## 4. 对话记录持久化

### 4.1 数据库

**新增 `chat_messages` 表：**

```sql
CREATE TABLE IF NOT EXISTS chat_messages (
    id TEXT PRIMARY KEY,
    project_id TEXT,
    role TEXT NOT NULL,          -- 'user' | 'assistant'
    content TEXT NOT NULL,
    tool_results TEXT,           -- JSON 数组，可为空
    created_at TEXT NOT NULL
);

CREATE INDEX idx_chat_messages_project ON chat_messages(project_id);
CREATE INDEX idx_chat_messages_created ON chat_messages(created_at);
```

### 4.2 后端

**新增 `internal/store/chat_store.go`：**
- `SaveMessage(msg ChatMessage) error`
- `GetMessages(projectID string, limit, offset int) ([]ChatMessage, error)`
- `DeleteMessages(projectID string) error` — 清空某项目对话
- `DeleteAllMessages() error` — 清空全部

**`services/ai_service.go` 改动：**
- `StreamChatWithAI` 完成后自动持久化用户消息和 AI 回复
- 新增 `GetChatHistory(projectID string, limit, offset int)` RPC 方法
- 新增 `ClearChatHistory(projectID string)` RPC 方法

### 4.3 前端

- `appStore.ts` 中 `chatMessages` 初始化时从后端加载历史
- 切换项目时重新加载对应项目的对话历史
- ChatPanel 添加"清空对话"按钮（已有，改为调用后端 RPC）
- 支持滚动到顶部时加载更多历史消息

## 5. AI 主动建议

### 5.1 后端

**`internal/ai/claude.go` 新增：**

```go
func (c *Client) GetProactiveSuggestions(tasks []model.Task, projectName string) (string, error)
```

分析维度：
- 逾期未完成的任务
- 今日到期的任务
- 超过 3 天未更新的进行中任务
- 可能的优先级调整建议

返回 Markdown 格式的简短建议（3-5 条）。

**`services/ai_service.go` 新增：**
- `GetProactiveSuggestions(projectID string)` — 暴露为 RPC

### 5.2 前端

**触发时机：**
- 应用启动后首次加载完成
- 切换项目时

**展示方式：**
- ChatPanel 顶部显示一个可折叠的建议卡片
- 使用 Motion 动画淡入
- 每条建议可点击，点击后作为用户消息发送给 AI 进一步讨论

### 5.3 频率控制

- 同一项目 30 分钟内不重复请求
- 前端缓存建议内容，切换回已缓存的项目时直接显示

## 6. 智能自动标签

### 6.1 数据库

**Task 模型扩展：**

```go
type Task struct {
    // ... 现有字段
    Tags string  // 逗号分隔，如 "设计,前端,紧急"
}
```

**SQLite 迁移：**

```sql
ALTER TABLE tasks ADD COLUMN tags TEXT DEFAULT '';
```

### 6.2 后端

**`internal/ai/claude.go` 新增：**

```go
func (c *Client) AutoTagTask(title, description string, existingTags []string) ([]string, error)
```

- 输入任务标题和描述，以及项目中已有的标签列表（保持一致性）
- 返回 1-3 个标签
- 使用较短的提示词和低 max_tokens 控制成本

**调用时机：**
- 任务创建时（包括 AI 工具调用创建和手动创建）
- 任务标题/描述更新时

**`services/task_service.go` 改动：**
- `CreateTask` 和 `UpdateTask` 中增加异步标签调用（不阻塞主流程）
- 标签生成完成后通过 Wails Event `task:tags:updated` 通知前端刷新

### 6.3 前端

**`TaskItem.tsx` 改动：**
- 在任务卡片中展示彩色标签（根据标签内容 hash 生成颜色）
- 标签可点击筛选同标签任务

**`TaskForm.tsx` 改动：**
- 支持手动编辑标签（输入 + 回车添加）
- 显示 AI 建议的标签，用户可确认或移除

## 7. 文件变更清单

### 后端（Go）

| 文件 | 操作 | 说明 |
|------|------|------|
| `internal/ai/claude.go` | 修改 | 新增 ChatStream、GetProactiveSuggestions、AutoTagTask |
| `services/ai_service.go` | 修改 | 新增 StreamChatWithAI、GetProactiveSuggestions RPC |
| `services/task_service.go` | 修改 | 创建/更新任务时触发自动标签 |
| `internal/store/chat_store.go` | 新增 | 对话持久化存储 |
| `internal/store/db.go` | 修改 | 新增 chat_messages 表迁移、tasks 表 tags 列迁移 |
| `internal/model/task.go` | 修改 | Task 结构体新增 Tags 字段 |
| `internal/core/core.go` | 修改 | 初始化 ChatStore |

### 前端（React/TypeScript）

| 文件 | 操作 | 说明 |
|------|------|------|
| `frontend/package.json` | 修改 | 新增 streamdown 依赖 |
| `frontend/src/hooks/useAIStream.ts` | 新增 | 流式消息 hook |
| `frontend/src/components/MarkdownRenderer.tsx` | 新增 | Streamdown 封装 |
| `frontend/src/components/ChatPanel.tsx` | 修改 | 流式渲染、移除手写 Markdown、主动建议卡片 |
| `frontend/src/components/DailySummary.tsx` | 修改 | 替换为 MarkdownRenderer |
| `frontend/src/components/TaskItem.tsx` | 修改 | 展示标签 |
| `frontend/src/components/TaskForm.tsx` | 修改 | 标签编辑 |
| `frontend/src/stores/appStore.ts` | 修改 | 对话持久化加载、建议缓存 |
| `frontend/src/style.css` | 修改 | Streamdown 主题适配 |

## 8. 不做的事

- 不引入 `@streamdown/math` 和 `@streamdown/mermaid`（不需要）
- 不实现语音输入
- 不实现多种 AI 人格/语气切换
- 不实现番茄钟功能
- 不实现对话导出
- 不实现任务统计仪表盘
- 不修改多窗口架构
- 不更换状态管理方案
