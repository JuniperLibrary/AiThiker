# Topics — 核心话题目录

**路径:** `topics/`
**作用:** 按领域组织的 AI Agent 技术话题集合。每个子目录代表一个独立的知识领域。

## 结构

```
topics/
├── agent-architecture/         # 核心 — Agent 系统架构设计（编排、运行时、分层、状态机）
├── llm-reasoning/              # 核心 — LLM 推理机制（思维链、推理扩展、幻觉机理）
├── prompt-engineering/         # 核心 — 提示工程（指令设计、结构、few-shot、系统提示）
├── tool-use/                   # 核心 — 工具使用与函数调用（Tool Calling、MCP、工具选择）
├── planning-and-decomposition/ # 核心 — 任务规划与分解（计划生成、任务拆解、重规划）
├── multi-agent-systems/        # 进阶 — 多 Agent 协作系统（角色分工、通信、编排协议）
├── memory-and-state/           # 进阶 — 记忆管理与状态持久化（工作/长期/情景记忆）
├── safety-and-alignment/       # 进阶 — 安全性与对齐（越狱防护、注入攻击、价值对齐）
├── evaluation-and-benchmarks/  # 进阶 — 评估方法与基准测试（评测体系、指标、测试集）
├── agent-frameworks/           # 进阶 — 主流 Agent 框架分析（LangChain、AutoGen 等）
├── knowledge-representation/   # 进阶 — 知识表示（RAG、图谱、知识蒸馏、知识库）
├── human-agent-interaction/    # 进阶 — 人机交互设计（对话范式、反馈机制、协作界面）
├── code-generation-agents/     # 进阶 — 代码生成 Agent（写码分层、验证、风险）
├── fine-tuning-for-agents/     # 进阶 — Agent 微调技术（工具调用、指令微调、RL 对齐）
└── observability-and-tracing/  # 补充 — 可观测性与链路追踪（追踪、日志、评测观测）
```

## 优先级分类
 
| 级别 | 说明 |
|------|------|
| ⭐ 核心 | 所有 Agent 系统的基础构建块，优先投入 |
| 🔷 进阶 | 特定场景或成熟系统的进阶主题 |
| 💡 补充 | 专业领域或前沿探索方向 |

## 约定

- 每个子目录以 `index.md` 作为入口
- 话题间交叉引用使用相对路径 `../target-dir/file.md`
- 核心话题之间引用频繁，进阶话题单向引用核心话题
