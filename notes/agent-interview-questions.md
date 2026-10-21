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

## 第二梯队：BondSeek 有设计、待整理成文 💡

| # | 面试题 | BondSeek 对应设计（素材位置） |
|---|--------|------------------------------|
| 8 | **RAG 检索流程怎么设计？**（召回→融合→重排→生成） | `bondseek-common/rag/retriever/retriever.go`：query 重写 → 三路并行 → RRF 融合 → Rerank → Decay |
| 9 | **多 Agent 怎么编排？** | `bondseek-agent/internal/agents/aistudio/subagents/orchestrator.go`：编排器 + 子 Agent + 上下文隔离 |
| 10 | **SSE 流式对话断线如何恢复？** | `agentcore/open_block_store.go` + `replay_log_store.go` + `wss_hydrator.go` |
| 11 | **如何降低模型幻觉？** | RAG Deep 模式自反思：充分性检查 → 幻觉检测 → 有用性评估三重循环 |
| 12 | **Agent 系统怎么做可观测性？** | `t_agent_memory_drops` / `t_agent_memory_history` 审计 + ClickHouse 指标 |
| 13 | **微服务下 Agent 系统怎么拆分？** | 5 服务拆法：system（用户）/ agent（对话）/ data（RAG）/ sandbox（执行）/ common（共享） |

## 第三梯队：知识库待填充 🚧

| # | 面试题 | 说明 |
|---|--------|------|
| 14 | **ReAct / Plan-and-Execute / Reflection 三种范式对比** | 认知架构基础题，`topics/agent-architecture/` 仅入口无内容 |
| 15 | **函数调用（Function Calling / Tool Calling）原理** | `topics/tool-use/` 仅入口无内容 |
| 16 | **多 Agent 协作模式（Supervisor / Peer / Hierarchical）** | `topics/multi-agent-systems/` 仅入口无内容 |
| 17 | **评估 Agent 有哪些指标？** | `topics/evaluation-and-benchmarks/` 仅入口无内容 |

---

## 📈 复习进度跟踪

> 复习完一道题在对应行前打 ☑️，形成进度条。

- ☐ #1 Context vs Memory
- ☐ #2 上下文窗口溢出
- ☐ #3 记忆管理三件套
- ☐ #4 记忆防污染
- ☐ #5 用户画像设计
- ☐ #6 记忆触发时机
- ☐ #7 LangChain vs LangGraph
- ☐ #8 RAG 检索流程 *(待整理)*
- ☐ #9 多 Agent 编排 *(待整理)*
- ☐ #10 SSE 断线恢复 *(待整理)*
- ☐ #11 防幻觉 *(待整理)*
- ☐ #12 可观测性 *(待整理)*
- ☐ #13 微服务拆分 *(待整理)*
- ☐ #14 认知架构范式 *(待写)*
- ☐ #15 工具调用 *(待写)*
- ☐ #16 多 Agent 模式 *(待写)*
- ☐ #17 评估指标 *(待写)*

---

## 关联文档

- `../README.md` — 知识库总导航（含本清单所引全部文档的入口）
- `../patterns/AGENTS.md` — 案例关联矩阵（理论↔案例↔框架）
