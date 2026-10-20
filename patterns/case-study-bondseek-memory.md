---
title: BondSeek 记忆与上下文系统案例分析
tags: [case-study, memory, context-management, memory-layered, rag, state-persistence, golang]
created: 2026-10-20
updated: 2026-10-20
status: draft
---

# BondSeek 记忆与上下文系统案例分析

> **案例对象:** [bondseek-ai](https://github.com/) — 债券行业多租户 AI SaaS 投研平台（Go 1.24 微服务，5 模块，生产级）
> **分析范围:** Context 管理（会话存储/组装/压缩/状态-对话分离）与 Memory 管理（L1/L2/L3 三层记忆 + RAG 领域知识）
> **资料来源:** `bondseek-common/context/`、`bondseek-common/memory/`、`bondseek-common/rag/`、`bondseek-agent/`、`BONDSEEK_MEMORY_SYSTEM_PROPOSAL.md`（2026-10-20 代码级深挖）
> **关联话题:** [Context 与 Memory 本体论](./topics/memory-and-state/context-vs-memory.md)、[会话压缩专题](./topics/memory-and-state/session-compression-in-agent-systems.md)

---

## 1. 问题描述

债券投研场景对记忆系统有四个硬约束，决定了设计走向：

1. **事实必须可审计**：用户持有组合、决策（"持有 230012 利率债 5000 万"）是金融事实，错了会亏钱——不允许 LLM 摘要模糊化，需要人可读、可编辑、可溯源。
2. **多租户 + 隔离**：tenant + user + workspace 三级隔离，必然要分区存储。
3. **上下文窗口有限**：长对话 + 工具调用 + RAG 文档争抢 120K 窗口，必须渐进压缩且不破坏工具调用对。
4. **SSE 流式交互**：对话中途刷新/断线要能恢复；写记忆不能被流式阻塞。

结论：通用范式（mem0 式"全量记忆向量化"）不满足审计与可编辑需求。BondSeek 选择了 **"结构化三层记忆 + 业务规则质量闸"**，向量检索仅保留在 RAG 知识通道。

---

## 2. Context 管理方案

### 2.1 双轨存储：展示历史 vs 工作上下文（状态-对话分离）

| 存储 | 内容 | 生命周期 |
|------|------|---------|
| PG `t_agent_conversation_history` | SSE 事件流，JSONB 分段（≤500 事件/段，`FOR UPDATE` 行锁） | **永不压缩**，前端展示 + 记忆抽取源 |
| PG `t_agent_context` + Redis(24h TTL) | 剪枝/压缩后的 **LLM 工作上下文快照** | 每轮 ReAct 迭代后同步，可从快照重建 |
| Redis open-block + replay-log | SSE 流式中间态：delta 实时进 Redis，封口后落 PG | 断线重连按 seq 补发，无缺口 |

**核心洞察**：状态与对话分离 = 把"要展示的完整历史"和"喂给模型的裁剪上下文"物理拆成两个存储。前者永不压缩（保真），后者随时可压（省预算）。这正对应理论框架中"状态层永不参与压缩决策"的原则。

### 2.2 上下文组装（agentcore/agent_def.go `DefaultExecute`）

```
system prompt = StaticZone(固定) + SubAgentZone(子Agent清单) + AgentZone(动态刷新)
              + MemoryZone(L1 用户画像 XML)
messages[0] = system prompt          # 每轮由 DynamicPromptRefresher 刷新（技能+plan summary）
messages[1] = <recalled_memories>    # L2/L3 记忆，独立第二条 system 消息
messages[n] = 用户消息(带时间前缀)    # [YYYY-MM-DD HH:mm]
```

统一链路：`LoadContext → ProcessContext(剪枝/压缩) → SaveContext`，RAG 侧复用 `getProcessedHistory` 走同一套裁剪。

### 2.3 两阶段渐进压缩（context/checker.go + pruner.go + compressor.go）

```
maxTokens          120000          # 可按模型覆盖
pruneThreshold     50% 用量        # 触发滑动窗口剪枝
compressThreshold  70% 用量        # 触发 LLM 摘要压缩
responseReserve    8000            # 回复预留
memoryBudget       5000            # 记忆注入预算
```

- **阶段1 剪枝**：滑窗保留最近 40K tokens（`adjustSplitIndexForToolPairs` 保护 assistant↔tool 调用对完整）+ 工具级裁剪（去重工具只留最后一次、`pruneableTools` 输出置 `[PRUNED]`、删除失败工具对、去重 assistant）。
- **阶段2 压缩**：保留最近 60K 不动，旧消息交 LLM 生成**五字段结构化摘要**（任务概述/当前状态/重要发现/下一步/需保留的上下文——借鉴 AgentScope SummarySchema，temperature 0.3）。
- **运行期熔断**：`BudgetMiddleware` 超预算时注入收尾提示 + `tool_choice=none`；工具连续失败 3 次硬降级。
- **预算拆分**：`SystemPromptTokens` 使系统提示词与对话历史的 token 预算独立。

> 与 [会话压缩专题](./topics/memory-and-state/session-compression-in-agent-systems.md) 对照：BondSeek 实现了理论中的"渐进式压缩 + 状态分离 + 工具对保护"，但在"压缩时 Agent 元认知对齐"（[SYSTEM DIRECTIVE] 标记）上较弱——它靠的是每轮重刷 system prompt 而非让 Agent 感知压缩。

---

## 3. Memory 管理方案

### 3.1 三层记忆（bondseek-common/memory/）

| 层 | 类型 | 存储 | 召回方式 | 质量闸 |
|----|------|------|---------|--------|
| **L1** | 用户画像 semantic | PG `t_agent_user_profiles`（category→tag→feature→value，唯一键防重） | 全量查 PG → XML 注入 **system prompt** | confidence≥0.85、黑名单、explicit/inferred 双通道 |
| **L2** | 情景记忆 episodic | sandbox RPC 落盘 `memory/facts.md`、`decisions.md`（tenant/user/workspace 隔离） | `MemoryManager` **LLM 按 query 筛选**（非向量） | importance≥0.85、主语必须为用户或其组合 |
| **L3** | 知识/纠错 knowledge | sandbox 全局 `_global_memory/memory/corrections.md`、`domain_knowledge.md`（跨用户共享） | 同上（corrections 倾向全量） | 3-Why 检验、Redis 全局写锁 |

**关键设计**：L2/L3 是人类可读 Markdown（`- [date] 事实 | importance: 0.9 | source: conv_xx | id: ep_xxxx`），经 sandbox RPC 读写（agent 与 sandbox 可分机部署）。这满足"可审计、可编辑、可溯源"——运维可直接改文件。

### 3.2 写入管线：四道质量闸（异步 SleeptimeWorker）

```
对话结束 → PublishIngestEvent(NATS agent.memory.ingest_requested)
  → SleeptimeWorker(异步)
    ① 用户开关 auto_memory_enabled + 短对话(<100字符)过滤
    ② L1 提取 LLM：confidence≥0.85 + 黑名单
    ③ MemoryManager.Write：L2/L3 纯提取，importance≥0.85
    ④ 价值评分 LLM：total≥8（跨会话价值/长期有效性/信号源/去会话独立）
  → errgroup 并行提交：
     线A：L1 dedup LLM(ADD/UPDATE/DELETE/NONE) → PG UPSERT + 审计表
     线B：MD dedup LLM(ADD/UPDATE/SKIP，保留旧 entry_id) → 整文件重写 .md
  → 成功才标记 memory_extracted=true
```

**防污染铁律**：任一闸失败 → **宁可整批不落库** 也不放行（价值评分失败等 Scheduler 10 分钟补偿重试，`maxCompensateAttempts=5` 后放弃）；dedup LLM 失败 → 跳过该层写入，不静默全 ADD。drops 审计表 `t_agent_memory_drops` 记录全部拒绝原因（short_conv/threshold/blacklist/value_score/dedup:skip）。

### 3.3 召回链路（对话前）

- L1：全量查 PG → 排序（非 stale 优先、confidence 降序）→ `<user_profile>` XML 注入 system prompt
- L2/L3：读 .md 文件 → `MemoryManager.Recall` LLM 按当前 query 从全文筛选 → `<recalled_memories>` 第二条 system 消息（token 预算默认 2000/全量 3000）
- 仅 `trigger=user` 且非 sub-agent 时执行；cron/context_init 跳过（Trigger 治理）

### 3.4 遗忘机制（forget）

| 机制 | 规则 | 实现 |
|------|------|------|
| L1 软遗忘 | inferred + confidence<0.7 + 90 天未更新 → `is_stale=true` | `RunMonthlyStaleCheck`（**需外部 cron 触发**）；召回时 stale 排最后 |
| L1 容量控制 | 单 tag ≥15 条 → LLM 合并 | `ProfileConsolidator` |
| 抽取放弃 | 补偿重试 ≥5 次 → 强制标记，停止重投 | `maxCompensateAttempts` |
| RAG 两级冷热 | 60 天无命中 → `is_active=false`（可重激活）；120 天 → Milvus/NebulaGraph **物理删除索引**，PG 永久保留 | 月度维护 Phase3（ClickHouse 命中统计，保留 180 天） |
| TTL | Redis 上下文快照 24h、RAG LLM 缓存 24h | — |

> 最接近 canonical "遗忘"的是 RAG 冷热冻结；L1 只有 stale 标记而非删除，且月度任务未自动挂 cron——这是"设计 vs 实现"差距的典型。

### 3.5 RAG 领域知识记忆（bondseek-common/rag/，与三层记忆正交）

- **存储**：Milvus 三集合 `rag_chunks/rag_entities/rag_relations`（COSINE、1024 维）+ NebulaGraph 图（space `bondseek_rag`、EDGE `relates_to`）+ PG 真相库/证据映射 + ClickHouse 热度
- **Fast 检索**：query 重写(StepBack+≤5 子问题×3 变体) → embedding → **三路并行 Milvus**（chunk 多样性分桶 15/10/10、entity/relation top20）→ **RRF 融合(k=60)** → **DashScope Rerank 精排 top30** → Decay Ranker（新闻类 7 天满分/30 天半衰）；partition_key 按月裁剪（≤24 月、默认回看 3 个月）
- **Deep 检索**：+ NebulaGraph LLM 引导多跳（≤5 跳、每跳≤3 边、候选≤20）+ 自反思循环（充分性检查≤2 轮补检 → 幻觉检测阈值 0.2 → 有用性评估）
- **明确不依赖 Milvus 的是三层会话记忆**——记忆的记忆存储是 PG 行 + Markdown，向量检索只属于 RAG 通道

---

## 4. 权衡分析（vs 通用范式）

| 维度 | BondSeek 选择 | 通用范式(mem0/MemGPT) | 为什么 |
|------|--------------|----------------------|--------|
| 会话记忆存储 | PG 行 + 可读 Markdown | 统一 vector store | 金融事实需审计/人工编辑，向量不可读 |
| 记忆召回 | L1 全量注入 + L2/L3 **LLM 筛选** | embedding 相似度 | LLM 审阅全文保证事实准确，代价是每轮 +1 次 LLM 调用 |
| 写入质量 | 四道业务规则闸 + 失败整批放弃 | 阈值/去重较宽松 | 坏记忆注入成本 > 漏存成本（宁可漏存不可错存） |
| 遗忘 | stale 标记 + RAG 冷热物理删除 | 时间衰减/LRU | 金融数据需保留可回溯（PG 永存），索引可删 |
| 记忆隔离 | tenant+user+workspace 三级 | 多租户字段 | SaaS 基本盘 |
| 记忆获取 | 对话结束后异步抽取 | 实时/异步混合 | 不阻塞 SSE 流式；代价是即时性差 |

**代价**：每轮最多 6-7 次 LLM 调用（召回 1 + 提取 2 + 评分 1 + dedup 2 + 合并 0-1）——token 成本显著高于纯向量方案；L2/L3 依赖 sandbox RPC，sandbox 不可用则 ingest 失败。

---

## 5. 提炼：四个可复用模式

1. **状态-对话分离双轨存储**：展示历史（永不全压）+ 工作上下文（随时可压）物理拆分
   → 适用：任何流式、长会话、需要历史可回看的 Agent 系统
   → 实现：`t_agent_conversation_history` / `t_agent_context` + Redis 快照

2. **结构化三层记忆 + 业务规则质量闸**：L1 画像(PG) / L2 情景 / L3 知识，写入全部过 4 道闸且失败整批放弃
   → 适用：事实准确性要求高的领域（金融/医疗/法律）
   → 实现：`memory/worker/sleeptime.go` 异步管线 + drops 审计表

3. **候选 + 价值评分分离**：先 LLM 纯提取候选，再审阅打分（≥8才落库），提取与落库解耦
   → 适用：决定"什么值得记"比"怎么记"更重要的场景
   → 实现：`MemoryManager.Write`(候选) + `filter/value_scorer.go`(评分)

4. **RAG 两级冷热遗忘**：60 天/120 天无命中 → 软失活/硬删除索引（PG 真相永存）
   → 适用：索引可重建(embedding/图)、真相必须保留的知识库
   → 实现：`rag/maintenance/phase3_cold_hot.go`

---

## 6. 已知缺口（设计 vs 实现差距）

| 缺口 | 状态 | 影响 |
|------|------|------|
| `ContextStorage.VectorizeAndStore`（压缩消息向量化） | TODO 桩 | 压缩后的早期上下文无法语义检索 |
| `HistoryRetriever`（Milvus 对话历史检索 0.7/0.3 混合） | TODO 桩 | 长程对话历史检索缺席 |
| `SummaryService`（会话级摘要，写 `t_agent_conversations.summary`） | 未接线 | 无会话级摘要兜底 |
| ClickHouse 记忆指标表 | 未实现 | 无法观测 ingest 质量 |
| 月度 stale / L2 整理任务 | 需外部 cron | 遗忘机制依赖外部调度 |

---

## 关联文档

- [`topics/memory-and-state/context-vs-memory.md`](../topics/memory-and-state/context-vs-memory.md) — 理论框架（本文是其实践案例）
- [`topics/memory-and-state/session-compression-in-agent-systems.md`](../topics/memory-and-state/session-compression-in-agent-systems.md) — 压缩策略理论
- `topics/agent-architecture/index.md` — 记忆在架构中的位置

---

> **思考：** BondSeek 证明了"业务规则优先 vs 通用范式"的取舍张力。当领域要求事实可审计时，牺牲 LLM 调用成本换取确定性闸门是合理的；但当"记住什么"由 LLM 判断时，质量闸就成为唯一防线——这也是为什么失败宁可整批不落。真正的通用记忆系统，需要在"向量化检索的表达力"与"结构化规则的确定性"之间找到混合路径。
