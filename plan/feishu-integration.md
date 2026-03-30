# TaskPilot 飞书集成方案

## Context

TaskPilot 是基于 Go + Wails v3 的桌面任务管理应用，已有 SQLite 数据库、Claude AI 集成（含 tool-call 能力）、事件系统。用户希望将任务系统与飞书打通，支持两种集成：**多维表格双向同步**（Phase 1）和**飞书 Bot 对话管理任务**（Phase 2）。用户已有飞书自建应用（App ID/Secret）。

---

## Phase 1: 飞书基础 + 多维表格双向同步

### 1.1 飞书 API Client (`internal/feishu/`)

**新建文件：**

- `internal/feishu/client.go` — HTTP 客户端，token 自动管理
  - 持有 `appID`, `appSecret`, `tenantAccessToken`, `tokenExpiry`
  - `doRequest(method, path, body)` 自动附加 `Authorization: Bearer <token>`，token 过期自动刷新
  - Token 接口: `POST /open-apis/auth/v3/tenant_access_token/internal/`（2h TTL）
  - Base URL: `https://open.feishu.cn/open-apis`

- `internal/feishu/bitable.go` — 多维表格 CRUD
  - `ListRecords(appToken, tableID, pageToken, pageSize)` → 分页获取记录
  - `CreateRecord(appToken, tableID, fields)` → 创建记录，返回 record_id
  - `UpdateRecord(appToken, tableID, recordID, fields)` → 更新记录
  - `DeleteRecord(appToken, tableID, recordID)` → 删除记录
  - `GetTableFields(appToken, tableID)` → 获取表格字段定义（用于自动映射）

- `internal/feishu/models.go` — API 请求/响应类型定义

- `internal/feishu/field_mapping.go` — Task ↔ Bitable 字段映射
  - 默认映射：Title→任务名称(Text), Status→状态(SingleSelect), Priority→优先级(SingleSelect), DueDate→截止日期(Date), Tags→标签(MultiSelect), Description→描述(Text)
  - `TaskToRecordFields(task, projectName)` / `RecordFieldsToTask(fields)`

### 1.2 数据模型 & 存储

**修改文件：`internal/store/db.go`** — 新增迁移：

```sql
CREATE TABLE IF NOT EXISTS feishu_sync_map (
    id                TEXT PRIMARY KEY,
    local_task_id     TEXT NOT NULL,
    bitable_record_id TEXT NOT NULL DEFAULT '',
    bitable_app_token TEXT NOT NULL DEFAULT '',
    bitable_table_id  TEXT NOT NULL DEFAULT '',
    last_synced_at    TEXT NOT NULL,
    created_at        TEXT NOT NULL,
    updated_at        TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sync_local ON feishu_sync_map(local_task_id);
CREATE INDEX IF NOT EXISTS idx_sync_record ON feishu_sync_map(bitable_record_id);
```

**新建文件：**

- `internal/model/feishu.go` — `SyncMapping` 结构体
- `internal/store/sync_store.go` — `SyncStore` CRUD（Create/Update/GetByLocalTaskID/GetByRecordID/ListAll/Delete）

**修改文件：`internal/core/core.go`** — AppCore 新增 `SyncStore *store.SyncStore`

### 1.3 飞书服务层

**新建文件：`services/feishu_service.go`**

```go
type FeishuService struct {
    Core      *core.AppCore
    AIService *AIService
    client    *feishu.Client
    syncStop  chan struct{}
}
```

暴露的 Wails RPC 方法：

| 方法 | 说明 |
|---|---|
| `GetFeishuConfig()` | 从 ConfigStore 读取飞书配置 |
| `SaveFeishuConfig(cfg)` | 保存配置，重新初始化 client |
| `TestConnection()` | 验证 App ID/Secret，获取 token |
| `StartSync()` | 启动后台定时同步 goroutine |
| `StopSync()` | 停止同步 |
| `SyncNow()` | 立即执行一次同步 |
| `GetSyncStatus()` | 返回最近同步时间、记录数、错误 |
| `InitBitableTable()` | 自动创建/检查多维表格字段结构 |

**配置键**（存 config 表，复用 ConfigStore）：
`feishu_app_id`, `feishu_app_secret`, `feishu_bitable_app_token`, `feishu_bitable_table_id`, `feishu_sync_enabled`, `feishu_sync_interval`

### 1.4 双向同步算法

```
SyncCycle():
  1. 拉取本地所有 tasks + 所有 SyncMapping
  2. 分页拉取 Bitable 所有记录
  3. 遍历本地任务:
     - 有映射 → 比较双方 UpdatedAt vs LastSyncedAt
       - 仅本地变更 → push 到 Bitable
       - 仅远程变更 → pull 到本地
       - 双方都变更 → last-write-wins（按时间戳），记日志
     - 无映射 → 本地新建，push 到 Bitable，创建映射
  4. 遍历 Bitable 记录中无映射的 → 远程新建，pull 到本地
  5. 处理删除：映射存在但一方消失 → 同步删除另一方
  6. 发射 "task:changed" 事件刷新 UI
```

同步间隔默认 5 分钟，任务变更时（监听 `task:changed`）也触发即时同步。

### 1.5 注册服务

**修改文件：`main.go`**

```go
feishuSvc := &services.FeishuService{Core: appCore, AIService: aiSvc}
// 加入 Services 列表
// app.Event.On("task:changed", ...) 触发即时同步
// go feishuSvc.AutoStart() 根据配置自动启动
```

### 1.6 前端 UI

**新建文件：`frontend/src/components/FeishuSettings.tsx`**
- 飞书配置表单：App ID/Secret（密码框）、Bitable App Token、Table ID
- 测试连接按钮
- 同步开关 + 间隔设置
- 同步状态显示 + 立即同步按钮

**修改文件：`frontend/src/components/SettingsView.tsx`** — 引入 FeishuSettings 区块

**新建文件：`frontend/src/components/FeishuSyncStatus.tsx`** — 侧边栏同步状态指示器

---

## Phase 2: 飞书 Bot 集成

### 2.1 Bot 消息收发

**新建文件：`internal/feishu/bot.go`**
- `SendTextMessage(chatID, text)` — `POST /im/v1/messages`
- `SendCardMessage(chatID, card)` — 发送交互式卡片
- `BuildTaskCard(task)` — 构建任务详情卡片
- `StartEventListener(onMessage)` — WebSocket 长连接接收消息

### 2.2 AI 路由

Bot 收到消息 → 路由到 `AIService.ChatWithAI()` → 复用现有 tool-call（create_task/update_task/list_tasks）→ 将 AI 回复通过 `SendTextMessage` 发回飞书。

### 2.3 任务变更通知

监听 `task:changed` 事件，当任务状态变更时自动发送卡片通知到配置的飞书群。

### 2.4 前端

FeishuSettings 中增加 Bot 配置区：Bot 开关、目标群 Chat ID、通知策略。

---

## 关键文件清单

| 操作 | 文件路径 |
|---|---|
| 新建 | `internal/feishu/client.go` |
| 新建 | `internal/feishu/bitable.go` |
| 新建 | `internal/feishu/bot.go` (Phase 2) |
| 新建 | `internal/feishu/models.go` |
| 新建 | `internal/feishu/field_mapping.go` |
| 新建 | `internal/model/feishu.go` |
| 新建 | `internal/store/sync_store.go` |
| 新建 | `services/feishu_service.go` |
| 新建 | `frontend/src/components/FeishuSettings.tsx` |
| 新建 | `frontend/src/components/FeishuSyncStatus.tsx` |
| 修改 | `internal/store/db.go` — 新增 feishu_sync_map 表迁移 |
| 修改 | `internal/core/core.go` — AppCore 添加 SyncStore |
| 修改 | `main.go` — 注册 FeishuService，事件监听 |
| 修改 | `frontend/src/components/SettingsView.tsx` — 引入飞书设置 |

## 验证方案

1. **单元测试**: field_mapping 的转换逻辑、sync_store 的 CRUD
2. **集成测试**: 用真实飞书 App ID/Secret 测试 token 获取、Bitable 读写
3. **端到端验证**:
   - 在 TaskPilot 创建任务 → 确认出现在飞书多维表格
   - 在飞书多维表格修改记录 → 确认 TaskPilot 同步更新
   - 删除测试：两端各删除一条 → 确认同步删除
   - 冲突测试：同时修改同一任务 → 确认 last-write-wins 生效
4. **Bot 验证** (Phase 2): 在飞书群发消息 → 确认 AI 回复并执行任务操作
