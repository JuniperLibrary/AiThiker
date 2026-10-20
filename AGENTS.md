# AiThiker — AI Agent 技术思考知识库

**生成时间:** 2026-08-13
**项目状态:** 初始化阶段

## 概述

AiThiker 是一个围绕 **AI Agent 技术** 的发散性、技术性思考项目。核心目标是对每个话题领域进行深度文档记录，涵盖架构、推理、工具使用、安全对齐、评估等方向。文档以 Markdown 形式组织，强调技术深度和实践洞察。

## 目录结构

```
AiThiker/
├── topics/                        # 核心话题目录（按领域划分）
│   ├── agent-architecture/        # Agent 系统架构设计
│   ├── llm-reasoning/             # LLM 推理机制与思维链
│   ├── prompt-engineering/        # 提示工程与指令设计
│   ├── multi-agent-systems/       # 多 Agent 协作系统
│   ├── tool-use/                  # 工具使用与函数调用
│   ├── memory-and-state/          # 记忆管理与状态持久化
│   ├── safety-and-alignment/      # 安全性与对齐
│   ├── evaluation-and-benchmarks/ # 评估方法与基准测试
│   ├── agent-frameworks/          # 主流 Agent 框架分析
│   ├── knowledge-representation/  # 知识表示与管理
│   ├── human-agent-interaction/   # 人机交互设计
│   ├── planning-and-decomposition/# 任务规划与分解
│   ├── code-generation-agents/    # 代码生成 Agent
│   ├── fine-tuning-for-agents/    # Agent 微调技术
│   └── observability-and-tracing/ # 可观测性与链路追踪
├── patterns/                      # 可复用的 Agent 设计模式
├── resources/                     # 论文、博客、工具资源汇总
├── experiments/                   # 实验记录与分析
└── notes/                         # 临时笔记与未分类思考
```

## 话题索引

| 目录 | 主题 | 优先级 |
|------|------|--------|
| `topics/agent-architecture/` | Agent 系统架构 | ⭐ 核心 |
| `topics/llm-reasoning/` | 推理机制 | ⭐ 核心 |
| `topics/prompt-engineering/` | 提示工程 | ⭐ 核心 |
| `topics/multi-agent-systems/` | 多 Agent | 🔷 进阶 |
| `topics/tool-use/` | 工具使用 | ⭐ 核心 |
| `topics/memory-and-state/` | 记忆管理 | 🔷 进阶 |
| `topics/safety-and-alignment/` | 安全对齐 | 🔷 进阶 |
| `topics/evaluation-and-benchmarks/` | 评估基准 | 🔷 进阶 |
| `topics/agent-frameworks/` | 框架分析 | 🔷 进阶 |
| `topics/knowledge-representation/` | 知识表示 | 🔷 进阶 |
| `topics/human-agent-interaction/` | 人机交互 | 🔷 进阶 |
| `topics/planning-and-decomposition/` | 规划分解 | ⭐ 核心 |
| `topics/code-generation-agents/` | 代码生成 | 🔷 进阶 |
| `topics/fine-tuning-for-agents/` | 微调技术 | 🔷 进阶 |
| `topics/observability-and-tracing/` | 可观测性 | 💡 补充 |

## 文档规范

### 文件命名
- 每个话题下以 `index.md` 为入口文件
- 细分主题用英文连字符命名: `topic-subtopic.md`
- 实验记录用日期前缀: `YYYY-MM-DD-description.md`

### Markdown 风格
- 一级标题 `#` 仅在 `index.md` 中使用
- 细分文档从二级标题 `##` 开始
- 代码块标注语言（如 `python`、`typescript`）
- 引用格式: `> **来源:** [标题](链接)`

### Frontmatter 元数据（可选建议）
```yaml
---
title: 标题
tags: [tag1, tag2]
created: YYYY-MM-DD
updated: YYYY-MM-DD
status: draft | review | published
---
```

## 反模式（本项目中避免）

- **空头理论** — 每个话题必有技术细节或代码示例支撑
- **重复记录** — 同一概念只在一个 topic 目录下深入展开，其他目录交叉引用
- **无来源断言** — 引用论文/文章时标注来源
- **过时信息** — 定期 review，标注"最后验证日期"

## 目标读者

- AI Agent 开发者与架构师
- LLM 应用研究人员
- 对 Agent 技术栈感兴趣的工程师

---

> AiThiker — Think deep, build agents.
