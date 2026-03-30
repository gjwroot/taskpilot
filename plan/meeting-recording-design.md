# 会议录音功能设计方案

## Context

TaskPilot 是一个基于 Wails v3 (Go + React) 的 AI 驱动桌面任务管理应用。用户希望增加会议录音功能，支持系统音频捕获、说话人分辨与重命名、AI 智能总结、以及将会议内容自动分解为项目任务。目前应用没有任何音频/媒体处理能力，需要从零构建。

---

## 一、需求总结

| 需求项 | 决策 |
|--------|------|
| 音频采集 | 系统音频捕获（ScreenCaptureKit 优先，低版本降级虚拟音频驱动） |
| 语音转文字 | 可配置：默认本地 Whisper，可选云端 API |
| 说话人分辨 | pyannote.audio (Python sidecar)，自动分辨 + 手动修正 |
| AI 分析 | 可配置：默认 Claude API，可选其他 AI 服务 |
| 功能入口 | 侧边栏「会议」+ 可关联到具体项目 |
| 任务创建 | 预览确认模式：AI 建议 → 用户勾选/编辑 → 批量创建 |
| 数据存储 | 本地文件（~/.taskpilot/meetings/），SQLite 存元数据 |

---

## 二、数据模型

### 2.1 Meeting（会议）

```go
type Meeting struct {
    ID             string    // UUID
    ProjectID      string    // 可选，关联项目
    Title          string    // 会议标题
    Status         string    // recording | transcribing | diarizing | analyzing | done | error
    AudioPath      string    // 音频文件路径 ~/.taskpilot/meetings/{id}/audio.wav
    TranscriptPath string    // 转录文件路径 ~/.taskpilot/meetings/{id}/transcript.json
    Summary        string    // AI 总结文本
    Duration       int       // 录制时长（秒）
    CreatedAt      time.Time
    UpdatedAt      time.Time
}
```

### 2.2 MeetingSpeaker（说话人）

```go
type MeetingSpeaker struct {
    ID           string // UUID
    MeetingID    string
    SpeakerLabel string // 自动编号 "Speaker 1"
    DisplayName  string // 用户命名 "张明"
    Color        string // UI 标识色 "#60a5fa"
}
```

### 2.3 TranscriptSegment（转录片段）

```go
type TranscriptSegment struct {
    ID        string
    MeetingID string
    SpeakerID string
    StartTime float64 // 秒
    EndTime   float64 // 秒
    Text      string
}
```

### 2.4 数据库迁移

在 `internal/store/db.go` 新增三张表：`meetings`、`meeting_speakers`、`transcript_segments`。

---

## 三、系统架构

### 3.1 核心流程

```
音频采集 → 语音转文字 → 说话人分辨 → AI 分析 → 任务预览/创建
```

### 3.2 后端新增模块

| 模块 | 路径 | 职责 |
|------|------|------|
| 音频采集 | `internal/audio/capture.go` | AudioCapturer 接口定义 |
| 音频采集 macOS | `internal/audio/capture_darwin.go` | macOS ScreenCaptureKit CGo 实现 |
| 音频采集降级 | `internal/audio/capture_fallback.go` | 虚拟音频驱动方案 |
| 转录引擎 | `internal/transcribe/engine.go` | Transcriber 接口定义 |
| 本地转录 | `internal/transcribe/whisper.go` | whisper.cpp CGo 绑定 |
| 云端转录 | `internal/transcribe/cloud.go` | OpenAI Whisper API 等 |
| 说话人分辨 | `internal/diarize/diarize.go` | pyannote.audio Python sidecar 管理 |
| 会议 AI 分析 | `internal/ai/meeting_analyzer.go` | 总结 + 任务分解 prompt |
| 会议数据存储 | `internal/store/meeting_store.go` | Meeting CRUD |
| 会议服务 | `services/meeting_service.go` | 编排录制→转录→分辨→分析流程 |
| 模型定义 | `internal/model/meeting.go` | Meeting, MeetingSpeaker, TranscriptSegment |

### 3.3 Python Sidecar (说话人分辨)

- 打包一个独立的 Python 脚本 `scripts/diarize.py`
- 使用 pyannote.audio 进行说话人分辨
- Go 通过 `os/exec` 调用，JSON 格式交换数据
- 首次使用时自动检测 Python 环境并安装依赖

### 3.4 文件存储结构

```
~/.taskpilot/
  meetings/
    {meeting-id}/
      audio.wav          # 原始音频
      transcript.json    # 转录结果（含时间戳）
      diarization.json   # 说话人分辨结果
      summary.md         # AI 总结
  whisper-models/        # 本地 Whisper 模型文件
```

---

## 四、UI 设计

### 4.1 侧边栏

在 `Sidebar.tsx` 新增「🎙 会议」入口，与「今日」「项目」同级。

### 4.2 页面 1：会议列表 (`MeetingList.tsx`)

- 顶部「开始录制」按钮
- 会议卡片列表：标题、关联项目、参会人数、时长、状态标签
- 录制中的卡片显示暂停/停止控制按钮
- 状态流转：🔴 录制中 → ⏳ 转录中 → 👥 分辨中 → 🤖 分析中 → ✅ 已完成

### 4.3 页面 2：转录详情 (`MeetingDetail.tsx`)

- **左侧**：逐句转录文本，每句带说话人颜色标签 + 时间戳
- **右侧**：说话人管理面板
  - 每个说话人卡片：颜色标识、名称（可点击编辑）、发言次数/时长统计
  - 支持拖拽合并说话人
  - 未命名说话人显示 "Speaker N"
- **顶部**：返回按钮、会议标题、「AI 分析」按钮

### 4.4 页面 3：AI 分析结果 (`MeetingAnalysis.tsx`)

- **左侧**：会议总结（关键结论、待决事项、行动要点）
- **右侧**：建议任务列表
  - 每个任务：勾选框 + 标题 + 优先级 + 指派人 + 截止日期
  - 用户可逐条编辑/取消勾选
  - 底部「创建已勾选任务」按钮，批量创建到关联项目

### 4.5 录制开始弹窗 (`RecordingStartDialog.tsx`)

- 选择/输入会议标题
- 选择关联项目（可选）
- 音频源选择（系统音频）
- 「开始录制」确认

### 4.6 新增前端组件清单

| 组件 | 路径 |
|------|------|
| MeetingList | `frontend/src/components/MeetingList.tsx` |
| MeetingDetail | `frontend/src/components/MeetingDetail.tsx` |
| MeetingAnalysis | `frontend/src/components/MeetingAnalysis.tsx` |
| RecordingStartDialog | `frontend/src/components/RecordingStartDialog.tsx` |
| RecordingIndicator | `frontend/src/components/RecordingIndicator.tsx` |
| SpeakerPanel | `frontend/src/components/SpeakerPanel.tsx` |
| TaskPreviewList | `frontend/src/components/TaskPreviewList.tsx` |

---

## 五、事件系统

复用 Wails 事件机制，新增事件：

| 事件名 | 触发时机 |
|--------|----------|
| `meeting:changed` | 会议 CRUD 操作完成 |
| `meeting:recording:status` | 录制状态变化（开始/暂停/恢复/停止） |
| `meeting:transcribe:progress` | 转录进度更新（百分比） |
| `meeting:diarize:progress` | 说话人分辨进度更新 |
| `meeting:analyze:start` | AI 分析开始 |
| `meeting:analyze:done` | AI 分析完成 |

---

## 六、AI Prompt 设计

### 6.1 会议总结 Prompt

```
你是一个专业的会议纪要助手。请分析以下会议转录文本，生成结构化总结：

## 输入
- 会议转录文本（含说话人标识和时间戳）
- 参会人列表

## 输出格式
1. **关键结论**：会议达成的主要共识（3-5 条）
2. **待决事项**：未达成共识需要后续跟进的问题
3. **行动要点**：明确的下一步行动（含责任人和时间）
```

### 6.2 任务分解 Prompt

```
基于以下会议总结和转录内容，分解出具体可执行的任务：

## 输出格式（JSON 数组）
[{
  "title": "任务标题",
  "description": "任务描述",
  "priority": 0-3,
  "assignee": "责任人（从参会人中匹配）",
  "due_date": "截止日期（从上下文推断）",
  "tags": ["标签"]
}]
```

---

## 七、设置页扩展

在 `SettingsView.tsx` 新增「会议录音」配置区：

- **转录引擎**：本地 Whisper / 云端 API（下拉选择）
- **Whisper 模型**：tiny / base / small / medium / large（本地模式）
- **云端 API 配置**：API 地址、API Key
- **会议 AI 服务**：使用默认 Claude / 自定义（地址+Key）
- **存储路径**：会议文件存储目录（默认 ~/.taskpilot/meetings/）

---

## 八、实施步骤

### Phase 1: 基础设施（数据模型 + 存储）
1. 新增 `internal/model/meeting.go` — Meeting, MeetingSpeaker, TranscriptSegment 结构体
2. 新增 `internal/store/meeting_store.go` — CRUD 操作
3. 在 `internal/store/db.go` 添加数据库迁移
4. 新增 `services/meeting_service.go` — 基础 CRUD 服务
5. 创建 `~/.taskpilot/meetings/` 目录结构

### Phase 2: 音频采集
6. 新增 `internal/audio/capture.go` — AudioCapturer 接口
7. 新增 `internal/audio/capture_darwin.go` — macOS ScreenCaptureKit 实现
8. 新增 `internal/audio/capture_fallback.go` — 虚拟音频驱动降级
9. 在 MeetingService 中集成录制控制（开始/暂停/停止）
10. 添加录制相关 Wails 事件

### Phase 3: 语音转文字
11. 新增 `internal/transcribe/engine.go` — Transcriber 接口
12. 新增 `internal/transcribe/whisper.go` — whisper.cpp 本地转录
13. 新增 `internal/transcribe/cloud.go` — 云端 API 转录
14. 集成到 MeetingService 的录制完成后流程

### Phase 4: 说话人分辨
15. 新增 `internal/diarize/diarize.go` — Diarizer 接口 + pyannote sidecar
16. 创建 `scripts/diarize.py` — Python 说话人分辨脚本
17. 实现说话人自动检测 + 结果合并到转录片段
18. 说话人重命名/合并 API

### Phase 5: 前端 - 会议列表与录制
19. 新增 Sidebar「会议」入口
20. 新增 `MeetingList.tsx` — 会议列表页
21. 新增 `RecordingStartDialog.tsx` — 录制开始弹窗
22. 新增 `RecordingIndicator.tsx` — 录制中状态指示
23. Zustand store 添加 meetings 状态
24. 添加 meeting 相关 Wails 事件监听

### Phase 6: 前端 - 转录详情与说话人管理
25. 新增 `MeetingDetail.tsx` — 转录详情页
26. 新增 `SpeakerPanel.tsx` — 说话人管理面板
27. 实现说话人重命名、合并交互

### Phase 7: AI 分析与任务创建
28. 新增 `internal/ai/meeting_analyzer.go` — 总结 + 任务分解
29. 新增 `MeetingAnalysis.tsx` — AI 分析结果展示
30. 新增 `TaskPreviewList.tsx` — 任务预览确认列表
31. 实现批量任务创建（复用现有 TaskService）

### Phase 8: 设置与配置
32. 扩展 `SettingsView.tsx` 添加会议录音配置区
33. 扩展 `ConfigService` 支持会议相关配置项

---

## 九、关键文件清单

### 已有文件（需修改）
- `internal/store/db.go` — 添加 meetings 相关表迁移
- `internal/core/core.go` — 初始化 MeetingStore
- `services/ai_service.go` — 可能复用部分 AI 调用逻辑
- `frontend/src/components/Sidebar.tsx` — 添加「会议」导航
- `frontend/src/stores/appStore.ts` — 添加 meetings 状态
- `frontend/src/hooks/useWailsEvents.ts` — 添加 meeting 事件监听
- `frontend/src/App.tsx` — 添加会议视图路由
- `frontend/src/components/SettingsView.tsx` — 添加会议配置
- `main.go` — 注册 MeetingService

### 新建文件
- `internal/model/meeting.go`
- `internal/store/meeting_store.go`
- `internal/audio/capture.go`, `capture_darwin.go`, `capture_fallback.go`
- `internal/transcribe/engine.go`, `whisper.go`, `cloud.go`
- `internal/diarize/diarize.go`
- `services/meeting_service.go`
- `internal/ai/meeting_analyzer.go`
- `scripts/diarize.py`
- `frontend/src/components/MeetingList.tsx`
- `frontend/src/components/MeetingDetail.tsx`
- `frontend/src/components/MeetingAnalysis.tsx`
- `frontend/src/components/RecordingStartDialog.tsx`
- `frontend/src/components/RecordingIndicator.tsx`
- `frontend/src/components/SpeakerPanel.tsx`
- `frontend/src/components/TaskPreviewList.tsx`

---

## 十、验证方式

1. **音频采集**：启动录制 → 播放系统音频 → 停止录制 → 检查 WAV 文件是否包含音频内容
2. **语音转文字**：用一段已知内容的音频测试，对比转录文本准确性
3. **说话人分辨**：用包含 2-3 个不同说话人的音频测试，验证分辨结果
4. **说话人重命名**：在 UI 中修改说话人名称，验证转录文本中同步更新
5. **AI 总结**：对一段会议转录调用 AI 分析，检查输出格式和内容质量
6. **任务创建**：从 AI 建议任务中勾选部分 → 批量创建 → 验证任务出现在对应项目中
7. **端到端**：完整走一遍 录制 → 转录 → 分辨 → 重命名 → AI 分析 → 创建任务 流程
8. **构建验证**：`wails3 build` 成功编译，无类型错误
