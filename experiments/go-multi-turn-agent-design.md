# 多轮对话 Go Agent 设计与实现

> **实验路径:** `experiments/go-agent/`
> **日期:** 2026-08-19
> **状态:** 已实现并验证

---

## 一、概述

基于内网环境的 OpenAI 兼容 API（`service-GLM-5-w8a8-vllm-Bigep`），用纯 Go 标准库实现一个命令行多轮对话 Agent。

### 核心目标

- 支持多轮对话上下文保持
- 流式（SSE）/ 非流式双模式
- 零外部依赖，内网可编译
- 命令行交互，支持内置管理命令

---

## 二、架构设计

### 2.1 分层结构

```
┌──────────────────────────────────────────┐
│              main.go                      │
│   CLI 交互层: scanner 循环 + 命令解析     │
├──────────────────────────────────────────┤
│              agent.go                     │
│   Agent 核心: 上下文管理 + 业务编排        │
├──────────────────────────────────────────┤
│              api.go                       │
│   APIClient: HTTP 传输层                  │
│   ├── ChatCompletion()   非流式           │
│   └── StreamChatCompletion()  SSE 流式    │
├──────────────────────────────────────────┤
│              types.go                     │
│   数据模型层: Message/Request/Response    │
├──────────────────────────────────────────┤
│              config.go                    │
│   配置层: 环境变量 → 默认值                │
└──────────────────────────────────────────┘
```

### 2.2 数据流

```
用户输入
    │
    ▼
main.go: scanner.Scan()
    │
    ▼
agent.Chat() / agent.ChatStream()
    │
    ├── 追加 user Message 到 messages[]
    ├── 构造 ChatCompletionRequest { messages: a.messages }
    │       │
    │       ▼
    │   api.go: APIClient.ChatCompletion() / StreamChatCompletion()
    │       │
    │       ▼
    │   POST http://{api}/v1/chat/completions
    │       │
    │       ▼
    │   模型返回响应
    │
    └── 追加 assistant Message 到 messages[]
    │
    ▼
返回结果给用户
```

---

## 三、多轮对话核心机制

### 3.1 上下文累积模式

多轮对话的本质：**每次请求携带全部历史消息，API 无状态，Agent 端维护状态。**

```go
// 伪代码 - 多轮对话循环
func (a *Agent) Chat(userInput string) (reply string) {
    // Step 1: 追加用户输入
    a.messages = append(a.messages, {role: "user", content: userInput})

    // Step 2: 发送全部历史
    resp := POST /v1/chat/completions { messages: a.messages }

    // Step 3: 追加模型回复
    a.messages = append(a.messages, {role: "assistant", content: resp.choices[0].message})

    return reply
}
```

**消息历史结构（3 轮对话后）：**

```
[0] system:  "你是一个智能 AI 助手..."
[1] user:    "我的名字是张三"
[2] assistant: "你好张三..."
[3] user:    "我的职业是什么？"
[4] assistant: "软件工程师"
[5] user:    "..."
[6] assistant: "..."
```

每轮对话新增 2 条消息（user + assistant），消息数组不断增长。

### 3.2 消息结构

```go
type Message struct {
    Role    string `json:"role"`    // "system" | "user" | "assistant"
    Content string `json:"content"`
}
```

复用 OpenAI 标准 Role 体系，直接序列化发送，无需格式转换。

### 3.3 系统提示词管理

- 始终位于 `messages[0]`
- `ClearHistory()` 清空 user/assistant 消息，保留 system prompt
- `SetSystemMessage()` 替换或新增

---

## 四、流式 / 非流式双模式

### 4.1 抽象对比

| 模式 | 函数签名 | 返回方式 | 上下文拼接时机 |
|------|---------|---------|--------------|
| 非流式 | `Chat(string) (string, error)` | 完整字符串 | 回复拿到后立即追加 |
| 流式 | `ChatStream(string) (<-chan string, error)` | channel 逐块推送 | 流结束后追加完整回复 |

### 4.2 流式实现要点

```go
func (c *APIClient) StreamChatCompletion(req *ChatCompletionRequest) (chan string, chan error) {
    // 1. 发起 HTTP 请求
    // 2. 逐行扫描 SSE 响应 ("data: ...")
    // 3. 跳过空行，跳过非 "data: " 前缀行
    // 4. 遇到 "[DONE]" 结束
    // 5. 解析 JSON 为 StreamChunk，提取 Delta.Content
    // 6. 通过 channel 发送
}
```

SSE 流式数据格式：

```
data: {"choices":[{"delta":{"content":"你好"},"finish_reason":null}]}

data: {"choices":[{"delta":{"content":"，"},"finish_reason":null}]}

data: {"choices":[{"delta":{},"finish_reason":"stop"}]}

data: [DONE]
```

Agent 层在 goroutine 中用 `strings.Builder` 拼接所有 chunk，流关闭后追加完整内容到历史。

---

## 五、并发安全设计

```go
type Agent struct {
    mu       sync.RWMutex   // 读写锁
    messages []Message
}
```

### 锁策略

| 操作 | 锁类型 | 原因 |
|------|--------|------|
| `Chat()` / `ChatStream()` 写消息 | `Lock()` 写锁 | 修改 messages |
| `GetHistory()` 读消息 | `RLock()` 读锁 | 只读，允许多并发 |
| `GetStats()` 读统计 | `RLock()` 读锁 | 只读，允许多并发 |
| `ClearHistory()` 重置 | `Lock()` 写锁 | 修改 messages |

### 消息副本保护

```go
func (a *Agent) GetHistory() []Message {
    a.mu.RLock()
    defer a.mu.RUnlock()
    history := make([]Message, len(a.messages))
    copy(history, a.messages)   // ← 返回副本，防止外部篡改
    return history
}
```

---

## 六、配置设计

### 环境变量加载

| 变量 | 用途 | 默认值 |
|------|------|--------|
| `API_BASE_URL` | 网关地址 | `20.200.25.216:8080` |
| `API_KEY` | 身份凭证 | (test-curl.sh 中的 GLM_KEY) |
| `MODEL_NAME` | 模型名称 | `service-GLM-5-w8a8-vllm-Bigep` |
| `MAX_TOKENS` | 单次最大 token | `1024` |
| `STREAM` | 流式开关 | `true` |
| `CHAT_ID` | 会话标识 | `chat_{pid}` |

所有配置提供 `getEnv` / `getEnvInt` / `getEnvBool` 三类加载函数，支持类型转换。

---

## 七、CLI 命令设计

| 命令 | 功能 |
|------|------|
| `/exit`, `/quit` | 退出程序 |
| `/clear` | 清空对话历史（保留系统提示词） |
| `/stats` | 显示对话统计和历史预览 |
| `/stream on\|off` | 切换流式/非流式模式 |
| `/help` | 显示帮助 |

---

## 八、验证结果

### 8.1 单元测试（本地逻辑）

```
9 测试全部通过:
  TestNewAgent          ✓  初始化 + 系统提示词
  TestSetSystemMessage  ✓  更新系统提示词
  TestClearHistory      ✓  清空保留 system
  TestMessageCreation   ✓  消息辅助函数
  TestGetStats          ✓  统计信息
  TestResetChatID       ✓  重置会话 ID
  TestConcurrentAccess  ✓  并发读写安全
  TestHistoryCopy       ✓  副本独立性
  TestLargeMessage      ✓  大消息处理
```

### 8.2 端到端测试（真实 API 调用）

**测试场景 — 3 轮对话上下文保持：**

| Round | 用户输入 | 模型回复 | 结果 |
|-------|---------|---------|------|
| 1 | 我叫张三，是北京软件工程师 | 确认信息并问候 | ✅ |
| 2 | 我刚才告诉你我的名字是什么？ | **张三** | ✅ |
| 3 | 我的职业是什么？ | **软件工程师** | ✅ |

消息历史自动累积至 7 条（1 system + 3 user + 3 assistant）。

### 8.3 编译验证

- `go build` 零错误
- 生成可执行文件 `agent.exe`

---

## 九、设计决策记录

| 决策 | 选项 | 选择理由 |
|------|------|---------|
| 外部依赖 | 引入 cobra/goquery vs 零依赖 | 内网环境无法下载，纯标准库 |
| 并发模型 | Mutex vs Channel | 读多写少，RWMutex 更高效 |
| 上下文传递 | 全量历史 vs 滑动窗口 | 全量简单可靠，未来可加截断 |
| 配置方式 | 配置文件 vs 环境变量 | 环境变量无需写文件 parser |
| 流式返回 | Channel vs Callback | Channel 解耦生产消费方 |

---

## 十、未来可扩展方向

- **长上下文管理**: `max_history` 滑动窗口截断，超长时丢弃最早非 system 消息
- **对话持久化**: 消息历史保存到文件/数据库，支持恢复
- **多会话管理**: 同时维护多个 chatID 的独立上下文
- **Tool Calling**: 接入函数调用能力
- **Prompt 模板**: 支持自定义 system prompt 模板
- **对话导出**: Markdown/JSON 格式导出对话记录

---

## 交叉引用

- [Agent 系统架构](../../topics/agent-architecture/index.md)
- [工具使用与函数调用](../../topics/tool-use/index.md)
- [记忆管理与状态持久化](../../topics/memory-and-state/index.md)
- [提示工程与指令设计](../../topics/prompt-engineering/index.md)

---

> **实验结论:** 纯 Go + 零外部依赖 可以实现完整的多轮对话 Agent。核心是 Agent 端维护消息历史数组，每次请求携带全部上下文。流式/非流式通过统一的 Channel 抽象对外暴露。内网环境编译部署可行。
