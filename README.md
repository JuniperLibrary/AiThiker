# AiThiker — AI Agent 技术思考知识库

> **这是什么:** 关于"AI Agent（智能体）"的学习笔记库——想搞懂 Agent 怎么工作、怎么设计、面试会问什么，从这里开始。
> **怎么用:** 按照下文"知识地图"找你想了解的主题 → 点开对应文档。文档里英文术语都有中文对照（开头有速查表，文末有完整术语表）。

---

## 🗺️ 知识地图（先看这篇）

> **如果只看一篇，看这个:** 理解 Agent 的"上下文（Context）"和"记忆（Memory）"到底是什么、怎么管理、区别在哪。

📄 [Context 与 Memory：定义、区别与协同管理](./topics/memory-and-state/context-vs-memory.md)
- 解决什么问题：模型"看到了什么"（上下文） vs Agent"记住了什么"（记忆）
- 适合谁：完全零基础，想先建立 Agent 记忆系统的整体概念

---

## 📚 已沉淀的实质文档（按主题分类）

### 1️⃣ 理论框架（先读，建立概念）

| 文档 | 一句话概括 | 难度 |
|------|-----------|------|
| [Context 与 Memory 本体论](./topics/memory-and-state/context-vs-memory.md) | 上下文与记忆的定义、区别、如何协同管理 | ⭐ 入门 |
| [会话压缩专题](./topics/memory-and-state/session-compression-in-agent-systems.md) | 对话太长放不下窗口怎么办：剪枝、摘要、压缩策略全解析 | ⭐⭐⭐ 进阶 |
| [LangChain vs LangGraph 对比](./topics/agent-frameworks/compare-langchain-langgraph.md) | 组件库 vs 状态机编排引擎，Agent 框架怎么选 | ⭐⭐ 中阶 |

### 2️⃣ 生产案例分析（BondSeek 债券 AI 平台——真实生产代码）

| 文档 | 一句话概括 | 难度 |
|------|-----------|------|
| [BondSeek 记忆与上下文系统](./patterns/case-study-bondseek-memory.md) | 生产级 Agent 怎么管记忆：三层记忆 + 质量闸 + 双轨存储 + 冷热遗忘 | ⭐⭐⭐ 进阶 |
| [BondSeek 用户记忆专项](./patterns/case-study-bondseek-user-memory.md) | 深度拆解"记住用户偏好"：明说 vs 推断、触发时机、画像数据结构 | ⭐⭐⭐ 进阶 |

### 3️⃣ 实验记录（动手实践）

| 文档 | 一句话概括 | 难度 |
|------|-----------|------|
| [Go 多轮对话 Agent 实验](./experiments/go-multi-turn-agent-design.md) | 用 Go 从零写一个多轮对话 Agent（零依赖） | ⭐⭐ 中阶 |
| [API 测试命令集](./resources/api-curl-commands.md) | 调用内网模型接口的 curl 命令参考 | ⭐ 入门 |

---

## 🧭 想解决什么问题 → 看哪篇

| 你的问题 | 去看 |
|---------|------|
| "模型为什么记不住之前聊的？" | [Context 与 Memory 本体论](./topics/memory-and-state/context-vs-memory.md) |
| "对话太长放不下怎么办？" | [会话压缩专题](./topics/memory-and-state/session-compression-in-agent-systems.md) |
| "LLM 怎么记住用户偏好？" | [BondSeek 用户记忆专项](./patterns/case-study-bondseek-user-memory.md) |
| "LangChain 和 LangGraph 选哪个？" | [LangChain vs LangGraph](./topics/agent-frameworks/compare-langchain-langgraph.md) |
| "真实生产系统记忆怎么做？" | [BondSeek 记忆与上下文系统](./patterns/case-study-bondseek-memory.md) |
| "Agent 面试会问记忆/上下文什么？" | [Context 与 Memory 本体论](./topics/memory-and-state/context-vs-memory.md) + [会话压缩专题](./topics/memory-and-state/session-compression-in-agent-systems.md) |

---

## 🗄️ 知识库结构总览

```
AiThiker/
├── README.md                       ← 你在这（总导航）
├── AGENTS.md                       ← 给 AI 开发助手看的约定（不用读）
├── topics/                         ← 📖 理论知识（按领域分目录）
│   ├── memory-and-state/           ← 记忆与上下文（当前最全）
│   ├── agent-frameworks/           ← Agent 框架分析
│   ├── agent-architecture/         ← Agent 架构设计
│   └── ...（共 15 个话题目录，多数为空/入口待填充）
├── patterns/                       ← 🏗️ 可复用设计模式 + 生产案例分析
├── experiments/                    ← 🧪 动手实验记录
├── resources/                      ← 📎 工具/接口参考
├── notes/                          ← 🗒️ 临时笔记（整理后迁移）
└── langchain/                      ← 📦 LangChain 学习/实验区（待填充）
```

---

## 📊 内容进度一览

| 区域 | 状态 | 说明 |
|------|------|------|
| 理论知识（topics/） | ◆◆◇◇ 20% | 2 个话题有实质内容（记忆、框架），其余 13 个仅入口占位 |
| 生产案例（patterns/） | ◆◆◆◆ 80% | 2 篇 BondSeek 深度案例，最完整 |
| 实验记录（experiments/） | ◆◆◇◇ 30% | 1 个 Go 实验 + 设计笔记 |
| 语言标注（术语表） | ◆◆◆◆ 90% | 所有实质文档均有三层标注（速查/内联/术语表） |
| Git 仓库 | ◆◇◇◇ 10% | 已初始化 + 首次提交，后续增量提交 |

---

> AiThiker — Think deep, build agents.
