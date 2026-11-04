# Agent 系统架构

> Agent 系统的认知循环、编排模式和组件设计。

## 🔥 从这篇开始（总纲）

> **想"看懂整体 Agent"先读这篇** —— 用"一次对话的完整旅程"串起全库所有案例，附 5 大设计哲学卡片。

📄 [从零到一构建 Agent：一次对话的完整旅程](./from-zero-to-agent.md)
- 解决什么问题：单篇案例是"零件说明书"，这篇是"装配图"——每个环节的"冲突→方案→代价"
- 适合谁：任何人（先有骨架，零件才有位置感）
- 读完收获：一张完整链路图 + 5 句可迁移的设计哲学

## 内容覆盖

- 认知架构模式（ReAct、Plan-and-Execute、Reflection）
- 组件解耦与依赖注入
- 状态管理与生命周期
- 可扩展性与插件系统

## 目录内文档

| 文档 | 主题 |
|------|------|
| [`from-zero-to-agent.md`](./from-zero-to-agent.md) | **总纲**：一次对话的完整旅程，串起全库所有案例 + 5 大设计哲学卡片 |
| [`cognitive-architecture-paradigms.md`](./cognitive-architecture-paradigms.md) | 认知架构三大范式对比：ReAct（边想边做）/ Plan-and-Execute（先计划后执行）/ Reflection（先做后审），含生产组合拳 + 30 秒面试框架 |
| [`loop-convergence-control.md`](./loop-convergence-control.md) | **循环收敛控制**：重复检索与多 Agent 来回交接怎么发现、怎么停；硬上限 / 动作指纹去重 / 收敛判据 / 权责结构四层治理 + BondSeek 事实核对 |

## 核心资源

- `papers/` — 相关论文笔记
- `experiments/` — 架构实验记录
- `patterns/` — 可复用架构模式

---

*本目录为话题入口，细分文档用英文连字符命名。*
