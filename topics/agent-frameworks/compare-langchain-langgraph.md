---
title: LangChain 与 LangGraph 对比分析
tags: [langchain, langgraph, agent-framework, orchestration, state-machine, comparison]
created: 2026-10-20
updated: 2026-10-20
status: draft
---

# LangChain 与 LangGraph 对比分析

> **分析版本:** LangChain 0.x ～ 1.x（2025 后 Agent 运行时统一为 LangGraph）、LangGraph 2024 起
> **一句话定位:** LangChain 是"组件库"（工具箱），LangGraph 是"状态机编排引擎"（运行总控）。
> **核心论点:** LangGraph 不是 LangChain 的竞争者，而是 **LangChain 对早期 Agent 编排缺陷的自我革命**——它继承了组件生态，却彻底更换了执行哲学。

---

## 1. 演化脉络

```
LangChain 0.x (2022)          LangChain 0.x AgentExecutor  LangGraph (2024)          LangChain 1.x (2025)
│                              │                              │                          │
├─ 组件化链条 Chain/LCEL       ├─ LangChain Expression Language (LCEL)
│                              │  声明式链式表达式，流式/批处理/异步                        │
│                              │  AgentExecutor: 内部是隐式死循环 while loop                │ 默认 Agent 运行时 = LangGraph
│                              │  ("Agent loop 缺乏可控性")                              │ (create_react_agent 等)
│                              ▼                              ▼                          ▼
└──────────────────────────► LangGraph: 图论状态机                 └─ LangChain 1.x 弃用 AgentExecutor
                             StateGraph + 持久化 Checkpoint       完全迁到 LangGraph 运行时
                            + Human-in-the-loop + 恢复/重放
```

**关键结论**：在 LangChain 1.x 生态里，"选 LangChain 还是 LangGraph"已是伪命题——**做 Agent 编排永远选 LangGraph，做组件拼接才用 LangChain**。

---

## 2. 核心哲学差异：链式管线 vs 状态机

| 维度 | LangChain | LangGraph |
|------|-----------|-----------|
| 核心隐喻 | **Pipeline / Chain**（线性或分支的数据流水线） | **有向图 + 状态**（节点是函数，边是路由） |
| 状态管理 | 无内建状态——数据靠显式传递（input→output） | **显式 State 对象**贯穿全程，节点读写共享状态 |
| 控制流 | 声明式（LCEL `\|` 组合）、隐式 Chain 式调用 | **显式图结构**：条件边、循环、分支完全可控 |
| 循环能力 | 无原生 loop（无原生循环语义） | **原生支持环**（图的环 = Agent 循环的命根子） |
| 持久化 | 无 | **Checkpointer**（内存/SQLite/Postgres/Redis），中断-恢复-重放 |
| 交互 | 单次执行、同步返回 | **Human-in-the-loop**（人工介入）：interrupt/resume，人可中途介入 |
| 确定性 | 高（线性链可预测） | 高（图是静态结构，执行路径可穷举检查） |
| 可视化 | 无内建 | **图可绘制**（`draw_mermaid` 导出 Mermaid 图） |

---

## 3. 最本质的差异：状态（State）的处理方式

这是两者分道扬镳的根源——**状态是否显式、是否可持久化**。

### 3.1 LangChain 的"状态"是隐式的

链的每一步输入来自上一步输出，中间结果要么通过 `RunnablePassthrough.assign()`（透传对象，把中间值塞进字典继续传）塞进 dict 传递，要么直接丢弃。没有"跨步骤共享的可变状态"这一概念，更谈不上持久化。

```python
# LangChain 风格（LCEL）：数据流，无状态
prompt | model | output_parser
# 想保留中间态？手搓 dict + lambda
```

### 3.2 LangGraph 的"状态"是显式、可持久化的

一个 `State`（状态）类型定义整个运行时共享的数据结构；每个节点函数接收 `state`、返回对 `state` 的部分更新（由 reducer 归约器合并）。

```python
from typing import TypedDict
from langgraph.graph import StateGraph, START, END

class AgentState(TypedDict):
    messages: list          # 对话历史，用 add reducer 追加
    current_step: int
    tool_results: dict

def tool_node(state: AgentState) -> AgentState:
    return {"tool_results": {"a": 1}}   # 部分更新，reducer 合并

g = StateGraph(AgentState)               # 定义状态图
g.add_node("agent", agent_node)          # 添加节点（执行单元）
g.add_node("tools", tool_node)
g.add_edge(START, "agent")                                      # 起始边
g.add_conditional_edges("agent", route_after_agent,             # 条件路由：看结果决定下一步
                        {"continue": "tools", "end": END})
g.add_edge("tools", "agent")                                    # 环：工具回到 Agent（循环）

app = g.compile(checkpointer=SqliteSaver(...))                  # 编译 + 挂持久化层

# 执行（第一轮）
result = app.invoke({"messages": []},
                    config={"configurable": {"thread_id": "t1"}})
# 恢复/续跑（同一 thread_id：多轮对话 = 同一运行时实例的延续）
result2 = app.invoke({"messages": [user_again]},
                     config={"configurable": {"thread_id": "t1"}})
```

三个关键点：

1. **State 是类型化的**：`TypedDict`/Pydantic 定义契约，节点只依赖接口不依赖实现。
2. **Reducer 合并**：`Annotated[list, operator.add]` 等 reducer 定义"state 更新如何合并"——多节点/多 Agent 并发写 state 不互相覆盖。
3. **Checkpointer 是一切的核心**：`thread_id`（线程 ID，会话标识）+ Checkpointer 把**多轮会话、断点恢复、图重放、时间旅行**变成配置项而非架构复杂度。

---

## 4. AgentExecutor 的死穴 → LangGraph 的救赎

**AgentExecutor**（LangChain 0.x 的 Agent 执行器）内部就是隐式 `while True` 循环：

```text
AgentExecutor 内部 = while True:
    1. 调用 LLM（决定调用工具 or 直接回答）
    2. 执行工具（action）
    3. 把结果 append 回 messages
    ...直到 LLM 决定停止

结构性缺陷:
 ❌ 循环不可见、不可控 —— "执行 3 轮就停"只能靠 max_iterations 硬截断
 ❌ 不可持久化 —— 进程崩溃则 Agent 状态全部蒸发
 ❌ 不能人工介入 —— 没有"执行到一半停下来问用户"的原生机制
 ❌ 不能并行/分支 —— 一个循环只能一条线走到黑
```

LangGraph 把 Agent 循环**显式建模成图**（agent 节点 → 工具节点 → 条件路由 → agent 节点…），于是：
- 迭代上限变成普通逻辑（条件边判断），而非硬截断
- 状态落到 Checkpointer → **持久化、断点恢复**
- `interrupt()` 在任意节点停住等人工输入，`Command(resume=...)` 继续
- 多 Agent 就是多个子图，边在图上显式连接

---

## 5. 多维对比矩阵

| 特性 | LangChain | LangGraph |
|------|-----------|-----------|
| 核心抽象 | Chain / Runnable / LCEL | StateGraph / Node / Edge / State |
| 状态管理 | 隐式传递 | **显式 State + Reducer** |
| 循环 | ❌ 无原生 | ✅ 原生（图环） |
| 条件分支 | 有限（`RunnableBranch`） | ✅ 条件边（完整路由逻辑） |
| 持久化 | ❌ | ✅ Checkpointer（SQLite/Postgres/Redis） |
| 断点/恢复 | ❌ | ✅ 原生（interrupt/resume/replay） |
| 人工介入 | ❌ | ✅ HITL（interrupt 节点） |
| 多 Agent | 手动拼 | ✅ 子图/Supervisor 原生 |
| 流式输出 | ✅（LCEL 流式 output） | ✅（深度集成，含流式 state 更新） |
| 可视化 | 弱 | ✅ Mermaid 图导出 |
| 记忆 | 外挂（`Memory` 类） | ✅ 内建（checkpoint 即记忆） |
| 时间旅行 | ❌ | ✅（重放历史 state） |
| 定位 | **组件库**（LLM/工具/向量库/RAG 胶水） | **编排引擎**（Agent 生命周期控制） |
| 典型用途 | 构建 RAG、工具定义、链式调用、生态集成 | 有状态、可控、可恢复的（多）Agent 系统 |

---

## 6. 实际分工（LangChain 1.x）

```
┌─────────────────────────────────────────────┐
│  LangGraph（编排运行时）                      │
│   create_agent / StateGraph / create_react_agent
│   Checkpointer / interrupt / 多Agent Supervisor
└──────────────┬──────────────────────────────┘
               │ 使用这些组件
┌──────────────▼──────────────────────────────┐
│  LangChain（组件生态）                       │
│   ChatModels / Tools(@tool) / Retrievers     │
│   Embeddings / VectorStores / Prompts        │
│   LCEL(Runnable) / OutputParsers             │
└─────────────────────────────────────────────┘
```

**实践建议**：
- 需要 **RAG / 工具调用 / 生态集成**（向量库、LangSmith）→ LangChain
- 需要 **Agent 生命周期控制**（循环、多轮状态、人工兜底、恢复）→ LangGraph
- 单个 Agent 场景 → LangChain 1.x 的 `create_react_agent`（底层就是 LangGraph，一行上手）

---

## 7. 选型决策指南

| 你的需求 | 选择 |
|---------|------|
| 搭 RAG、做问答、链式调用 | LangChain（LCEL） |
| 单 Agent + 工具循环，进程不挂即可 | LangChain `create_react_agent`（LangGraph 兜底） |
| 持久化会话、断点续跑、多轮长任务 | **LangGraph**（Checkpointer 是刚需） |
| 需要人工审批 / 中途介入工具结果 | **LangGraph**（interrupt） |
| 多 Agent 协作、分层编排 | **LangGraph**（子图 + Supervisor） |
| 快速原型演示 | LangChain LCEL（最简） |

---

## 8. 名词标注表（术语中英文对照）

> 编程术语保留英文原文，中文解释辅助理解。下面列出本对比中出现的核心名词。

| 英文术语 | 中文解释 |
|---------|---------|
| **LangChain** | LangChain：组件生态/工具链框架（2022），提供模型、工具、RAG、链式调用等"积木" |
| **LangGraph** | LangGraph：图论状态机编排引擎（2024），LangChain 的 Agent 运行时正式方案 |
| **LCEL**（LangChain Expression Language） | LangChain 表达式语言：用 `\|` 声明式组合 Runnable 的语法，内置流式/批处理/异步支持 |
| **Chain / Pipeline** | 链 / 流水线：数据按固定顺序流经各处理步骤的线性（或分支）结构 |
| **StateGraph** | 状态图：以节点（函数）+ 边（路由）定义的有状态图，LangGraph 的核心抽象 |
| **Node / Edge** | 节点 / 边：图的组成元素——节点是执行单元（函数），边定义执行顺序与路由 |
| **State** | 状态：贯穿图执行全程的显式共享数据结构，节点通过读写 State 交互 |
| **Reducer** | 归约器：定义 State 更新的合并规则（如 `operator.add` 追加），防止并发覆盖 |
| **Checkpointer** | 检查点保存器：持久化 State 快照的组件（内存/SQLite/Postgres/Redis），实现断点续跑 |
| **thread_id** | 线程 ID：Checkpointer 的键，标识一条对话/执行线；同一 thread_id 续跑 = 多轮会话 |
| **Checkpoint** | 检查点：某个执行时点的 State 快照，用于恢复、重放、时间旅行 |
| **Human-in-the-loop (HITL)** | 人在环路：流程执行中暂停、等待人工输入/审批的机制 |
| **interrupt / resume** | 中断 / 恢复：HITL 的两个原语——`interrupt()` 停住执行，`Command(resume=...)` 继续 |
| **AgentExecutor** | Agent 执行器：LangChain 0.x 的隐式 while 循环式 Agent 编排器（已弃用） |
| **create_react_agent** | React（Reasoning + Acting）Agent 的高层工厂函数：工具循环 + 思考-行动范式，底层跑 LangGraph |
| **Conditional Edges** | 条件边：根据运行时状态决定下一步走哪条边的路由逻辑 |
| **Replay / Time Travel** | 重放 / 时间旅行：基于检查点重放历史执行、回溯到任意 state 的能力 |
| **Supervisor** | 监督者（编排模式）：多 Agent 中负责调度/委派的中心节点 |
| **Subgraph** | 子图：可嵌套进更大图的独立图，用于多 Agent 分层编排 |
| **Runnable** | 可运行对象：LangChain 的统一可调用抽象（模型、链、解析器都实现它），支持 `\|` 组合 |
| **RunnablePassthrough** | LangChain 透传对象：把输入原样传递/赋给中间变量，用来看守链中中间状态 |
| **Retriever** | 检索器：对向量库/文档做检索的抽象，RAG 的核心组件 |
| **Streaming** | 流式输出：按 token/事件分块推送结果而非一次性返回 |
| **LangSmith** | LangChain 官方的可观测性/调试/评测平台 |

---

## 9. 权衡分析与反模式

**适用边界：**
- LangGraph 的引入成本（图结构建模、State 设计、checkpointer 选型）在简单线性场景下是**过度设计**——纯 RAG 问答用 LCEL 更直接。
- LangGraph 强在有环图，但**有环图也更难调试**——靠 mermaid 可视化 + LangSmith 追踪来兜底。
- 版本差异巨大：本文基于 2025 LangChain 1.x 生态，0.x API（如 `from langchain.agents import AgentExecutor`）已废弃，勿照搬旧教程。

**反模式：**
- ❌ **框架崇拜**——先画需求再选框架，不能"因为熟 LangChain 就用它做 Agent"
- ❌ **用 LangChain 硬凹状态**——需要跨轮状态/恢复时还在 LCEL 里手搓 dict，应直接上 LangGraph
- ❌ **无版本标注**——LangChain API 迭代快，记录时必须标注版本（本文已标注）
- ❌ **忽略学习成本**——LangGraph 的状态机心智模型需要专门学习（上手约 1-2 周评估）

---

## 关联文档

- `index.md` — Agent 框架话题入口
- `../agent-architecture/index.md` — Agent 架构模式（编排、运行时）
- `../memory-and-state/context-vs-memory.md` — LangGraph checkpoint 即"记忆"的理论对应
- `../tool-use/index.md` — 工具定义与调用（两类框架皆支持 @tool）
- `../multi-agent-systems/index.md` — 多 Agent 协作与 Supervisor 模式

---

> **思考:** LangGraph 的诞生本质上是"Agent 编排控制权争夺"的胜利——把 Agent 的执行从"模型自主的隐式循环"改造成"开发者可观测、可操控、可恢复的显式图"。这与本知识库多次强调的"Agent 可观测性优先、确定性优先"理念一致：**State 显式化 + 持久化，是 Agent 从"玩具"走向"生产"的分水岭。**
