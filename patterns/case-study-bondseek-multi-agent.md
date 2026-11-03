---
title: BondSeek 多 Agent 编排系统（AI Studio Supervisor 模式全解析）
tags: [case-study, multi-agent, orchestrator, supervisor, subagent, tool-wrapping, reapl, golang]
created: 2026-10-21
updated: 2026-10-21
status: draft
---

# BondSeek 多 Agent 编排系统（AI Studio Supervisor 模式全解析）

> **案例对象:** [bondseek-ai](https://github.com/) — 债券行业多租户 AI SaaS 投研平台（Go 1.24 微服务）
> **分析范围:** 多 Agent 编排的完整实现：Orchestrator（协调者）职责 / SubAgent 工具化包装 / 统一 ExecutionEngine / 动态提示词 / 技能分层路由 / 上下文隔离与结果瘦身
> **资料代码:** `bondseek-agent/internal/agents/aistudio/`（subagents/ 目录 + core/ 目录 + prompts/orchestrator_prompt.go）（2026-10-21 代码级深挖）
> **关联文档:** [RAG 检索系统](./case-study-bondseek-rag.md)、[整体案例分析](./case-study-bondseek-memory.md)、[用户记忆专项](./case-study-bondseek-user-memory.md)
> **面试话题索引:** [agent-interview-questions.md](../notes/agent-interview-questions.md) — 第 9 题"多 Agent 怎么编排"（本文即答案素材）

## 📌 名词速查表（先看这里）

> 本文讲多 Agent 编排，术语集中在"角色、包装、循环"三块。核心词先看这里（完整对照见文末"名词标注表"）：

| 英文 | 读作/理解成 | 一句话解释 |
|------|------------|-----------|
| **Multi-Agent** | "多智能体" | 多个 AI 角色分工协作完成一个任务的系统 |
| **Orchestrator** | "编排器/监工" | 总指挥：理解意图、委派专家、整合结果（自己不干活） |
| **SubAgent** | "子智能体/专家" | 专门领域的执行者（技术面专家、基本面专家…） |
| **Supervisor** | "监督者模式" | 编排架构：一个监工调度多个工人 |
| **Task** | "任务" | 派给子 Agent 的活儿（一段自包含的任务说明） |
| **Task Description** | "任务描述" | 监工写给专家的"工作单"（唯一传参） |
| **Function Calling** | "函数调用" | LLM 输出 JSON 表示"我要调用哪个工具、传什么参数" |
| **Tool** | "工具" | 模型能调用的外部能力（本系统里 SubAgent 也被包装成工具） |
| **ReAct** | "思考-行动循环" | 推理 → 行动 → 观察结果 → 再推理的标准 Agent 循环 |
| **ExecutionEngine** | "执行引擎" | BondSeek 循环逻辑的统一实现（所有 Agent 共用） |
| **Session** | "会话" | 一次对话的独立容器（子 Agent 每次调用开全新子会话） |
| **隔离 (Isolation)** | "互不污染" | 每个子任务的会话独立，不让上轮状态漏进下轮 |

---

## 🎯 TL;DR 冲突摘要（先看这段）

> **这篇文档解决的核心矛盾：一个复杂任务（投研分析）需要多种专业能力（技术面/基本面/可视化/组装），但"让一个 Agent 全干"必然上下文爆炸、职责混乱、一崩全崩；"让多个 Agent 各自为战"又没人整合结果。** 单独一个 Agent 干不动，多个 Agent 乱干，怎么办？BondSeek 的破局思路一句话总结：**"把多 Agent 协作降维成'函数调用'——监工不干活，只负责把专家包装成工具，用 LLM 的 Function Calling 机制调度它们。"** 读完全文你只需要带走一个画面：**"监工(Orchestrator) == 一个只懂'派活'的 LLM；专家(SubAgent) == 一个'随叫随到、干完就走、只交摘要'的工具；协作正确性 == 靠'全新子会话隔离 + 共享工作区 + 摘要瘦身'三个机制保障。"**

---

## 1. 架构总览：两层嵌套的"监工-工人"模型

### 1.1 什么是 Supervisor 模式

> **Supervisor（监督者）模式** = 多个 Agent 协作时，有一个"监工 Agent"（Orchestrator）负责调度，其余 Agent 是"工人"（Worker/SubAgent），各自只做自己领域的事。监工不做实事，做三件事：**理解意图、委派任务、整合结果**。

### 1.2 BondSeek 的两层嵌套全景

```
用户提问
  │
  ▼
┌─ Orchestrator（监工 / AI Studio 协调者）─────────────────────────┐
│  自己的 LLM ReAct 循环（flash=12轮 / deep=18轮）                  │
│  职责：理解意图 → 委派专家 → 整合结果（代码注释明说：              │
│        "纯协调者角色，不承担业务处理或状态维护"）                  │
│                                                                  │
│  被包装成工具的 SubAgent（LLM 函数调用决策谁上场）：              │
│  ├─ technical_analyst（技术面专家）→ 内置二级 data_analyst       │
│  ├─ fundamental_analyst（基本面专家）→ 内置二级 data_analyst     │
│  ├─ visualization_expert（可视化专家）                            │
│  ├─ dashboard_creator（仪表盘/研报组装）                          │
│  ├─ skill_creator（技能创建）※有条件注册                          │
│  ├─ cron_creator（定时任务创建）※有条件注册                       │
│  └─ dispatch 工具：dispatch_to_codecopilot / dispatch_to_quantlab │
│                         （跨模块路由：把任务转交给别的 Agent 模块）│
└──────────────────────────────────────────────────────────────────┘
          ▲ SubAgent 返回「摘要 + 产出文件列表」（不是完整输出！）
          │
  技术面专家（SubAgent）自己的 ReAct 循环（同一个 ExecutionEngine）
  自己的工具集：read_file / list_files / write_file / bash / 指标计算 / analyze_documents
  内置二级 SubAgent：data_analyst（数据查询专家，需要时才查数据）
```

### 1.3 核心设计哲学（orchestrator.go:46-50 注释原文）

```go
// 设计原则：
// - 纯协调者角色，不承担业务处理或状态维护
// - 使用 ExecutionEngine 执行，不重复实现循环逻辑
// - 工具注册和管理由 Orchestrator 负责
// - 工作空间由 Sandbox 服务远程管理
```

**三个"复用"一个"隔离"**：
- **循环逻辑复用**：Orchestrator 和 SubAgent 都用同一个 `ExecutionEngine`（只是系统提示词、工具集、最大轮数不同）
- **工具注册复用**：SubAgent 构造函数内自主注册自己的工具（封装修细节）
- **事件记录复用**：`recordEvent` 注入子 Agent，让子 Agent 的工具事件也写进主会话历史
- **会话/上下文隔离**：每次 SubAgent 调用开全新子会话，无状态，互不污染

> **面试一句话版**：BondSeek 的多 Agent 不是"每个 Agent 一套轮子"，而是"一辆车（ExecutionEngine）+ 不同司机（角色配置）：换工具箱、换岗位说明书、换驾驶时长（迭代上限）"。

---

## 2. SubAgent 接口：专家的"岗位说明书"（base.go:29-63）

```go
type SubAgent interface {
	Type() string                      // 标识："technical_analyst"
	Description() string               // 职责描述（给监工 LLM 看，决定是否委派）
	Category() string                  // 分组："分析层" / "可视化层" / "专项工具"
	Prompt() string                    // 系统提示词（岗位说明书全文）
	ToolNames() []string               // 可用工具名单
	MaxIterations() int                // 最大迭代轮数
	Execute(ctx, taskDescription,      // 执行：任务描述 + 子会话 ID + 主会话 ID
	        childConversationID, mainConversationID) (string, *SubAgentTree, error)
}
```

| 方法 | 给谁看 | 用途 |
|------|--------|------|
| `Type` / `Description` / `Category` | 监工 LLM + 前端 | 决定"这个专家能干什么、归哪一类" |
| `Prompt` / `ToolNames` / `MaxIterations` | SubAgent 自己 | 配置自己的循环 |
| `Execute` | SubAgent 自己 | 真正干活：带全新子会话从零执行 |

**接口设计要点**：`Description()` 和 `Category()` 是**写给编排器看的元数据**——监工靠描述判断"这个问题该派给谁"，靠分组生成"专家清单"区段展示。**专家自己不长"嘴巴"解释给自己听，元数据是给调度者消费的。**

---

## 3. SubAgentTool：把专家包装成"工具"（base.go:65-257）⭐

### 3.1 为什么要把 SubAgent 包装成 Tool

> 监工（Orchestrator）本身是一个 LLM ReAct 循环。LLM 唯一的"行动出口"是**函数调用（Function Calling）**——它输出一个 JSON，说"我要调用 `technical_analyst` 这个工具，参数是…"。BondSeek 给每个 SubAgent 包了一层 `SubAgentTool`，于是 **"召唤专家"就变成一个普通的工具调用**，监工 LLM 用和"读文件"完全一样的机制来"派活"。

工具定义（ToTool / Definition，会进 LLM 的 tools 参数）：

```json
{
  "type": "function",
  "function": {
    "name": "technical_analyst",
    "description": "多资产行情数据查询与技术指标计算，产出 CSV + analysis/tech.md...",
    "parameters": {
      "type": "object",
      "properties": {
        "task_description": {
          "type": "string",
          "description": "任务补充说明（可选）。省略时专家将直接基于用户原始问题自主执行。"
        }
      },
      "required": []
    }
  }
}
```

### 3.2 Execute 流程（base.go:144-257）：四步护城河

```
Step 1  解析 task_description（监工写的工作单）
        └─ 为空 → 兜底用 runtime.UserQuery（原始用户问题）
Step 2  注入当前时间前缀（BuildUserTimePrefix）
        └─ ⭐ 防坑设计：防止子 Agent 把工具返回的历史数据日期误当成"现在"
           （查上月行情返回 2025-xx-xx，子 Agent 误认 2025 是当前时间）
Step 3  createChildConversation：每次调用创建全新子会话
        └─ SubAgent 从零开始执行，无需 reset 逻辑
           （注释："行为等同于 agentcore 默认的 ReuseSubConversation=false"）
Step 4  发 subagent_start 事件 → agent.Execute() 全权执行 → 返回
        └─ 只返回「摘要 + 产出文件列表」（generateSummary）
```

### 3.3 ⭐ 父子会话机制（三个 ID 的分工）

| ID | 谁创建 | 用途 |
|----|--------|------|
| `mainConversationID` | 用户开启对话时 | 主会话：用户历史、事件记录 |
| `childConversationID` | 每次 SubAgent 调用时（`CreateChildConversationWithMetadata`） | 子会话：该专家这一次执行的全部中间状态 |
| `workspaceID` | 会话启动 | **不随会话走**：所有子 Agent 共享同一工作区（`workspaces/<tenant>/<user>/<conversation>`） |

**关键设计**：**会话隔离但工作区共享**——
- **会话（Context）**每次调用全新的 → 防止对话历史相互污染
- **工作区（Workspace/文件）**所有子 Agent 共享 → 专家产出的 CSV/MD 文件，下一个专家直接能读

> **这套设计完美解耦"记忆隔离"和"成果共享"**：对话上下文不许串，但劳动成果（文件）必须共享。这是多 Agent 协作里最常见的两个矛盾点，BondSeek 用"会话 + 工作区"两维度同时解决。

### 3.4 ⭐ 结果瘦身：只回摘要，不回全量（base.go:218-256）

```go
// === Phase 2: 只返回摘要和文件列表，不返回完整输出 ===
var outputFiles []string
// 从 SubAgentTree.Steps 提取所有 OutputFiles
...
return &agenttools.ToolResult{
	Title:  t.agent.Type() + " 执行完成",
	Output: summary,   // 分析结论摘要
	Metadata: map[string]any{
		"agent_type":   t.agent.Type(),
		"child_session": childID.String(),
		"output_files":   outputFiles,
		"total_steps":    totalSteps,
		"execution_time": executionTime,
		// llm_content 确保摘要能进入 orchestrator 的 LLM 上下文，
		// 避免 orchestrator 再用 read_file 逐个读取文件
		"llm_content": summary,
	},
}, nil
```

**为什么只回摘要**：技术面专家可能跑了几十步、读了几百行数据，如果完整输出进监工上下文，**监工的上下文窗口会被子 Agent 的"副产品"撑爆**。只回"结论 + 文件路径"：
- 结论（摘要）→ 监工 LLM 直接用来整合回答
- 文件路径 → 监工/下游专家需要细节时再 read_file

> **这是一句能写进简历的设计**：多 Agent 协作的上下文治理核心是"**子 Agent 的输出按需给**"——摘要常驻、细节按需读取、中间过程一律丢弃。

---

## 4. ExecutionEngine：所有人都用的那个"循环"（engine.go）⭐

### 4.1 ReAct 循环（engine.go:140-386）

```
for step := 1; step <= MaxIterations; step++ {
	if ctx.Err() != nil { /* 用户中断：正常退出，不报错 */ }

	// 每轮先刷新系统提示词动态部分（DynamicPromptRefresher）
	// 替换 messages[0].Content（含：专家清单、技能列表、当前时间）

	// 1. LLM 推理（流式，支持指数退避重试 3 次）
	response := callLLM(ctx, messages, toolChoiceOverride)

	// 2. 无工具调用 → 完成！保存上下文，回调 OnComplete
	if len(response.ToolCalls) == 0 { success := true; break }

	// 3. 执行工具调用（Action），结果追加进消息（Observation）
	toolMessages := executeToolCalls(ctx, response.ToolCalls)
	messages = append(messages, toolMessages...)

	// 4. 太多次失败？注入强制降级指令
	if injected := checkAndInjectDegradation(...); injected { ... }

	// 5. 工具返回 _break_loop=true？提前退出
	if checkBreakLoop(toolMessages) { success := true; break }

	// 6. 记录工具调用树 + 保存上下文
}
// 达到最大迭代 → "宽容完成"：有产出文件也算成功
```

### 4.2 三个工程级防护（engine.go 最值得抄的部分）

| 防护 | 实现 | 解决什么问题 |
|------|------|------------|
| **LLM 指数退避重试** | callLLM 最多 3 次，延迟 200/400/600ms；**已向客户端发送过内容则不重试**（防止重复输出） | LLM 瞬时故障，不抖动给用户 |
| **工具 3 连败硬降级** | `toolFailCounts` 计数，≥3 次该工具被硬拦截（executeToolCalls），并向 LLM 注入消息"停止重试，说明不可用后完成任务" | 防止 Agent 卡死在坏工具上死循环 |
| **宽容完成** | 达到最大迭代但**已有产出文件**（图表/数据）→ 标记 Success=true | 迭代上限不等于失败，有成果就是干完了 |

### 4.3 中间件机制（engine.go:52-108）

```go
// 横切中间件（预算/审计/RAG 注入等），按注册顺序执行
middlewares []types.Middleware
// 钩子：BeforeReasoning（推理前，可注入强制 tool_choice）/ AfterTool / BeforeCompress
if config.TokenBudget > 0 {
    middlewares = append(middlewares, NewBudgetMiddleware(config.TokenBudget))
}
```

- **BudgetMiddleware（预算熔断）**：token 超限后自动设置 `tool_choice="none"`（禁止再调工具），强制收尾
- 钩子挂点：`BeforeReasoning`（推理前）、`AfterTool`（工具后）、`BeforeCompress`（压缩前）
- 设计思想：**横切关注点（预算、审计、注入）不写在循环体里，而是插件化挂接**——循环逻辑保持纯净

---

## 5. 动态提示词（DynamicPromptRefresher）：每轮"活"的系统提示词 ⭐

### 5.1 机制（orchestrator.go:217-235）

```go
dynamicRefresher := func() string {
	parts := []string{prompts.OrchestratorStaticPrompt}  // 静态部分（身份+工作策略）
	if zone := o.buildSubAgentZone(); zone != "" {
		parts = append(parts, zone)     // 专家清单（根据已注册 SubAgent 自动生成）
	}
	if o.homepageSkillContent != "" {
		parts = append(parts, o.homepageSkillContent)  // 首页技能（context_type 会话）
	}
	if section := o.buildSkillListSection(ctx); section != "" {
		parts = append(parts, section)  // 可用技能列表（每轮从技能库枚举）
	}
	parts = append(parts, prompts.BuildTimeZone())  // 当前时间（最后）
	return strings.Join(parts, "\n\n")
}
// engine 每轮循环开始前：messages[0].Content = dynamicRefresher()
```

### 5.2 为什么要动态刷新

| 区段 | 为什么必须动态 |
|------|--------------|
| **专家清单区** | 跟"这次注册了哪些 SubAgent"走（skill_creator/cron_creator 是有条件注册的，不是每次都在） |
| **技能列表区** | 跟"用户这轮激活了什么技能"走，每轮从 DB 技能库重新枚举 |
| **时间区** | 跟真实时钟走（"现在几点了"必须逐轮更新） |

### 5.3 ⭐ 静态 vs 动态的平衡艺术

- **静态部分保持逐字不变**：系统提示词开头固定 → 触发 LLM 侧 **prompt cache（提示词缓存）**命中，省 token、降延迟（人工智能推理服务的缓存机制：相同前缀直接复用缓存）
- **动态部分追加在后**：变化的部分不破坏缓存前缀
- 代码注释原文："保持纯静态利于 LLM 侧 prompt cache 命中"（orchestrator_prompt.go:11）

> **面试可答点**：动态提示词不是"每次全量重发"，而是"**静态前缀保缓存 + 动态后缀促更新**"的工程平衡。

---

## 6. 系统提示词里的"软约束"（Prompt 工程）

### 6.1 OrchestratorStaticPrompt 拆解（orchestrator_prompt.go:12-65）

```
## 身份
你是用户的投研分析助手…负责理解意图、调度专家、整合结果。
你自己不执行数据查询或图表生成，必须委托专家完成。   ← 角色边界写死

## 工作策略（把编排流程固化为 if-then 规则）
1. 非分析类 → 直接回复
2. 技能创建 → 委派 skill_creator；定时任务 → 委派 cron_creator
3. 普通分析任务，按层级依次调度：
   - 分析层：委派 tech/fund（能并行就并行调用）    ← 鼓励并行
   - 可视化层：只要产出过 CSV/MD → "必须"先委派 visualization_expert  ← 强制依赖
     （仅在"用户只要文字"或"两专家都明确无可视化素材"时跳过）
   - 研报组装层：调用"前自检"——"本轮调过 visualization_expert 吗？没有就回去"  ← 反幻觉
4. 继续对话 → 先 read_file("index.md") 回顾进展
5. 回答简洁 ≤1000 字
6. 委派过程中不要发送叙述性文本，只在最后整合输出

## 失败处理与用户解释（失败怎么说人话）
- 工具反复报错 → 直接说明关键错误
- 达到最大迭代 → 说明任务未完成，列出已产出文件
- 无产物 → 说明失败阶段和原因，不要声称已完成

## 技能加载约束（权限分层）
- 你只能 load_skill 通用技能（scope=aistudio）
- 专家级技能可见元数据但不可加载 → 委派时在 task_description 指示专家自己加载
```

### 6.2 三句最值得学的 Prompt 心法

| 心法 | 实现 | 解决什么问题 |
|------|------|------------|
| **"你自己不执行，必须委托"** | 身份段写死角色边界 | 防止监工越权自己干，破坏分工 |
| **"能并行就并行调用"** | 策略段显式鼓励 | 让 LLM 知道并行是允许的选项 |
| **"调用前自检"** | 策略段嵌入核查步骤 | 反漏步骤：LLM 自身上下文里核实"我前一步做了吗" |

---

## 7. 技能分层路由：权限与调度解耦（orchestrator.go:565-639）

```go
// scopeExpertMapping：专家级 scope → SubAgent 名称映射
var scopeExpertMapping = map[string]string{
	"aistudio-techanalyst":   "technical_analyst",
	"aistudio-fundanalyst":   "fundamental_analyst",
	"aistudio-visualization": "visualization_expert",
}
```

### 7.1 三层技能权限

| 层 | scope | 谁能加载 | 用途 |
|----|-------|---------|------|
| 通用技能 | `aistudio` | Orchestrator 直接 `load_skill` | homepage 场景、技能展示 |
| 专家级技能 | `aistudio-techanalyst` / `fundanalyst` / `visualization` | **只有对应专家** | 分析框架、行业方法论 |
| （前端激活的技能） | 动态 | 按映射路由 | 用户点了"用这个技能" |

### 7.2 编排器看到元数据但不越权

- `buildSkillListSection`：Orchestrator **能看到**专家级技能的元数据（名称+描述）→ 用于"这个需求该派给谁"的决策
- 但加载动作被限制：委派时在 `task_description` 开头写 `请先 load_skill("xxx") 加载技能框架，然后严格按其指导执行`
- **SubAgent 收到委派后用自己 scope 的 load_skill 加载**

> **设计价值**：调度决策权和领域执行权分离——监工懂"派谁"但不碰"怎么干"，专家懂"怎么干"但无权指挥别人。技能即权限边界。

---

## 8. SubAgent 内部结构：嵌套的第二个层级（以 technical_analyst 为例）

### 8.1 构造函数自主注册工具（technical_analyst.go:61-132）

```go
fmt.NewTechnicalAnalyst(...) {
	// 1. 创建带命名空间的 runtime 副本（隔离文件输出路径）
	subRuntime := runtime.WithSubAgentType(string(constants.AgentTypeAIStudioTechAnalyst))

	// 2. 自己的工具注册表（全新 Registry）
	toolRegistry := core.NewRegistry()

	// 3. Common 工具
	toolRegistry.Register(file.NewReadFileTool(subRuntime))
	toolRegistry.Register(file.NewListFileTool(subRuntime))

	// 4. Skill 系统（DB-backed load_skill）
	skillKit := NewSubAgentSkillKit(skillStore, ...)

	// 5. ⭐ 二级 SubAgent：data_analyst（按需调用查询数据）
	if dataAnalyst != nil {
		toolRegistry.Register(NewSubAgentTool(dataAnalyst, subRuntime, nil))
	}

	// 6. 数据处理工具：write_file + bash（LLM 自己写 Python 脚本执行）
	toolRegistry.Register(file.NewWriteFileTool(subRuntime))
	toolRegistry.Register(file.NewBashTool(subRuntime))
	// ...

	// 7. 网络搜索（仅用户前端开启时注册）
	if webSearch { toolRegistry.Register(NewWebSearchTool(...)) }

	// 8. 文档分析（RAG 服务可用时）
	if runtime.RAGService != nil { toolRegistry.Register(NewDocumentsTool(...)) }
}
```

### 8.2 ⭐ 三个值得注意的细节

| 细节 | 含义 |
|------|------|
| **runtime.WithSubAgentType** | 为子 Agent 创建**命名空间的 runtime 副本** → 文件输出路径按专家隔离（各专家写自己的目录） |
| **二级 SubAgent（data_analyst）** | 技术面专家自己也有"下属"——数据查询专家。**说明嵌套层级是递归的，不是只有两层"监工-工人"** |
| **条件注册工具** | webSearch 开关由用户控制、RAG 服务由可用性控制 → **工具集按运行环境动态决定**，不给 Model 暴露不可用的工具 |

### 8.3 完整嵌套链（一次真实任务的调用树）

```
Orchestrator（监工）
 └─ tool: technical_analyst（技术面专家）
     ├─ tool: read_file / list_files
     ├─ tool: data_analyst（二级：数据查询专家）← 技术面专家派给它的
     │     └─ tool: query_market_data / compute_indicators ...
     ├─ tool: write_file + bash（写 Python 脚本跑数据）
     └─ tool: analyze_documents（文档 RAG 检索）
 └─ tool: visualization_expert（可视化专家）
     └─ tool: write_file + bash（生成 ECharts JSON）
 └─ tool: dashboard_creator（研报组装）
     └─ tool: read_file（读 charts/xxx.json）
最终：Orchestrator 整合 → 1000 字内输出 → 前端研究报告
```

> **这个调用树会通过 ToolCallRecorder 记录成层次化调用树**（metadata["tool_call_tree"] 存进上下文），前端渲染成"容器 + children"结构，用户能看到"哪一步在调用哪个专家"。

### 8.4 ⭐ 挖掘补充：SubAgent Execute 循环的四个细节（technical_analyst.go:201-588）

**细节 1：迭代上限是"实测数据"驱动的，不是拍脑袋**

```go
// MaxIterations 返回最大迭代次数
// 实测三只券对比分析约需 27 步，统一设为 40 步保留充足容错空间
func (ta *TechnicalAnalyst) MaxIterations() int {
	return 40
}
```

- 注释里直接写了**实测依据**："三只券对比分析约需 27 步"→ 设 40 步留容错
- **面试点**：Agent 迭代上限不是模型参数，是**用真实业务的复杂度测出来的**——你先跑通典型场景数步数，再乘 1.3~1.5 倍留余量
- 对比：Orchestrator 只有 12/18 轮（它是调度者，不干重活），专家 27 步、上限 40——**角色复杂度不同，上限不同**

**细节 2：双会话事件路由——"SSE 发主会话，上下文存子会话"**

```go
// buildCallbacks（technical_analyst.go:416-535）
callbacks := types.EngineCallbacks{
	// 文本事件：live 推送带 subagent_id（= agentName），前端归入对应子 Agent 容器
	OnTextDelta: func(content string) {
		ta.sse.SendTextDelta(content, ta.Type())  // subagent_id = "technical_analyst"
	},
	// ...
	// 上下文保存到子会话（不是主会话）！
	SaveContext: func(messages []contextx.Message) error {
		return ta.saveChildContext(childConversationID, messages, engine)
	},
}
```

| 事件 | 流向 | 为什么 |
|------|------|--------|
| **SSE 文本/工具事件** | → 主会话（带 `subagent_id`） | 前端按 subagent_id 归入对应专家容器展示 |
| **上下文快照** | → 子会话（childConversationID） | 子 Agent 的内部状态不影响主会话历史 |

**细节 3：子会话上下文"可能被重建"（loadChildContext）**

```go
// 每次 Execute 的入口：
// 1. 尝试从 ContextService 加载子会话历史快照
snapshot, err := ta.contextService.GetLatestContext(ctx, childConversationID, tenantID)
if err != nil || snapshot == nil {
	// 首次执行 → 全新上下文（system prompt + taskDescription）
	return []contextx.Message{...}, nil
}
// 2. 快照存在 → 恢复历史 + 每轮刷新 system prompt + 追加当前任务
```

- **设计意图**：虽然 `SubAgentTool` 每次调用都创建**新的** child ID，但 `loadChildContext` 仍支持"**已有子会话被重用/上下文重建**"的场景（如断线恢复、重试）
- 快照存在时走 `恢复历史 + 刷新 system prompt + 追加当前任务` 三步骤——**执行引擎天然支持"续跑"而不只是"从零跑"**
- **面试点**：子 Agent 无状态 ≠ 无持久化。状态不是存在内存里，是存在 ContextService（DB）里——内存无状态 + 存储可重建，是 Agent 可恢复性的正确姿势

**细节 4：唯一线程级状态隐患（值得观察）**

```go
// Execute 开头保存主会话 ID 到结构体字段
ta.mainConversationID = mainConversationID.String()
```

- SubAgent 的 `mainConversationID` 是**实例字段**，不是 Execute 局部变量
- 若同一个 `TechnicalAnalyst` 实例被**并发**调用（两个 goroutine 同时 Execute），存在字段竞争的理论风险
- BondSeek 当前架构下通常一个会话一个 Agent 实例（串行执行），所以不触发；但哨兵值 `ta.mainConversationID` 在报错路径里的使用值得后续跟踪

---

## 9. 与通用多 Agent 范式对照

| 维度 | BondSeek AI Studio | 通用 Supervisor 模式 | 朴素多 Agent（各干各的，无编排） |
|------|-------------------|---------------------|--------------------------------|
| 调度决策 | LLM Function Calling 自主 + Prompt 硬规则兜底 | LLM 自主（靠提示词） | 无 |
| 上下文 | 每次子任务全新子会话，互不污染 | 共享/隔离看实现 | 各自独立 |
| 成果共享 | 工作区共享（文件跨专家可读） | 靠显式传参 | 无 |
| 结果传递 | 摘要 + 文件列表（瘦身） | 常拼接完整输出 | 各自返回 |
| 循环实现 | 同一 ExecutionEngine（配置差） | 各 Agent 独立实现 | 各 Agent 独立 |
| 并行 | 提示词显式鼓励并行 | 视实现 | 无 |
| 失败保护 | 3 连败硬降级 + 宽容完成 + 预算熔断 | 多靠超时 | 无 |
| 权限 | 技能按 scope 分层 | 无明确分层 | 无 |
| 嵌套深度 | 3 层（编→专家→二级专家） | 2 层为主 | 1 层 |

**一句话**：BondSeek 把"编排"做成了**一套可以递归复用的体系**——同一个引擎、同一套工具包装、同一套防错机制，用"配置差异"（角色 + 工具 + 提示词 + 轮数）构建出不同层级的 Agent；再用"会话隔离 + 工作区共享"解决多 Agent 协作的两大矛盾，用"摘要瘦身"解决上下文爆炸。

---

## 反模式（本主题避免）

- **监工自己干活**——Orchestrator 承担业务处理会破坏分工，角色边界写死在系统提示词第一段
- **子 Agent 全量输出给监工**——上下文会被副产物撑爆，只回摘要+文件路径
- **共享对话历史**——子任务必须全新子会话，否则上轮状态污染下轮判断
- **不共享劳动成果**——会话隔离 ≠ 文件隔离，生产文件放共享工作区让下游专家直接读
- **给模型暴露不可用工具**——webSearch/RAG 按开关和可用性条件注册
- **迭代上限 = 失败**——宽容完成：有产出文件就算干完，别把"没跑满 100 分"当"挂科"
- **调用专家时发叙述性文本**——只有最后整合输出，过程别刷屏干扰用户

---

## 📖 名词标注表（完整中英对照）

> 本文（多 Agent 编排系统）全部关键英文术语，看懂这张表 = 读懂本文。

| 英文 | 中文理解 | 详细解释 |
|------|---------|---------|
| **Multi-Agent** | 多智能体 | 多个 AI 角色分工协作的系统 |
| **Agent** | 智能体 | 一个能自主思考和行动的 AI 角色（有 LLM + 工具 + 循环） |
| **Orchestrator** | 编排器/监工 | 总指挥 Agent：理解意图、委派、整合，不干活 |
| **SubAgent** | 子智能体/专家 | 专门领域的 Worker，有自己工具集和系统提示词 |
| **Supervisor** | 监督者/监工模式 | 编排架构：一个监工 + 多个工人的合作模型 |
| **Worker** | 工人 | 执行具体任务的 Agent（= SubAgent） |
| **Task / Task Description** | 任务 / 任务描述 | 派给专家的"工作单"（唯一必要的传参） |
| **Function Calling** | 函数调用 | LLM 输出 JSON 决定调哪个工具、传什么参数 |
| **Tool Wrapping** | 工具化包装 | 把 SubAgent 包成 Tool 让 LLM 用函数调用机制调度 |
| **Tool Registry** | 工具注册表 | 角色能用哪些工具的登记册（每个 Agent 一个） |
| **ReAct** | 思考-行动循环 | Reason（推理）→ Action（行动）→ Observation（观察）循环 |
| **ExecutionEngine** | 执行引擎 | 循环逻辑的统一实现，所有 Agent 复用 |
| **Iteration** | 迭代轮数 | Agent 循环最多跑多少轮（flash=12 / deep=18） |
| **Session / Conversation** | 会话 | 对话容器（子会话 = 每次调用的全新对话） |
| **子会话 (Child Conversation)** | 子对话 | SubAgent 每次调用创建的全新会话（无状态） |
| **Main Conversation** | 主会话 | 用户的原始对话（事件记录都写这里） |
| **Workspace** | 工作区 | 共享文件目录（所有子 Agent 的人都在这放产出） |
| **隔离 (Isolation)** | 互不污染 | 子任务会话独立，状态不串 |
| **摘要瘦身 (Summary Only)** | 只回摘要 | 子 Agent 只回结论+文件列表，不回全量输出 |
| **llm_content** | 给模型看的内容 | 工具结果 Metadata 里标记"这段要进 LLM 上下文" |
| **DynamicPromptRefresher** | 动态提示词刷新器 | 每轮调用前重拼系统提示词的函数 |
| **Prompt Cache** | 提示词缓存 | 静态前缀稳定性带来的 LLM 缓存命中（省 token） |
| **SubAgent Zone** | 专家清单区段 | 系统提示词里自动生成的"专家列表"章节 |
| **Skill Scope** | 技能作用域 | 技能归属哪个角色（aistudio / 各专家 scope） |
| **scopeExpertMapping** | 专家路由表 | scope → SubAgent 的映射，决定技能派给谁 |
| **task_description 指 示** | 委派指令 | 监工在任务描述开头指示专家自己 load_skill |
| **Middleware** | 中间件 | 横切逻辑（预算/审计/注入），钩子挂接不进循环体 |
| **BudgetMiddleware** | 预算熔断器 | token 超限强制 tool_choice="none" 收尾 |
| **tool_choice** | 工具选择强制 | 强制 LLM 调/不调工具（"none"=禁止） |
| **指数退避 (Exponential Backoff)** | 逐次加时重试 | 失败后 200→400→600ms 重试，最多 3 次 |
| **硬降级 (Degradation)** | 强制降级 | 工具 3 连败被拦截 + 注入"停止重试"指令 |
| **_break_loop** | 提前退出标记 | 工具结果带此标记则立即结束循环（Success） |
| **宽容完成 (Lenient Complete)** | 有产出即成功 | 迭代上限但有产出文件 = 成功 |
| **ToolCallRecorder** | 工具调用记录器 | 记录层次化调用树（前端渲染容器结构用） |
| **SubAgentTree** | 子 Agent 执行树 | 子 Agent 内部步骤/产出文件/耗时的结构化记录 |
| **recordEvent** | 事件记录函数 | 把 SSE 事件写入历史表（子 Agent 也注入） |
| **Cross-Module Dispatch** | 跨模块路由 | dispatch 到 CodeCopilot / QuantLab 等其他 Agent 模块 |
| **ReuseSubConversation** | 子会话复用开关 | 复用(false=每次都新建) 降低状态污染 |

---

## 关联文档

- [`case-study-bondseek-rag.md`](./case-study-bondseek-rag.md) — RAG 检索（SubAgent 的工具集里有 analyze_documents 检索工具）
- [`case-study-bondseek-memory.md`](./case-study-bondseek-memory.md) — 记忆系统（对话结束异步发布 ingest 事件）
- [`case-study-bondseek-user-memory.md`](./case-study-bondseek-user-memory.md) — L1 画像注入（仅在 trigger=user 时注入 system prompt）
- [`../topics/agent-frameworks/compare-langchain-langgraph.md`](../topics/agent-frameworks/compare-langchain-langgraph.md) — 框架对比（LangGraph 也支持 Supervisor 模式）
- [`notes/agent-interview-questions.md`](../notes/agent-interview-questions.md) — 面试话题索引（第 9 题）
