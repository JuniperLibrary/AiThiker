# Agent Frameworks — 目录约定

**路径:** topics/agent-frameworks/
**作用:** 主流 Agent 框架的深度分析、特性对比与集成模式。

## 文档分类

| 类别 | 命名模式 | 说明 |
|------|----------|------|
| 入口 | index.md | 话题总览（已存在） |
| 框架分析 | framework-*.md | 单个框架深度分析 |
| 对比表格 | compare-*.md | 框架间多维度对比 |
| 集成指南 | integration-*.md | 指定框架的集成模式 |
| 实验记录 | YYYY-MM-DD-*.md | 框架试用记录 |

## 框架对比矩阵约定

框架对比文档使用统一格式的矩阵表：

| 特性 / 框架 | LangChain | AutoGen | CrewAI | OpenAI Assistants |
|-------------|-----------|---------|--------|-------------------|
| 编排模式 | DAG/Graph | Conversation | Sequential | Stateless |
| 工具定义 | @tool decorator | Tool class | Tool dataclass | function calling |
| 记忆支持 | Built-in | Custom | Built-in | Thread history |
| 多 Agent | LangGraph | Native | Native | Assistant per agent |
| 流式输出 | ✓ | ✓ | ✓ | ✓ |
| 可观测性 | LangSmith | Custom | Custom | Dashboard |
| 开源协议 | MIT | MIT | MIT | Proprietary |

## 框架分析文档结构

每个框架分析文档应包含：
1. 概述 → 框架定位、核心哲学
2. 架构抽象 → 关键抽象层（Agent, Tool, Memory, Orchestrator）
3. 核心模式 → 代码示例展示典型 API 使用方式
4. 性能特征 → 基准测试与扩展性分析
5. 集成模式 → 与现有工具/框架的搭配方式
6. 权衡分析 → 适用边界、性能瓶颈、学习成本

## 反模式（本目录外）

- 框架崇拜与万能论 → 每个框架标注适用边界与非适用场景
- 版本锁定 → 标注分析时针对的具体版本号
- 忽略学习成本 → 每个框架分析包含上手时间评估

---

交叉引用: → ../agent-architecture/, ../../patterns/, ../tool-use/, ../evaluation-and-benchmarks/
