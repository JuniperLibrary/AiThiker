# Patterns — Agent 设计模式

**路径:** `patterns/`
**作用:** 记录可复用的 Agent 设计模式、架构模板和常见问题的标准化解决方案。

## 范围

- 认知架构模式（ReAct、Plan-and-Execute、Reflection 等）
- 通信模式（Agent-to-Agent、Human-in-the-Loop）
- 错误恢复与容错模式
- 状态管理与持久化模式

## 约定

- 每个模式一个独立文件: `pattern-name.md`
- 包含：问题描述 → 解决方案 → 权衡分析 → 代码示例
- 与 `topics/` 中的相关话题交叉引用
- 真实项目案例分析使用前缀: `case-study-项目名-主题.md`

## 内容索引

| 文件 | 主题 |
|------|------|
| [`case-study-bondseek-memory.md`](./case-study-bondseek-memory.md) | BondSeek 生产级记忆与上下文系统：三层记忆、质量闸写入、双轨存储、冷热遗忘 |
| [`case-study-bondseek-user-memory.md`](./case-study-bondseek-user-memory.md) | BondSeek 用户记忆（L1 画像）专项：tag 枚举、explicit/inferred 分层、dedup 博弈、触发时机 |
| [`case-study-bondseek-rag.md`](./case-study-bondseek-rag.md) | BondSeek RAG 检索系统：Query 四步扩展、三路并行召回、RRF 融合、Rerank 精排、时间衰减、分区裁剪 |
| [`case-study-bondseek-multi-agent.md`](./case-study-bondseek-multi-agent.md) | BondSeek 多 Agent 编排：Supervisor 模式、SubAgent 工具化包装、统一 ExecutionEngine、动态提示词、技能分层路由 |
| [`case-study-bondseek-sse-recovery.md`](./case-study-bondseek-sse-recovery.md) | BondSeek SSE 断线恢复：事件序号(seq)驱动、Replay Log 回放、水位线去重、断线自动补洞 |
| [`case-study-bondseek-hallucination.md`](./case-study-bondseek-hallucination.md) | BondSeek 防幻觉体系：自反思三重循环（充分性/幻觉检测/有用性）+ 引用治理 + 重新生成 |
| [`case-study-bondseek-observability.md`](./case-study-bondseek-observability.md) | BondSeek 可观测性：审计日志 + 工具调用树 + ClickHouse 指标 + 结构化日志三层模型 |
| [`case-study-bondseek-microservices.md`](./case-study-bondseek-microservices.md) | BondSeek 微服务拆分：5 服务拆法（4 服务 + 1 共享库）、数据所有权（表前缀）、EventBus 快慢线、网关 ForwardAuth |

> 💡 **先读总纲再读案例**：`../topics/agent-architecture/from-zero-to-agent.md` — 用"一次对话的完整旅程"把以上案例串成一张图，附 5 大设计哲学卡片。

## 关联矩阵（理论 ↔ 案例 ↔ 框架）

> 阅读案例时，对照本矩阵快速定位对应的理论知识章节。

| 案例中的设计 | 对应的理论知识 | 对应框架概念 |
|-------------|--------------|-------------|
| **双轨存储**（展示历史永不压缩 vs 工作上下文随时可压） | [`memory-and-state/context-vs-memory.md`](../topics/memory-and-state/context-vs-memory.md) §3.2 分层组装 | LangGraph Checkpoint（持久化状态） |
| **两阶段渐进压缩**（50% 剪枝 / 70% 压缩） | [`memory-and-state/session-compression-in-agent-systems.md`](../topics/memory-and-state/session-compression-in-agent-systems.md) §1.2 渐进式压缩 | LangGraph 状态裁剪 |
| **L1 用户画像注入 system prompt**（XML 全量注入） | [`context-vs-memory.md`](../topics/memory-and-state/context-vs-memory.md) §4.2 生命周期·召回 | LangChain Memory 类（外挂记忆） |
| **四道质量闸写入**（失败整批放弃） | [`case-study-bondseek-user-memory.md`](./case-study-bondseek-user-memory.md) §3 提取管线 | mem0 阈值过滤 |
| **冷热遗忘**（60/120 天物理删索引） | [`context-vs-memory.md`](../topics/memory-and-state/context-vs-memory.md) §4.2 生命周期·遗忘 | 无直接对应（业界弱项） |
| **触发治理**（仅 user 触发，cron 跳过） | [`case-study-bondseek-user-memory.md`](./case-study-bondseek-user-memory.md) §8 触发时机 | 无直接对应（业务定制） |
| **Query 四步扩展**（重写/抽象/分解/变体） | 无直接理论章节（建议新建 `topics/planning-and-decomposition/`） | LangChain MultiQueryRetriever / HyDE |
| **三路并行召回 + RRF 融合** | 无直接理论章节（建议新建 `topics/knowledge-representation/` 或 evaluation） | LangChain EnsembleRetriever（RRF） |
| **分区裁剪 + Decay 时间衰减** | [`context-vs-memory.md`](../topics/memory-and-state/context-vs-memory.md) §4.2 生命周期·遗忘（时效性思想同源） | Milvus partition_key + rerank function |
| **SubAgent 工具化包装**（每次全新子会话 + 只回摘要） | 无直接理论章节（建议新建 `topics/multi-agent-systems/`） | LangGraph Supervisor / LangChain Tool-Executor |
| **统一 ExecutionEngine + 动态提示词**（同引擎多角色 + 静态保缓存） | 无直接理论章节（建议新建 `topics/multi-agent-systems/`） | LangGraph 状态图 / Anthropic multi-agent research |
| **技能分层路由**（scope 权责分离 + task_description 委派） | [`case-study-bondseek-user-memory.md`](./case-study-bondseek-user-memory.md) §4 dedup（同为"分层可信"思想） | 无直接对应（业务定制） |
| **seq 水位线回放**（事件序号驱动断线恢复） | 总纲 [`from-zero-to-agent.md`](../topics/agent-architecture/from-zero-to-agent.md) ⑧（same 机制系列） | SSE / EventSource 重连语义 |
| **自反思三重循环**（充分性→幻觉→有用性，不合格返工） | 总纲 [`from-zero-to-agent.md`](../topics/agent-architecture/from-zero-to-agent.md) ⑦ 防失败 | Reflection 认知架构 / Self-Refine |
| **引用全程治理**（强制带引用→清洗→核对） | 总纲 哲学一（LLM 不可信 → 规则兜底） | RAG 引用 / grounded generation |
| **数据所有权拆分**（表前缀 + Redis 前缀 + 模型目录三层钉死边界） | 无直接理论章节（建议新建 `topics/agent-architecture/` 微服务专题） | DDD 限界上下文 / 数据库 per-service |
| **EventBus 快慢线**（FastLane 内存实时 vs SlowLane 持久化可靠） | 总纲 [`from-zero-to-agent.md`](../topics/agent-architecture/from-zero-to-agent.md) ⑩（红线纪律系列） | NATS JetStream / Kafka 分层存储 |
| **网关 ForwardAuth 收敛认证**（system 唯一验 JWT，服务只读请求头） | 无直接理论章节 | K8s Gateway API ForwardAuth / OAuth2 Proxy |
