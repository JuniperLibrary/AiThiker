# AiThiker — AI Agent 技术思考知识库

> 📦 **GitHub 描述（可直接复制）:**
> `AiThiker — AI Agent 技术思考知识库：从 Context/Memory、Function Calling、认知架构到多 Agent 协作、RAG、防幻觉与可观测性，含 8 篇 BondSeek 生产案例深度拆解 + 理论框架 + 动手实验。Think deep, build agents.`
>
> 🏷️ **建议 Topics:** `ai-agents` `llm` `rag` `multi-agent` `langchain` `langgraph` `function-calling` `knowledge-base` `prompt-engineering` `ai-notes`

> **这是什么:** 关于"AI Agent（智能体）"的学习笔记库——想搞懂 Agent 怎么工作、怎么设计、面试会问什么，从这里开始。
> **怎么用:** 按照下文"知识地图"找你想了解的主题 → 点开对应文档。文档里英文术语都有中文对照（开头有速查表，文末有完整术语表）。

---

## 🗺️ 知识地图（先看这篇）

> **如果只看一篇，看这个:** 理解 Agent 的"上下文（Context）"和"记忆（Memory）"到底是什么、怎么管理、区别在哪。

📄 [Context 与 Memory：定义、区别与协同管理](./topics/memory-and-state/context-vs-memory.md)
- 解决什么问题：模型"看到了什么"（上下文） vs Agent"记住了什么"（记忆）
- 适合谁：完全零基础，想先建立 Agent 记忆系统的整体概念

---

## 🧭 总纲（强烈建议先读这篇）

> **想"看懂整体 Agent"从这篇开始**——前两版知识地图都是单点，这篇把全库串成一张图。

📄 [从零到一构建 Agent：一次对话的完整旅程](./topics/agent-architecture/from-zero-to-agent.md)
- 解决什么问题：单篇案例是"零件说明书"，这篇是"装配图"——用"一次对话的完整旅程"（加载记忆→理解问题→编排专家→检索资料→流式输出→断线恢复→异步写回记忆）讲清每个环节的"冲突→方案→代价"
- 读完收获：一张 Agent 系统完整链路图 + **5 大设计哲学卡片**（LLM 不可信→规则兜底 / 上下文是稀缺资源 / 分离与隔离是协作前提 / 一切数字化 / 增强永不拖垮主链路）

---

## 📚 已沉淀的实质文档（按主题分类）

### 1️⃣ 理论框架（先读，建立概念）

| 文档 | 一句话概括 | 难度 |
|------|-----------|------|
| [从零到一构建 Agent（总纲）](./topics/agent-architecture/from-zero-to-agent.md) | 一次对话的完整旅程：每环节的冲突→方案→代价 + 5 大设计哲学 | ⭐⭐ 中阶（建议先读） |
| [Context 与 Memory 本体论](./topics/memory-and-state/context-vs-memory.md) | 上下文与记忆的定义、区别、如何协同管理 | ⭐ 入门 |
| [会话压缩专题](./topics/memory-and-state/session-compression-in-agent-systems.md) | 对话太长放不下窗口怎么办：剪枝、摘要、压缩策略全解析 | ⭐⭐⭐ 进阶 |
| [LangChain vs LangGraph 对比](./topics/agent-frameworks/compare-langchain-langgraph.md) | 组件库 vs 状态机编排引擎，Agent 框架怎么选 | ⭐⭐ 中阶 |
| [认知架构范式对比（ReAct / Plan / Reflection）](./topics/agent-architecture/cognitive-architecture-paradigms.md) | Agent 的"思考与行动"三大组织方式：边想边做/先计划后执行/先做后审 | ⭐⭐ 中阶 |
| [函数调用原理（Function Calling / Tool Calling）](./topics/tool-use/function-calling.md) | Agent 怎么"动手"：工具定义/参数/结果三要素 + 调用闭环 + BondSeek Go 实现 | ⭐⭐ 中阶 |
| [多 Agent 协作模式（Supervisor / Peer / Hierarchical）](./topics/multi-agent-systems/coordination-patterns.md) | 多个 AI 怎么分工：老板模式/对等模式/分层模式对比 + 工具化包装落地 | ⭐⭐ 中阶 |
| [评估 Agent 全解（指标 / 方法 / 基准）](./topics/evaluation-and-benchmarks/method-agent-evaluation-metrics.md) | Agent 怎么打分：四维度指标 + 端到端/分解 + 自动/人工 + GAIA 等基准 | ⭐⭐ 中阶 |

### 2️⃣ 生产案例分析（BondSeek 债券 AI 平台——真实生产代码）

| 文档 | 一句话概括 | 难度 |
|------|-----------|------|
| [BondSeek 记忆与上下文系统](./patterns/case-study-bondseek-memory.md) | 生产级 Agent 怎么管记忆：三层记忆 + 质量闸 + 双轨存储 + 冷热遗忘 | ⭐⭐⭐ 进阶 |
| [BondSeek 用户记忆专项](./patterns/case-study-bondseek-user-memory.md) | 深度拆解"记住用户偏好"：明说 vs 推断、触发时机、画像数据结构 | ⭐⭐⭐ 进阶 |
| [BondSeek RAG 检索系统](./patterns/case-study-bondseek-rag.md) | RAG 怎么找资料：Query 四步扩展、三路并行召回、RRF 融合、Rerank 精排、时间衰减 | ⭐⭐⭐ 进阶 |
| [BondSeek 多 Agent 编排](./patterns/case-study-bondseek-multi-agent.md) | 多个 AI 角色怎么分工：监工+专家模式、专家包装成工具、统一执行引擎 | ⭐⭐⭐ 进阶 |
| [BondSeek SSE 断线恢复](./patterns/case-study-bondseek-sse-recovery.md) | 流式对话断线怎么办：事件序号+回放日志+水位线去重，断线自动补全 | ⭐⭐⭐ 进阶 |
| [BondSeek 防幻觉体系](./patterns/case-study-bondseek-hallucination.md) | 模型胡说怎么办：自反思三重循环（查够不够→查有没有出处→查有没有用）+ 强制引用 | ⭐⭐⭐ 进阶 |
| [BondSeek 可观测性](./patterns/case-study-bondseek-observability.md) | Agent 黑盒怎么查问题：审计日志+工具调用树+指标统计，AI 决策也留痕 | ⭐⭐⭐ 进阶 |
| [BondSeek 微服务拆分](./patterns/case-study-bondseek-microservices.md) | 大系统怎么拆小服务：业务边界+数据所有权（表前缀）+EventBus 快慢线+网关注册认证 | ⭐⭐⭐ 进阶 |

### 3️⃣ 实验记录（动手实践）

| 文档 | 一句话概括 | 难度 |
|------|-----------|------|
| [Go 多轮对话 Agent 实验](./experiments/go-agent/SPEC.md) | 多轮对话 Agent 深度规格（v1.0 题目）+ v0 现状记录 | ⭐⭐⭐ 进阶 |
| [北京外埠车出行合规助手](./experiments/beijing-driving-helper/README.md) | 合规出行判断工具：区域/限行时段分析 + 进京证账本（单文件 HTML） | ⭐ 入门 |
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
| "RAG 怎么检索到对的资料？" | [BondSeek RAG 检索系统](./patterns/case-study-bondseek-rag.md) |
| "多个 Agent 怎么分工协作？" | [BondSeek 多 Agent 编排](./patterns/case-study-bondseek-multi-agent.md) |
| "流式对话断线怎么办？" | [BondSeek SSE 断线恢复](./patterns/case-study-bondseek-sse-recovery.md) |
| "模型回答胡说/幻觉怎么办？" | [BondSeek 防幻觉体系](./patterns/case-study-bondseek-hallucination.md) |
| "Agent 黑盒怎么排查问题？" | [BondSeek 可观测性](./patterns/case-study-bondseek-observability.md) |
| "Agent 系统怎么拆微服务？" | [BondSeek 微服务拆分](./patterns/case-study-bondseek-microservices.md) |
| "想整体看懂 Agent 系统？" | [从零到一构建 Agent（总纲）](./topics/agent-architecture/from-zero-to-agent.md) |
| "ReAct / Plan-and-Execute / Reflection 有什么区别？" | [认知架构范式对比](./topics/agent-architecture/cognitive-architecture-paradigms.md) |
| "Function Calling 是什么原理？" | [函数调用原理](./topics/tool-use/function-calling.md) |
| "多个 Agent 怎么分工协作？" | [多 Agent 协作模式](./topics/multi-agent-systems/coordination-patterns.md) + [BondSeek 多 Agent 编排](./patterns/case-study-bondseek-multi-agent.md) |
| "Agent 做得对不对怎么评？" | [评估 Agent 全解](./topics/evaluation-and-benchmarks/method-agent-evaluation-metrics.md) |
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
├── experiments/                    ← 🧪 动手实验记录 + 小型工具项目
├── resources/                      ← 📎 工具/接口参考
├── notes/                          ← 🗒️ 临时笔记（整理后迁移）
└── langchain/                      ← 📦 LangChain 学习/实验区（待填充）
```

---

## 📊 内容进度一览

| 区域 | 状态 | 说明 |
|------|------|------|
| 理论知识（topics/） | ◆◆◆◇ 40% | 记忆 2 篇 + 框架 1 篇 + **总纲 1 篇**（agent-architecture） |
| 生产案例（patterns/） | ◆◆◆◆ 100% | 8 篇 BondSeek 深度案例（记忆/画像/RAG/编排/SSE/防幻觉/可观测性/微服务），全部带 TL;DR 冲突摘要 |
| 实验记录（experiments/） | ◆◆◇◇ 30% | 1 个 Go 实验 + 设计笔记 + 1 个出行合规工具 |
| 语言标注（术语表） | ◆◆◆◆ 95% | 全部实质文档三层标注（速查/内联/术语表）+ TL;DR |
| **总纲导航** | ◆◆◆◆ 100% | 「一次对话的完整旅程」串起全部案例 + 5 大设计哲学卡片 |
| Git 仓库 | ◆◇◇◇ 10% | 已初始化 + 首次提交，有大量未提交变更待确认 |

---

> AiThiker — Think deep, build agents.
