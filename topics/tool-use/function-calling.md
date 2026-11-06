---
title: 函数调用原理：Function Calling 与工具使用全解
tags: [tool-use, function-calling, tool-calling, tools, json-schema, golang, bondseek, error-handling]
created: 2026-10-29
updated: 2026-10-29
status: drafted
---

# 函数调用原理：Function Calling 与工具使用全解

> **这是什么:** 工具使用（Tool Use）与函数调用（Function Calling）的完整原理——**为什么需要、怎么定义、模型怎么决定调、代码怎么执行、结果怎么喂回去**。面试题 #15 的答案素材。
> **为什么重要:** "Agent 的循环"里最核心的动作就是"调工具"——不懂工具调用原理，就不懂 Agent 怎么"动手"。
> **本文定位:** 理论为主（面向任何 LLM/语言），工具调用协议以 OpenAI JSON Schema 为基线；**落地部分以 BondSeek 的 `agenttools` + `tools/register.go`（Go 实现）为实例**——读完你能看到"理论怎么变成一行行真实代码"。
> **关联案例:** [多 Agent 编排](../../patterns/case-study-bondseek-multi-agent.md) | [认知架构范式](../agent-architecture/cognitive-architecture-paradigms.md)（ReAct 范式的 Action 就是工具调用）| [RAG 检索](../../patterns/case-study-bondseek-rag.md)（检索类工具的典型形态）
> **代码参考:** `bondseek-common/agenttools/tool.go` + `bondseek-agent/internal/tools/register.go`（2026-10-29 读取）

## 📌 名词速查表（先看这里）

| 英文 | 读作/理解成 | 一句话解释 |
|------|------------|-----------|
| **Function Calling** | "函数调用（专有名词）" | 模型"请求调用某个函数"的能力——不是它自己执行，是它输出"我想调 X 函数、参数是 Y"的结构化指令 |
| **Tool** | "工具" | Agent 能调用的外部能力（查库/执行代码/算指标）——函数调用的"函数"就是工具 |
| **Tool Calling** | "工具调用" | Function Calling 的通用叫法（不止函数，任意能力） |
| **JSON Schema** | "JSON 结构规范（专有名词）" | 描述参数长什么样的标准格式（字段名/类型/必填/描述） |
| **Tool Definition** | "工具定义" | 把工具"说明书"发给模型：名字 + 干什么 + 参数结构 |
| **Arguments** | "参数" | 模型按定义填出来的实际参数值（JSON） |
| **Tool Result** | "工具结果" | 工具执行完返回给模型的内容 |
| **Registry** | "注册表" | 存放所有可用工具的"工具库"，按名字找到对应实现 |
| **Max Iterations** | "最大轮数" | 防止"调工具→喂结果→再调"无限循环的硬上限 |
| **Header-Driven** | "模型主导（专有名词）" | 由模型决定调不调、调哪个——区别于代码写死 |

---

## 🎯 TL;DR 冲突摘要（先看这段）

> **这篇文档解决的核心矛盾：LLM 只会"输出文字"，不会真的"执行动作"。** 用户说"帮我算出这只债券的收益率"，模型不知道收益率怎么算——它需要查数据库、跑代码。但模型又不能直接碰你的系统（太危险），也不会真去查库（没有连接）。
> **破局方案就是 Function Calling（函数调用）：模型不"执行"工具，而是"请求"调用工具——它输出一段结构化 JSON（"我要调用 get_yield，参数是 bond_id=xxx"），你的代码收到这个请求后真正执行工具、把结果文本喂回给模型，模型基于结果继续推理。** 这样一来：模型负责"决策调什么"（它擅长），代码负责"真正执行"（它可靠），两边各干各擅长的。
> 完整闭环一句话：**模型收到工具清单（定义）→ 决定调哪个（决策）→ 输出参数（结构化 JSON）→ 代码执行（真实动作）→ 结果喂回（观察）→ 模型继续推理。**
> 读完全文你只需要带走一个判断：**工具调用 = 模型提出"我想做 X"的请求，代码负责"真的做 X"——把"决策"和"执行"分离，是 Agent 安全可控、能落地的根基。**

---

## 1. 为什么需要工具调用（先理解问题）

### 1.1 LLM 的三个"做不到"

| 做不到的事 | 例子 | 为什么做不到 |
|-----------|------|------------|
| **不知道实时数据** | "XX 债券今天的价格" | 训练数据有截止日期 |
| **不会精确计算** | "这个 10 年期国债的久期" | 数学能力弱、易幻觉 |
| **不能执行动作** | "把这些数据写进 Excel" | 模型只输出文字，不碰系统 |

**没有工具调用的 LLM 会怎样**：要么编造数字（幻觉），要么说"我查不了"（无能）。

### 1.2 工具调用的本质：把"决策"和"执行"分离

```
┌─────────────────────┐         ┌──────────────────────┐
│      LLM（决策方）    │         │     你的代码（执行方）  │
│                     │         │                      │
│  • 理解用户需求       │  请求   │  • 收到结构化 JSON      │
│  • 决定调哪个工具     │ ──────►  │  • 校验参数            │
│  • 生成参数           │         │  • 真正执行（查库/跑码） │
│  • 解读结果继续推理   │ ◄──────  │  • 返回结果文本         │
└─────────────────────┘  结果    └──────────────────────┘
```

**面试金句**：**LLM 不执行工具，它只"请求"调用工具；真正执行的是你的代码。这就是为什么工具调用让 Agent 既"聪明"（模型决策）又"安全可控"（代码执行）。**

---

## 2. 工具的三要素：定义 / 参数 / 结果

每个工具由三部分组成，缺一不可：

```
① 定义（Definition）   → 给模型看的"说明书"：名字 + 干什么 + 参数长什么样
② 参数（Arguments）    → 模型按说明书填的"订单"：想要什么值
③ 结果（Result）       → 执行完返回的"货"：模型下一步推理的依据
```

### 2.1 工具定义（Definition）——JSON Schema

模型是"盲人"，它只知道你给它的文字。**工具定义就是给模型看的"工具使用说明书"**：

```json
{
  "type": "function",
  "function": {
    "name": "search_bonds",
    "description": "按关键词搜索债券基本信息（代码/名称/评级/票面利率）",
    "parameters": {
      "type": "object",
      "properties": {
        "keyword": {
          "type": "string",
          "description": "债券代码或名称关键词"
        },
        "limit": {
          "type": "integer",
          "description": "返回条数",
          "default": 10
        }
      },
      "required": ["keyword"]
    }
  }
}
```

**三个关键字段**：
| 字段 | 作用 | 写不好会怎样 |
|------|------|------------|
| `name` | 唯一标识 | 重复会撞车，找不到该调谁 |
| `description` | **模型判断"什么时候用这个工具"的唯一依据** | 写不清楚模型就不会调，或滥用 |
| `parameters` | 参数结构（JSON Schema） | 模型填错参数，执行必失败 |

> **重要**：`description` 是给模型读的，不是给人读的——**你要写"这个工具用来查债券信息"，而不是写代码注释。** 模型靠它决定"现在该不该调这个工具"。

### 2.2 参数（Arguments）——模型填的 JSON

模型决定调工具后，按定义输出参数（结构化 JSON）：

```json
{"keyword": "24国债05", "limit": 5}
```

**你的代码要做的**：解析 JSON + 校验必填参数 + 类型转换——然后才真正执行。

### 2.3 结果（Result）——喂回给模型的"观察"

工具执行完，结果要**格式化成文本/结构化 JSON 喂给模型**：

```json
{
  "title": "搜索到 3 只匹配债券",
  "output": "1. 24国债05（019745）利率债 票面2.30%...\n2. ...",
  "metadata": {"duration_ms": 12, "success": true}
}
```

**关键**：结果要**给模型能理解的信息**，而不是给前端看的完整数据。太长的结果会被模型"淹没"（抓不住重点）——BondSeek 的做法是 `Title/Output/Metadata` 三分：**Title 给前端展示、Output 给模型推理、Metadata 存结构化元数据（时长/文件/成功状态）**。

---

## 3. 完整的调用闭环（模型视角的"动作循环"）

这是 ReAct 循环中 `Action` 的放大版——一次工具调用全流程：

```
① 系统发送：工具定义列表 + 对话历史
      ↓
② 模型推理：理解用户需求，判断"需要调 search_bonds"
      ↓
③ 模型输出：{"name": "search_bonds", "arguments": {"keyword": "24国债05"}}
      ↓
④ 代码端：注册表按名字找到 search_bonds → 校验参数 → 执行（查数据库）
      ↓
⑤ 代码端：把结果格式化成 ToolResult 文本
      ↓
⑥ 代码端：把结果作为一个"角色消息"塞回对话（role=tool）
      ↓
⑦ 模型继续推理：看到结果 → 决定"再查一个"或"信息够了，生成答案"
      ↓
⑧ 循环直到模型输出"最终答案"（或达到最大轮数上限）
```

**每一步谁负责**：
| 步骤 | 谁 | 干的事 |
|------|-----|--------|
| ①③⑦ | **模 型** | 看懂定义、决定调、生成参数、解读结果 |
| ④⑤⑥ | **代码** | 注册表路由、参数校验、真实执行、格式化回喂 |

> **面试金句**：一次工具调用 = 模型"请求"（结构化 JSON）→ 代码"执行"（真实动作）→ 结果"回喂"（新消息）→ 模型"再推理"。**AI Agent 的"动作能力"就是这么来的——模型永远只在"想"，代码永远只在"做"。**

---

## 4. 落地实例：BondSeek 的工具抽象（Go）⭐⭐

### 4.1 工具接口（agenttools/tool.go）——统一契约

```go
// Tool 工具接口：任何工具只要实现这三个方法就能接入系统
type Tool interface {
    Name() string                                     // 工具唯一标识
    Definition() ToolDefinition                       // 给 LLM 看的定义（JSON Schema）
    Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error)  // 真正执行
}

// ToolResult 统一输出结构（三分法）
type ToolResult struct {
    Title    string         `json:"title"`              // 给前端展示的简短标题
    Output   string         `json:"output"`             // 给模型推理的完整内容
    Metadata map[string]any `json:"metadata,omitempty"` // 给系统的结构化元数据
    Error    error          `json:"-"`                  // 错误（不发给模型）
}
```

**设计亮点 = 问题的答案**：
| 设计 | 解决什么 |
|------|---------|
| **接口统一（三方法）** | 所有工具一个样子，执行引擎不用关心具体工具逻辑 |
| **`Execute` 收 `json.RawMessage`** | 参数不绑定具体类型——工具自己解析校验（灵活） |
| **`Title/Output/Metadata` 三分** | 前端、模型、系统三方各取所需，互不污染 |

### 4.2 参数解析与校验（公共方法）

```go
// ParseArgs 泛型解析：把模型给的 JSON 参数转成强类型结构
func ParseArgs[T any](args json.RawMessage) (*T, error) {
    var result T
    if err := json.Unmarshal(args, &result); err != nil {
        return nil, fmt.Errorf("failed to parse arguments: %w", err)
    }
    return &result, nil
}

// ValidateRequired 校验必填参数
func ValidateRequired(args map[string]any, required []string) error {
    for _, key := range required {
        if _, ok := args[key]; !ok {
            return fmt.Errorf("missing required parameter: %s", key)
        }
    }
    return nil
}
```

### 4.3 工具注册（register.go）——工厂 + 全局注册表

```go
// init：启动时注册全部可用工具
func init() {
    reg := agentcore.GlobalTools    // 全局工具注册表

    reg.Register("read", func(rt *agentcore.Runtime) agenttools.Tool {
        return sandbox.NewReadTool(rt)     // 工厂函数：延迟创建 + 注入 Runtime
    })
    reg.Register("bash", func(rt *agentcore.Runtime) agenttools.Tool {
        return sandbox.NewBashTool(rt)
    })
    reg.Register("web_search", func(rt *agentcore.Runtime) agenttools.Tool {
        return research.NewWebSearchTool(rt)
    })
    reg.Register("search_reports", func(rt *agentcore.Runtime) agenttools.Tool {
        return research.NewSearchReportsTool(rt)
    })
    // ... read/write/edit/grep/lsp/plan_steps/think 等
}
```

**注册模式学习的三个点**：
| 模式 | 好处 |
|------|------|
| **工厂函数注册**（`func(rt) Tool`） | 工具直到要用才创建——不用的工具不浪费资源 |
| **`init()` 触发** | 启动时自动注册，加新工具只写一个文件即可（插件感） |
| **Runtime 注入** | 工具执行需要上下文（当前 Agent、会话、基础设施）——统一注入，工具不自己拼 |

### 4.4 一个具体工具的完整形态（标准结果封装）

```go
// 成功结果：Title 给前端、Output 给模型、Metadata 存结构化信息
func NewSuccessResult(title, output string, files []string, duration int64) *agenttools.ToolResult {
    return &agenttools.ToolResult{
        Title:    title,
        Output:   output,
        Metadata: map[string]any{
            "output_files": files,
            "duration_ms":  duration,
            "success":      true,
        },
    }
}

// llmContent 场景：结果里有关键信息（如表结构）必须让模型看到
func NewSuccessResultWithLLMContent(title, output, llmContent string, ...) *agenttools.ToolResult {
    metadata := NewStandardMetadata(files, duration, true, "")
    if llmContent != "" {
        metadata["llm_content"] = llmContent   // 专门给模型的关键信息
    }
    return &agenttools.ToolResult{Title: title, Output: output, Metadata: metadata}
}
```

> 📎 完整代码见：`bondseek-common/agenttools/tool.go` + `bondseek-common/agenttools/`（工具接口）+ `bondseek-agent/internal/tools/register.go`（注册）

---

## 5. 工具调用工程上有哪些坑（面试加分区）⭐⭐

> 理论讲完讲实战。这些问题不问不会 +1，问到了立刻让你和其他候选人拉开差距。

### 5.1 无限循环（最常见事故）

```
模型：调 search_bonds → 结果：返回 3 条 → 模型：再调 search_bonds → 结果：同 3 条 → ...
```

**缓 解**：
| 手段 | 说明 |
|------|------|
| **最大轮数上限（Max Iterations）** | 硬性锁死循环轮数，超了就停（BondSeek 各 Agent 配不同轮数上限） |
| **相同动作熔断** | 连续 N 次调用同一工具同一参数 → 判定死循环，强制终止 |
| **可观测性** | 工具调用树（`ToolCallRecorder`）——看每次调用耗时、参数，定位"为什么反复调" |

### 5.2 参数错误 / 工具失败

**模型填错参数是常态，不是意外。** 处理链：
```
校验失败 → 有意义的错误消息（告诉模型"缺什么"）→ 喂回给模型 → 模型自己修正重调
```

**关键**：**错误信息是给模型看的调试信息**——"missing required parameter: keyword"比"参数错误"强 100 倍，模型看了能自己改。

### 5.3 结果太长 / 太脏（上下文污染）

工具返回 2000 行的数据，全部塞回窗口 → 上下文爆炸 + 模型抓不住重点。

**缓 解**：
- **结果裁剪/摘要**：只回给模型"够推理的精简版"
- **输出与元数据分离**：`Output`（模型要的精简文本）vs `Metadata`（系统要的完整数据）——**大结果放 Metadata 不全量回喂模型**
- 呼应记忆案例的**上下文治理**：工具输出也走压缩/剪枝

### 5.4 工具的"边界"与新工具治理

**给模型的工具不是越多越好**——工具越多模型越容易"选错"：
| 问题 | 缓解 |
|------|------|
| 工具太多 → 模型选择困难 | 按 Agent 只暴露它该用的工具子集 |
| 危险工具误用 | 敏感工具加权限校验（谁能调 / 什么时候能调） |
| 工具随版本过期 | 注册表 + 版本管理，废弃工具下架 |

---

## 6. 工具执行失败怎么处理（面试题 #18 答案素材）⭐⭐

> **面试追问重灾区**："如果 Tool 执行失败（比如网络断了）怎么办？"——**答得好直接拉开差距**：大多数人只说"重试"，真正懂行的人会说"先分类再处理 + 失败是给模型的观察"。

### 6.1 核心思想：失败不是中断，是新的观察

> **面试金句**：**"把失败当成一条工具结果（Observation）喂回给模型，而不是中断整个对话。** 模型看到失败的 Observation，可以——重试、换参数、换工具、降级——**它永远有决策权。这正是 Agent 和工作流的区别**：工作流失败就停（代码写死），Agent 的失败是'新的输入'，模型自己决定下一步。"

### 6.2 第一步：先分类——不同失败不同处理

**不加区分地一律重试是错的**——重试只对瞬时类失败有效：

| 失败类型 | 典型表现 | 正确策略 |
|---------|---------|---------|
| **网络/基础设施**（瞬时） | 超时、连接断开、限流 429、5xx | ✅ **重试**（带退避，有上限） |
| **参数/格式**（模型错） | 缺必填参数、类型错误、JSON 解析失败 | ✅ **反馈给模型自改**（重试无效） |
| **业务/数据**（稳定） | 查无数据、权限不足、资源不存在 | ✅ **降级/绕行**（重试无效） |
| **安全/越权** | 试图调用无权限工具 | ✅ **直接拒绝**（绝不重试） |

### 6.3 第二步：四种处理策略

**① 重试（Retry）——只用于瞬时失败**
- **指数退避 + 抖动**（`1s→2s→4s`，避免重试风暴打在同一时刻）
- **有上限**（如 3 次或总时长 ≤30s），超了转降级
- **前提是幂等**：读操作可安全重试；写操作要幂等设计（呼应 SSE 案例的水位线去重——相同消息只生效一次）

**② 反馈给模型自改（Self-Correction）——Agent 特有**
> "参数错了怎么办？传统系统是'调用方自己改'；Agent 系统是**把错误信息作为工具结果喂回给模型，让它自己改**。错误消息要**写给模型看**：`missing required parameter: keyword`，模型看到了就能自己修正参数重调。"

```
模型调 search_bonds → 缺 keyword → 代码返回错误信息（role=tool）
→ 模型看到 "missing required parameter: keyword"
→ 自己补上 keyword 重调 → 成功
```

**③ 降级 / 绕行（Degradation）——针对业务失败**
- **换工具**：`search_bonds` 挂 → 用 `sql_query` 兜底查
- **降数据源**：实时行情失败 → 用缓存 / 最近交易日数据（明确标注数据时点）
- **降能力**：完整分析做不了 → 给简化答案 + 明确告知局限

**④ 优雅失败（Graceful Failure）——最后兜底**
> "实在不行，**明确告诉用户'这条查不到 / 暂时不可用'，附原因和建议**——**绝不假装成功**。'编造一个结果糊弄过去'是 Agent 最不能犯的错（幻觉红线，呼应防幻觉案例）。"

### 6.4 结合项目：BondSeek 怎么落地这条

| BondSeek 设计 | 对应哪条 |
|--------------|---------|
| **工具调用树记录 duration_ms + success/error** | 失败可观测——每个失败点留痕，失败原因 / 重试次数能回放（呼应可观测性案例） |
| **错误码体系（error_codes.go）**：固定错误码 + 模板参数，前端/模型拿到的是结构化提示而非裸字符串 | 失败消息"写给模型看" + 机器可识别 |
| **事件总线快慢线**：重要事件走 SlowLane 持久化（重试不丢），实时推送才走 FastLane | 重试的前提——消息不丢 |
| **全局 Recovery 中间件 + 上下文治理**：失败重试不把失败残留带进下一轮 | 兜底 + 上下文不污染 |

### 6.5 面试追问防御

**Q: "重试次数怎么定？都重试会浪费钱"**
> "按失败类型定：429 限流可以多试（退避后服务可能恢复），5xx 少试，**参数错不重试**（重试 100 次也是错，直接反馈模型）。每次重试都烧 token，所以上限 + 退避是硬要求。"

**Q: "什么时候算'真失败'该放弃？"**
> "三个信号任一触发就放弃：**重试次数耗尽 / 总超时上限 / 模型多轮修正仍参数错误**。放弃后走降级或优雅失败——**绝不能静默返回错误或编造结果**。"

**Q: "失败的错误信息太长，塞回窗口怎么办？"**
> "失败信息同样走**上下文治理**——裁剪/摘要后再喂回，关键只留'哪错了'（如 `missing: keyword`），几千字堆栈进 Metadata 不全量回喂。"

---

## 7. 面试回答框架（30 秒版）

### 送分版（只说原理）

> "**Function Calling（函数调用）的本质是让模型输出结构化的工具调用请求**——我给模型提供工具定义（名字、描述、JSON Schema 参数结构），模型判断需要调用时，输出 `{"name": "search_bonds", "arguments": {...}}` 这样的结构化 JSON。我的代码解析这个请求、校验参数、真正执行工具，再把结果格式化喂回给模型，模型基于结果继续推理。**模型负责'决策调什么'，代码负责'真正执行'，两者分离。**"

### 加分版（+ 落地细节 + 工程坑）⭐

> "实现上有几个关键的工程点。**第一，工具接口要统一**——我们定义了 `Tool` 接口（`Name()/Definition()/Execute()`），所有工具一个样子，执行引擎不关心具体逻辑。**第二，工具定义要注册**——启动时用工厂函数注册到全局注册表，要用才创建、按 Agent 暴露子集。**第三，结果要三分法**——`Title` 给前端展示、`Output` 给模型推理、`Metadata` 存结构化元数据，避免把几千行原始数据塞回上下文。**第四，防死循环**——必须配最大轮数上限 + 相同动作熔断，不然模型会反复调同一工具不收敛。**第五，错误信息要写给模型看**——'missing required parameter: keyword' 这种消息能让模型自己修正重调。"

### 送分版追加：工具失败处理（面试追问必答）

> "工具失败先**分类再处理**：网络类（超时/429/5xx）带退避重试、有上限；参数类是模型错了，**把'缺什么'的错误信息喂回去让它自改**；业务类（查无数据/权限不足）重试无效，走降级——换工具、降数据源，实在不行优雅告知用户，**绝不编造结果**。核心原则：**失败是给模型的一条 Observation，不是对话的终结**——这是 Agent 区别于工作流的地方，模型在 loop 里失败后仍有决策权。"

---

## 反模式（本主题避免）

- **让模型"执行"工具**——模型只"请求"，执行永远在代码（安全边界）
- **工具定义 description 写给人类看**——它是给模型判断"何时调用"的依据
- **工具结果全量塞回上下文**——太长会淹没模型，要裁剪/摘要/元数据分离
- **不注册直接硬编码工具列表**——加工具改代码、按 Agent 分不了工
- **没有轮数上限**——模型会无限循环调工具烧 token
- **错误消息不告诉模型缺什么**——"参数错误"模型不知道怎么改，等于白传

---

## 📖 名词标注表（完整中英对照）

| 英文 | 中文理解 | 详细解释 |
|------|---------|---------|
| **Function Calling** | 函数调用 | 模型输出"想调用某函数+参数"的结构化请求（不自己执行） |
| **Tool Calling** | 工具调用 | Function Calling 的通用叫法（工具＝能力，不限于函数） |
| **Tool** | 工具 | Agent 可调用的外部能力（查库/执行/计算） |
| **Tool Definition** | 工具定义 | 发给模型的"说明书"：名字+描述+参数结构 |
| **JSON Schema** | JSON 结构规范 | 描述参数结构的标准格式（OpenAI Tool Calling 用它） |
| **Arguments** | 参数 | 模型按定义填的实际参数（JSON） |
| **Tool Result** | 工具结果 | 执行后返回给模型的内容（Title/Output/Metadata） |
| **Registry** | 注册表 | 工具库：按名路由到实现 |
| **Factory Function** | 工厂函数 | 延迟创建工具的函数（`func(rt) Tool`） |
| **Runtime 注入** | 运行时注入 | 创建工具时传入当前上下文（Agent/会话/基础设施） |
| **Max Iterations** | 最大轮数 | 防死循环的硬上限 |
| **熔断 (Circuit Break)** | 熔断 | 连续相同动作时强制终止 |
| **structured_output** | 结构化输出 | 模型按 JSON Schema 输出，保证格式可控 |
| **角色消息（tool role）** | 工具角色消息 | 工具结果作为一条特殊角色消息塞回对话 |

---

## 关联文档

- [`cognitive-architecture-paradigms.md`](../agent-architecture/cognitive-architecture-paradigms.md) — ReAct 范式的 `Action` 就是工具调用（Action 步骤放大版 = 本文）
- [`case-study-bondseek-multi-agent.md`](../../patterns/case-study-bondseek-multi-agent.md) — SubAgent 也是"工具"（工具化包装）+ 工具调用树（可观测）
- [`case-study-bondseek-rag.md`](../../patterns/case-study-bondseek-rag.md) — 检索类工具的典型形态（search_reports / web_search）
- [`case-study-bondseek-observability.md`](../../patterns/case-study-bondseek-observability.md) — 工具调用树（每次工具调用留痕：耗时/参数/结果）
- [`notes/agent-interview-questions.md`](../../notes/agent-interview-questions.md) — 面试话题索引（第 15 题）
