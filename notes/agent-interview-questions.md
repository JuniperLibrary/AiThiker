# Agent 面试话题清单与答案索引

> **创建:** 2026-10-20
> **用途:** Agent 面试复习的"话题地图"——每道题标注答案素材所在的知识库文档。
> **使用方法:** 挑一道题 → 读对应文档 → 用自己的话复述 → 打勾。

---

## 📌 怎么看这张表

- **已覆盖 ✅** = 本知识库已有完整文档，直接读文档即可复习
- **有素材 💡** = BondSeek 代码里有对应设计，但尚未整理成文，需要补
- **待填充 🚧** = 知识库里还没有内容，需要新写

---

## 第 0 题：什么是 Agent（必考送分题，但分水岭）

> **几乎所有 Agent 面试的第一题。** 答到"决策权在模型手里"才是懂行。见专项文档：

📄 [面试必答 0 号题：什么是 Agent（五层递进回答框架）](./what-is-agent.md)

---

## 第一梯队：已覆盖可直接作答 ✅

| # | 面试题 | 答案素材所在文档 |
|---|--------|----------------|
| 1 | **Context 和 Memory 有什么区别？** | [`topics/memory-and-state/context-vs-memory.md`](../topics/memory-and-state/context-vs-memory.md) |
| 2 | **对话太长放不下窗口怎么办？** | [`topics/memory-and-state/session-compression-in-agent-systems.md`](../topics/memory-and-state/session-compression-in-agent-systems.md) |
| 3 | **LLM 没有长期记忆，如何做记忆管理？** | [`patterns/case-study-bondseek-memory.md`](../patterns/case-study-bondseek-memory.md) §3 |
| 4 | **如何避免记忆污染 / 记住错误信息？** | [`patterns/case-study-bondseek-memory.md`](../patterns/case-study-bondseek-memory.md) §3.2 四道闸 |
| 5 | **用户画像怎么设计？** | [`patterns/case-study-bondseek-user-memory.md`](../patterns/case-study-bondseek-user-memory.md) §1-2 |
| 6 | **记忆什么时候写入、什么时候读取？** | [`patterns/case-study-bondseek-user-memory.md`](../patterns/case-study-bondseek-user-memory.md) §8 触发时机 |
| 7 | **LangChain 和 LangGraph 有什么区别？** | [`topics/agent-frameworks/compare-langchain-langgraph.md`](../topics/agent-frameworks/compare-langchain-langgraph.md) |

## 第二梯队：BondSeek 有设计、已全部沉淀成文 ✅（#8-13）

| # | 面试题 | 答案素材所在文档 |
|---|--------|----------------|
| 8 | **RAG 检索流程怎么设计？**（召回→融合→重排→生成） | [`patterns/case-study-bondseek-rag.md`](../patterns/case-study-bondseek-rag.md)：query 扩展 → 三路并行 → RRF 融合 → Rerank → Decay → 分区裁剪 |
| 9 | **多 Agent 怎么编排？** | [`patterns/case-study-bondseek-multi-agent.md`](../patterns/case-study-bondseek-multi-agent.md)：Supervisor 模式、SubAgent 工具化包装、统一引擎、动态提示词 |
| 10 | **SSE 流式对话断线如何恢复？** | [`patterns/case-study-bondseek-sse-recovery.md`](../patterns/case-study-bondseek-sse-recovery.md)：事件序号(seq) + Replay Log 回放 + 水位线去重 |
| 11 | **如何降低模型幻觉？** | [`patterns/case-study-bondseek-hallucination.md`](../patterns/case-study-bondseek-hallucination.md)：自反思三重循环（充分性→幻觉→有用性）+ 引用治理 + 重新生成 |
| 12 | **Agent 系统怎么做可观测性？** | [`patterns/case-study-bondseek-observability.md`](../patterns/case-study-bondseek-observability.md)：审计日志 + 工具调用树 + ClickHouse 指标 + 三层模型 |
| 13 | **微服务下 Agent 系统怎么拆分？** | [`patterns/case-study-bondseek-microservices.md`](../patterns/case-study-bondseek-microservices.md)：业务边界 + 数据所有权（表前缀）+ EventBus 快慢线 + 网关 ForwardAuth |

## 第三梯队：已全部覆盖 ✅（#14-17）

| # | 面试题 | 答案素材所在文档 |
|---|--------|----------------|
| 14 | **ReAct / Plan-and-Execute / Reflection 三种范式对比** | [`topics/agent-architecture/cognitive-architecture-paradigms.md`](../topics/agent-architecture/cognitive-architecture-paradigms.md)：三大范式对比 + 生产组合拳 + 30 秒面试框架 |
| 15 | **函数调用（Function Calling / Tool Calling）原理** | [`topics/tool-use/function-calling.md`](../topics/tool-use/function-calling.md)：工具三要素 + 调用闭环 + BondSeek Go 落地 + 工程坑（防死循环/错误处理/上下文污染） |
| 16 | **多 Agent 协作模式（Supervisor / Peer / Hierarchical）** | [`topics/multi-agent-systems/coordination-patterns.md`](../topics/multi-agent-systems/coordination-patterns.md)：三模式对比 + 选型决策框架 + Sub-Agent 工具化包装落地 + 30 秒面试框架 |
| 17 | **评估 Agent 有哪些指标？** | [`topics/evaluation-and-benchmarks/method-agent-evaluation-metrics.md`](../topics/evaluation-and-benchmarks/method-agent-evaluation-metrics.md)：四维度指标 + 端到端/分解 + 自动/人工 + GAIA 等基准 + Effect Size |
| 18 | **Tool 执行失败怎么处理？（如网络异常）** | [`topics/tool-use/function-calling.md`](../topics/tool-use/function-calling.md) §6：先分类（网络=重试/参数=自改/业务=降级/安全=拒绝）+ 失败是给模型的 Observation + BondSeek 落地 + 追问防御 |
| 19 | **Agent 一直重复搜索 / 在多个 Agent 之间来回交接怎么处理？** | [`topics/agent-architecture/loop-convergence-control.md`](../topics/agent-architecture/loop-convergence-control.md)：两类循环根因 + 检测信号 + 硬上限/动作指纹去重/收敛判据/权责结构 + 兜底 + BondSeek 事实核对 + 行业方案对照 |

---

## 📈 复习进度跟踪

> 复习完一道题在对应行前打 ☑️，形成进度条。

- ☐ #0 什么是 Agent — [专项回答框架](./what-is-agent.md)

- ☐ #1 Context vs Memory
- ☐ #2 上下文窗口溢出
- ☐ #3 记忆管理三件套
- ☐ #4 记忆防污染
- ☐ #5 用户画像设计
- ☐ #6 记忆触发时机
- ☐ #7 LangChain vs LangGraph
- ☑️ #8 RAG 检索流程 — [RAG 案例文档](../patterns/case-study-bondseek-rag.md)
- ☑️ #9 多 Agent 编排 — [Multi-Agent 案例文档](../patterns/case-study-bondseek-multi-agent.md)
- ☑️ #10 SSE 断线恢复 — [SSE 案例文档](../patterns/case-study-bondseek-sse-recovery.md)
- ☑️ #11 防幻觉 — [防幻觉案例文档](../patterns/case-study-bondseek-hallucination.md)
- ☑️ #12 可观测性 — [可观测性案例文档](../patterns/case-study-bondseek-observability.md)
- ☑️ #13 微服务拆分 — [微服务案例文档](../patterns/case-study-bondseek-microservices.md)
- ☑️ #14 认知架构范式 — [认知架构范式文档](../topics/agent-architecture/cognitive-architecture-paradigms.md)
- ☑️ #15 工具调用 — [函数调用原理文档](../topics/tool-use/function-calling.md)
- ☑️ #16 多 Agent 模式 — [多 Agent 协作模式文档](../topics/multi-agent-systems/coordination-patterns.md)
- ☑️ #17 评估指标 — [评估 Agent 全解文档](../topics/evaluation-and-benchmarks/method-agent-evaluation-metrics.md)
- ☑️ #18 工具失败处理 — [函数调用原理文档](../topics/tool-use/function-calling.md) §6
- ☐ #19 重复搜索 / 多 Agent 交接循环 — [循环收敛控制文档](../topics/agent-architecture/loop-convergence-control.md)

> 🎉 **#0~#18 全部 19 题已覆盖，#19 已登记待复习** — 知识库可作为完整求职复习体系。

---

## 关联文档

- `../README.md` — 知识库总导航（含本清单所引全部文档的入口）
- `../patterns/AGENTS.md` — 案例关联矩阵（理论↔案例↔框架）
