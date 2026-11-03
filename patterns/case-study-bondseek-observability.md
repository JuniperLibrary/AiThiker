---
title: BondSeek 可观测性体系（审计日志 + 调用树 + 指标全解析）
tags: [case-study, observability, audit-log, tool-call-tree, metrics, tracing, clickhouse, golang]
created: 2026-10-21
updated: 2026-10-21
status: draft
---

# BondSeek 可观测性体系（审计日志 + 调用树 + 指标全解析）

> **案例对象:** [bondseek-ai](https://github.com/) — 债券行业多租户 AI SaaS 投研平台（Go 1.24 微服务）
> **分析范围:** Agent 系统可观测性的完整实现：审计日志（请求/记忆双线）/ 工具调用树（层次化链路）/ 指标与统计（ClickHouse）/ 结构化日志（zap）
> **资料代码:** `bondseek-common/middleware/audit_logger.go` + `rag/maintenance/phase5_stats.go` + `aistudio/core/tool_call_recorder.go` + 各模块 recordEvent 埋点（2026-10-21 代码级深挖）
> **关联文档:** [多 Agent 编排](./case-study-bondseek-multi-agent.md)、[用户记忆专项](./case-study-bondseek-user-memory.md)
> **面试话题索引:** [agent-interview-questions.md](../notes/agent-interview-questions.md) — 第 12 题"Agent 系统怎么做可观测性"（本文即答案素材）

## 📌 名词速查表（先看这里）

> 本文讲可观测性，核心是"审计、追踪、指标、日志"。先看这里（完整对照见文末"名词标注表"）：

| 英文 | 读作/理解成 | 一句话解释 |
|------|------------|-----------|
| **Observability** | "可观测性" | 出了问题能查出"为什么"的能力（日志/追踪/指标三件套） |
| **Audit Log** | "审计日志" | 谁在什么时候做了什么操作的记录（不可抵赖） |
| **Trace** | "追踪" | 一次请求在系统里走了哪些步骤的链路 |
| **Metric** | "指标" | 可量化的数字（延迟、次数、覆盖率） |
| **Structured Log** | "结构化日志" | 带 key-value 字段的日志（可检索可统计） |
| **Tool Call Tree** | "工具调用树" | Agent 里每次工具调用的层级关系树 |
| **ClickHouse** | "时序数据库（专有名词）" | 专跑时间序列统计的数据库 |
| **EventBus** | "事件总线（专有名词）" | 服务间广播事件的通道（NATS） |
| **Debug Info** | "调试信息" | 检索过程产生的中间诊断数据 |

---

## 🎯 TL;DR 冲突摘要（先看这段）

> **这篇文档解决的核心矛盾：Agent 系统是"黑盒中最黑的黑盒"——一次回答的背后是几十次 LLM 调用、工具调用、子 Agent 调度，出错了你根本不知道是哪一环。** 传统后端看日志就行，Agent 系统需要看的是"**模型为什么这么答**"——这要求不仅在系统层面打日志，还要记录**决策过程本身**（调用了哪个工具、检索了多少资料、幻觉率多高）。BondSeek 的破局思路一句话总结：**"可观测性分两层——系统层用审计日志和指标看'服务健康'，Agent 层用调用树和调试信息看'AI 决策'，两层叠加才叫 Agent 可观测性。"** 读完全文你只需要带走一个画面：**一次对话 = 一条带完整调用的"决策链路"，每个 AI 决策点（检索/打分/重试）都留痕，出问题能从头查到尾。**

---

## 1. 可观测性的五根支柱（全景）

| 支柱 | BondSeek 实现 | 回答什么问题 |
|------|--------------|------------|
| **请求审计** | `AuditLoggerMiddleware`：记录每个 API 请求的完整信息 | 谁调了什么接口？响应多慢？ |
| **记忆审计** | `t_agent_memory_history` / `t_agent_memory_drops`：记忆变更全程留痕 | 哪条记忆被谁改成什么样？为什么被丢弃？ |
| **调用树** | `ToolCallRecorder`：引擎每步工具调用记录成树 | Agent 这次回答内部调了什么、顺序、耗时？ |
| **指标统计** | ClickHouse：检索命中、覆盖等时序统计 | 最近 QPS？检索质量趋势？ |
| **结构化日志** | zap：所有模块全链路 key-value 日志 | 任意环节发生了什么？上下文是什么？ |

---

## 2. 横切关注点 1：请求审计中间件（audit_logger.go）⭐

### 2.1 什么是审计日志中间件

一个 Gin 中间件，**拦截所有 HTTP 请求**，把请求信息发布到 EventBus（事件总线）→ 存储为审计记录：

```go
func AuditLoggerMiddleware(eb eventbus.EventBus, config *AuditConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// ... 收集请求信息：method/path/body/headers ...
		auditEvent := &event.AuditEvent{
			RequestID:    ...,  // 请求 ID
			TenantID:     ...,  // 租户
			UserID:       ...,  // 用户
			Action:       c.Request.Method + " " + c.Request.URL.Path,
			ResourceType: ...,  // 资源类型
			ResourceID:   ...,  // 资源 ID
			IPAddress:    ...,  // IP
			UserAgent:    ...,
			RequestBody:  ...,
			ResponseCode: ...,  // 响应码
			Duration:     ...,  // 耗时（毫秒）
			Error:        c.Errors.String(),
			Timestamp:    ...,
		}
		// 发布审计事件到总线 → 落库
		eb.Publish(context.Background(), constants.EventAuditLogCreated, ebEvent)
	}
}
```

### 2.2 三个值得学习的设计点

| 设计 | 解决什么问题 |
|------|------------|
| **异步发布**（`eb.Publish`） | 审计不阻塞主请求——审计是"副作用"，不能拖慢业务（呼应总纲哲学五：增强不拖垮主链路） |
| **脱敏**（`maskSensitiveData`） | 请求体里可能有密码/Token——审计也要防泄露（金融合规刚需） |
| **排除清单**（`shouldExclude`） | 健康检查等高频率无意义路径不打审计——防无用数据淹没 |

> **为什么审计用"事件"不用"直接写库"**：审计是**横切关注点**，不该耦合在业务代码里。发一个事件，由事件消费者统一落库——业务代码干净，审计逻辑集中，还能用 EventBus 的重试保证不丢。

---

## 3. 横切关注点 2：记忆审计（贯穿记忆案例）

> 记忆审计已在[用户记忆专项](./case-study-bondseek-user-memory.md)深挖，此处聚焦"可观测性视角"再看一次：

| 审计表 | 记录什么 | 查询用途 |
|--------|---------|---------|
| `t_agent_memory_history` | L1 每次变更：`event ∈ {ADD,UPDATE,DELETE,MERGE}` + `triggered_by` + `old/new_content` | "这条记忆怎么来的、谁改的、为什么改" |
| `t_agent_memory_drops` | 四道闸丢弃：`drop_reason ∈ {short_conv, threshold:*, blacklist:*, value_score:*}` | "为什么这条没被记住"（排查记忆系统失灵） |

**可观测性视角的金句**：**记忆系统必须"连丢弃都留痕"**——你敢丢一条记忆，就得证明你"为什么敢丢"。这在医疗/金融级 Agent 是合规底线，也是排查"记忆为什么不生效"的唯一途径。

---

## 4. 横切关注点 3：工具调用树（ToolCallRecorder）⭐⭐

### 4.1 为什么 Agent 需要"调用树"而不是"调用日志"

普通日志记录的是"一条平铺的事件流"：

```
tool_start(technical_analyst)
tool_start(read_file)
tool_start(data_analyst)
tool_complete(data_analyst)
tool_complete(read_file)
tool_complete(technical_analyst)
```

**问题**：平铺日志看不出**层级关系**——read_file 是 orchestrator 调的，还是 technical_analyst 调的？data_analyst 是几级嵌套？

BondSeek 用 `ToolCallRecorder` 记录**层次化调用树**：
- 普通工具 → `RecordToolCall`（挂到当前节点）
- SubAgent 工具 → `RecordSubAgentCall`（子 Agent 的树挂到父节点）
- 结果存 `metadata["tool_call_tree"]` → 前端渲染成"容器 + children 树"（用户能看到"哪一步调用了哪个专家"）

### 4.2 调用树长什么样（用户视角）

```
└─ Orchestrator
   ├─ tool: technical_analyst
   │   ├─ tool: data_analyst（二级）
   │   └─ tool: read_file
   ├─ tool: visualization_expert
   └─ tool: dashboard_creator
```

**前端价值**：用户不是在浏览器看"黑盒"，而是能看到 AI 分几步、每步干什么——这既是可观测性，也是**信任构建**。

### 4.3 调用树的三个可观测性用途

| 用途 | 具体问题 |
|------|---------|
| **故障定位** | "哪个工具卡了 30 秒"——树的节点带 `duration_ms` |
| **成本审计** | "这次回答调了几次 LLM"——树的节点数 = LLM 调用次数（可换算 token 成本） |
| **质量复盘** | "为什么答得不好"——看调用树哪里绕了远路、哪里失败了 |

> **面试金句**：Agent 可观测性区别于传统系统的核心是——**调用树记录了"AI 的思考过程"（调了谁、为什么调、结果如何），而传统日志只记录"系统的执行过程"。**

---

## 5. 横切关注点 4：指标与统计（ClickHouse）

### 5.1 ClickHouse 的角色

- **时序指标库**：存检索命中统计、覆盖统计等**随时间变化的数字**
- **SummingMergeTree 表引擎**：相同 key 的插入自动合并求和（写吞吐高、查询快）
- **月度清理**（phase5_stats.go）：超 90 天统计删除 + `OPTIMIZE TABLE` 合并碎片分区——**指标也有生命周期，不能无限膨胀**

```go
// phase5_stats.go —— 月度维护第 5 阶段：清理过期统计
func (r *Runner) phaseStats(ctx context.Context, runMonth string) error {
	retainDays := r.config.StatsRetentionDays  // 默认 90 天
	if err := r.clickHouseStore.CleanOldStats(ctx, retainDays); err != nil { ... }
	if err := r.clickHouseStore.OptimizeTable(ctx); err != nil { ... }  // 合并碎片
}
```

### 5.2 Agent 业务里的"指标化"思想

| 量化对象 | 变成什么指标 | 在哪埋点 |
|---------|------------|---------|
| 检索命中 | `recordRetrieveHits`（chunk/entity/relation 命中数） | RAG service |
| 检索质量 | 引用覆盖率（CitationCoverage） | 幻觉检测 |
| 反思轮数 | `debug.ReflectionRounds++` | 自反思循环 |
| 工具执行 | `duration_ms`（每次工具调用） | ToolCallRecorder |
| 记忆丢弃 | drop_reason 计数 | 记忆四道闸 |

**核心思想**：**一切可量化的行为都变成指标**——覆盖 AI 决策的每一步，让"这次回答到底是好是坏、贵不贵、病在哪"都可以用数据回答。

---

## 6. 横切关注点 5：RAG 检索调试信息（QueryDebugInfo）

RAG service 的检索全程构建 `debug` 对象，随响应返回：

```go
debug.RetrievedChunks    // 检索到多少 chunk
debug.RetrievedEntities  // 检索到多少实体
debug.RetrievedRelations // 检索到多少关系
debug.ReflectionRounds   // 自反思转了几轮
coverageResult           // 引用覆盖率
confidence               // 置信度（幻觉检测输出）
```

**这是 RAG 特有的可观测性**：不仅看"返回了什么结果"，还看"检索过程的每个阶段数字"——**这正是 Agent 系统区别于普通 API 的地方：中间过程本身就是诊断数据。**

---

## 7. 结构化日志：贯穿全链路的基础设施

| 规范 | BondSeek 要求 |
|------|--------------|
| 库 | `go.uber.org/zap`（禁止 `fmt.Println`/`log.Println`） |
| 关键操作 | INFO（登录/创建/更新/删除） |
| 异常 | ERROR（含堆栈） |
| 上下文 | 日志必须含 `user_id` / `tenant_id` / `request_id` |
| 敏感信息 | **禁止**输出 Token / API Key / 密码 |

```go
logger.Info("user created",
	zap.String("user_id", userID.String()),
	zap.String("username", username),
	zap.Duration("duration", elapsed))
```

**面试点**：结构化日志是可观测性的"地基"——所有高级观测（追踪/审计/指标）都建立在"日志带了足够的上下文字段能关联起来"这个前提上。

---

## 8. 一句话总结：Agent 可观测性的三层模型

```
┌─ 第三层 · Agent 决策观测（AI 专属）───────────┐
│  工具调用树 + 检索调试信息 + 反思轮数 + 置信度    │
│  → 回答"AI 为什么这么答"                       │
├─ 第二层 · 业务事件观测 ───────────────────────│
│  请求审计 + 记忆审计 + 业务指标                  │
│  → 回答"系统做了什么业务操作"                   │
├─ 第一层 · 基础设施观测（所有系统通用）───────────│
│  zap 结构化日志 + ClickHouse 指标               │
│  → 回答"服务健康吗、性能如何"                   │
└────────────────────────────────────────────────┘
```

**三层齐全才是 Agent 可观测性**——普通系统做到第一层就够，Agent 系统必须做到第三层，因为**"AI 的决策过程"是它最大的不确定性，也是出问题时的第一嫌疑对象。**

### 面试回答框架（30 秒版）

> "我们的 Agent 可观测性分三层。**基础设施层**：zap 结构化日志，统一带 user_id/tenant_id/request_id，禁止打敏感信息。**业务事件层**：审计日志走中间件 + EventBus 异步落库（请求审计 + 记忆变更/丢弃审计），外加 ClickHouse 指标（检索命中、覆盖率每月清理）。**Agent 决策层**：这是 Agent 系统特有的——`ToolCallRecorder` 记录层次化工具调用树（哪个专家调了哪个工具、耗时多少，前端渲染成调用链可视化），RAG 全程生成 QueryDebugInfo（检索了多少、反思几轮、幻觉率、置信度）。三层合起来，一次回答的来龙去脉可以完整回放——这在排查'AI 为什么答错'时是命根子。"

---

## 反模式（本主题避免）

- **只打平铺日志**——Agent 必须记录层级调用树，平铺日志看不出嵌套关系
- **审计直接写库阻塞请求**——审计必须异步（EventBus），不能拖慢业务
- **审计不做脱敏**——请求体里可能有密码 Token，金融场景合规大忌
- **记忆丢弃不留痕**——"敢丢"必须"可证"，丢弃审计是排查记忆失灵的唯一途径
- **指标无限膨胀**——ClickHouse 统计也要设保留期 + 合并碎片
- **日志不带上下文**——没有 user_id/tenant_id 的日志无法串联一次对话全链路

---

## 📖 名词标注表（完整中英对照）

> 本文（可观测性）全部关键英文术语，看懂这张表 = 读懂本文。

| 英文 | 中文理解 | 详细解释 |
|------|---------|---------|
| **Observability** | 可观测性 | 从外部输出推断内部状态的能力 |
| **Telemetry** | 遥测 | 自动采集系统运行数据（日志/指标/追踪总称） |
| **Audit Log** | 审计日志 | 记录"谁在何时做了什么"（强调合规与追溯） |
| **AuditEvent** | 审计事件 | 一条审计记录的载体（发到 EventBus） |
| **Trace / Tracing** | 追踪 | 一次请求跨服务/步骤的链路跟踪 |
| **调用树 (Call Tree)** | 调用树 | 工具调用的层级结构（父-子-孙） |
| **ToolCallRecorder** | 工具调用记录器 | 记录工具调用树的组件 |
| **SubAgentTree** | 子 Agent 执行树 | 子 Agent 内部步骤/产出/耗时的结构化记录 |
| **Metric** | 指标 | 可量化的性能/质量数字 |
| **ClickHouse** | 时序数据库（专有名词） | 存时序统计指标的列式数据库 |
| **SummingMergeTree** | 求和合并树（专有名词） | ClickHouse 表引擎，同 key 自动合并累加 |
| **Structured Log** | 结构化日志 | key-value 形式日志（zap） |
| **脱敏 (Masking)** | 敏感信息打码 | 审计/日志前替换敏感字段 |
| **EventBus** | 事件总线 | 服务间异步事件广播（NATS） |
| **QueryDebugInfo** | 检索调试信息 | RAG 检索过程的中间诊断数据 |
| **recordRetrieveHits** | 命中记录 | 记录检索命中了多少 chunk/实体/关系 |
| **Coverage** | 覆盖率 | 回答中带引用的句子比例 |
| **Confidence** | 置信度 | 回答可信度（幻觉检测产出 0~1） |
| **Retention** | 保留期 | 数据保留多久（统计默认 90 天） |
| **OPTIMIZE TABLE** | 合并表 | 合并碎片分区的维护操作 |
| **request_id** | 请求 ID | 串联一次请求全链路日志的标识 |
| **横切关注点 (Cross-cutting)** | 横切逻辑 | 审计/日志等不属于业务本身但每个请求都要的逻辑 |

---

## 关联文档

- [`case-study-bondseek-multi-agent.md`](./case-study-bondseek-multi-agent.md) — 编排（调用树的前端渲染、recordEvent 埋点）
- [`case-study-bondseek-user-memory.md`](./case-study-bondseek-user-memory.md) — 记忆审计（history/drops 表）
- [`case-study-bondseek-hallucination.md`](./case-study-bondseek-hallucination.md) — 防幻觉（覆盖率/置信度/反思轮数也是观测指标）
- [`case-study-bondseek-rag.md`](./case-study-bondseek-rag.md) — RAG（RetrieveCallbacks 阶段回调和调试信息）
- [`notes/agent-interview-questions.md`](../notes/agent-interview-questions.md) — 面试话题索引（第 12 题）
