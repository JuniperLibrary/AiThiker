# Multi-Agent Systems — 目录约定

**路径:** topics/multi-agent-systems/
**作用:** 多 Agent 协作系统的通信协议、角色分配与共识机制文档。

## 文档类型

| 类型 | 命名模式 | 说明 |
|------|----------|------|
| 入口 | index.md | 话题概述与索引（已存在） |
| 概念文档 | concept-*.md | 核心概念深度分析 |
| 架构文档 | arch-*.md | 编排 vs 去中心化架构对比 |
| 案例文档 | case-*.md | 具体多 Agent 场景分析 |
| 实验记录 | YYYY-MM-DD-*.md | 实验与验证记录 |

## 交互图约定

Agent 间通信必须使用 Mermaid sequence diagram：

`mermaid
sequenceDiagram
    participant Coordinator
    participant AgentA
    participant AgentB
    Coordinator->>AgentA: Task assignment
    AgentA->>AgentB: Sub-task delegation
    AgentB-->>AgentA: Result
    AgentA-->>Coordinator: Aggregated result
`

- participant 名称使用 CamelCase 角色名
- 消息体用英文简写，描述动宾结构

## 通信模式文档结构

每个通信模式文档应包含：
1. 概述 — 模式名称与适用场景
2. 协议定义 — 消息格式（JSON Schema 示例）
3. 交互流程 — Mermaid sequence diagram
4. 优缺点 — 对比表格
5. 实现参照 — 现有框架中的对应实现

## 反模式（本目录避免）

- 无协议定义的架构图 — 必须附带消息格式
- 单场景方案 — 注明适用约束与扩展点
- 忽略通信开销 — 分析延迟与带宽瓶颈

---

引用外部话题: 见 ../agent-architecture/, ../../patterns/
