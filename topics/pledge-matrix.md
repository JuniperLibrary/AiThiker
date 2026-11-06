# topics 承诺-存在矩阵报告

> **生成日期:** 2026-11-04
> **生成方式:** `node topics/check-pledges.mjs`（机器判定，可重跑）
> **范围:** topics/ 下全部 *.md（42 个文件，不含本报告）
> **判定规则:** 文件中出现的文件/目录引用（Markdown 链接、反引号路径、表格承诺、正文路径）必须在文件系统中存在；含斜杠路径依次按本目录 → topics/ → 仓库根解析，裸文件名同序解析。首段命中外部系统白名单（BondSeek 源码/示例路径）的标记 ⚪ 不可判定；裸文件名在仓库中有唯一同名文件的标记 ⚪ 文档名指称。
> **对应题目:** topics/SPEC.md §5.1（深度轴 D6 对抗验证之机器裁决）

---

## 汇总

| 指标 | 数值 |
|---|---|
| 扫描文件 | 42 |
| 引用总数 | 234 |
| ✅ 存在 | 197 |
| ❌ 缺失 | 0 |
| ⚪ 文档名指称（未链接，可唯一解析） | 0 |
| ⚪ 外部系统引用（机器不可判定） | 21 |
| ⚪ 显式未创建承诺（stub） | 16 |
| ⚠️ 控制字符污染（\t / \r 混入正文） | 0 处 / 0 个文件 |

---

## ❌ 缺失（0）—— 必须补齐或删除承诺

| 来源文件 | 行 | 引用 | 判定尝试 | 备注 |
|---|---|---|---|---|


---

## ⚪ 显式未创建承诺（stub，16）—— 行内已声明"计划/未创建/待创建"，不视为坏承诺；补齐文档后移出本表

| 来源文件 | 行 | 引用 |
|---|---|---|
| topics/llm-reasoning/AGENTS.md | 11 | `chain-of-thought.md` |
| topics/llm-reasoning/AGENTS.md | 14 | `reflection.md` |
| topics/llm-reasoning/AGENTS.md | 13 | `self-consistency.md` |
| topics/llm-reasoning/AGENTS.md | 12 | `tree-of-thought.md` |
| topics/planning-and-decomposition/AGENTS.md | 12 | `dependency-resolution.md` |
| topics/planning-and-decomposition/AGENTS.md | 11 | `hierarchical-planning.md` |
| topics/planning-and-decomposition/AGENTS.md | 14 | `plan-representation.md` |
| topics/planning-and-decomposition/AGENTS.md | 13 | `replanning.md` |
| topics/prompt-engineering/AGENTS.md | 41 | `../llm-reasoning/chain-of-thought.md` |
| topics/prompt-engineering/AGENTS.md | 12 | `few-shot-selection.md` |
| topics/prompt-engineering/AGENTS.md | 13 | `output-formatting.md` |
| topics/prompt-engineering/AGENTS.md | 14 | `prompt-catalog/` |
| topics/prompt-engineering/AGENTS.md | 11 | `system-prompt-design.md` |
| topics/tool-use/AGENTS.md | 14 | `error-handling.md` |
| topics/tool-use/AGENTS.md | 11 | `tool-definition.md` |
| topics/tool-use/AGENTS.md | 12 | `tool-registry.md` |

---

## ⚪ 文档名指称（0）—— 裸文件名提及，可唯一解析到仓库文件；建议按约定转为相对链接

| 来源文件 | 行 | 引用 | 唯一匹配 |
|---|---|---|---|


---

## ⚪ 外部系统引用（21）—— 指向 BondSeek 源码 / 示例路径，不在本仓库，存在性机器不可判定（SPEC §9 待核 #1）

| 来源文件 | 行 | 引用 |
|---|---|---|
| topics/agent-architecture/from-zero-to-agent.md | 124 | `agentcore/` |
| topics/agent-architecture/from-zero-to-agent.md | 90 | `aistudio/` |
| topics/agent-architecture/from-zero-to-agent.md | 43 | `context/` |
| topics/agent-architecture/from-zero-to-agent.md | 139 | `core/engine.go` |
| topics/agent-architecture/from-zero-to-agent.md | 74 | `retriever/query_processor.go` |
| topics/agent-architecture/from-zero-to-agent.md | 107 | `retriever/retriever.go` |
| topics/memory-and-state/session-compression-in-agent-systems.md | 726 | `.aithiker/sessions/` |
| topics/memory-and-state/session-compression-in-agent-systems.md | 511 | `.config/opencode/opencode.json` |
| topics/memory-and-state/session-compression-in-agent-systems.md | 492 | `.local/share/opencode/` |
| topics/memory-and-state/session-compression-in-agent-systems.md | 507 | `.local/state/opencode/` |
| topics/memory-and-state/session-compression-in-agent-systems.md | 81 | `api/v3/users/` |
| topics/memory-and-state/session-compression-in-agent-systems.md | 504 | `project/global.json` |
| topics/memory-and-state/session-compression-in-agent-systems.md | 674 | `session/state/tasks_active.json` |
| topics/memory-and-state/session-compression-in-agent-systems.md | 660 | `session/summaries/summary_1.json` |
| topics/memory-and-state/session-compression-in-agent-systems.md | 632 | `session/turns/turn_001.json` |
| topics/memory-and-state/session-compression-in-agent-systems.md | 642 | `session/turns/turn_002.json` |
| topics/tool-use/function-calling.md | 180 | `agenttools/tool.go` |
| topics/tool-use/function-calling.md | 15 | `bondseek-agent/internal/tools/register.go` |
| topics/tool-use/function-calling.md | 285 | `bondseek-common/agenttools/` |
| topics/tool-use/function-calling.md | 15 | `bondseek-common/agenttools/tool.go` |
| topics/tool-use/function-calling.md | 13 | `tools/register.go` |

---

## ⚠️ 行内控制字符污染（0）—— 正文中混入字面 \t / \r

| 文件 | 行 | 片段 |
|---|---|---|


---

## ✅ 存在（197）

| 来源文件 | 行 | 引用 | 解析到 |
|---|---|---|---|
| topics/AGENTS.md | 6 | `SPEC.md` | topics/SPEC.md |
| topics/AGENTS.md | 3 | `topics/` | topics |
| topics/SPEC.md | 6 | `../README.md` | README.md |
| topics/SPEC.md | 34 | `../experiments/go-agent/SPEC.md` | experiments/go-agent/SPEC.md |
| topics/SPEC.md | 6 | `../notes/agent-interview-questions.md` | notes/agent-interview-questions.md |
| topics/SPEC.md | 199 | `./` | topics |
| topics/SPEC.md | 29 | `evaluation-and-benchmarks/index.md` | topics/evaluation-and-benchmarks/index.md |
| topics/SPEC.md | 59 | `experiments/` | experiments |
| topics/SPEC.md | 28 | `llm-reasoning/AGENTS.md` | topics/llm-reasoning/AGENTS.md |
| topics/SPEC.md | 33 | `notes/agent-interview-questions.md` | notes/agent-interview-questions.md |
| topics/SPEC.md | 59 | `patterns/` | patterns |
| topics/SPEC.md | 28 | `planning-and-decomposition/AGENTS.md` | topics/planning-and-decomposition/AGENTS.md |
| topics/SPEC.md | 28 | `prompt-engineering/AGENTS.md` | topics/prompt-engineering/AGENTS.md |
| topics/SPEC.md | 29 | `resources/` | resources |
| topics/SPEC.md | 29 | `resources/api-curl-commands.md` | resources/api-curl-commands.md |
| topics/SPEC.md | 29 | `resources/papers.md` | resources/papers.md |
| topics/SPEC.md | 29 | `safety-and-alignment/index.md` | topics/safety-and-alignment/index.md |
| topics/SPEC.md | 28 | `tool-use/AGENTS.md` | topics/tool-use/AGENTS.md |
| topics/SPEC.md | 175 | `tool-use/invocation-patterns.md` | topics/tool-use/invocation-patterns.md |
| topics/SPEC.md | 13 | `topics/` | topics |
| topics/SPEC.md | 7 | `topics/SPEC.md` | topics/SPEC.md |
| topics/SPEC.md | 199 | `topics/check-pledges.mjs` | topics/check-pledges.mjs |
| topics/SPEC.md | 28 | `topics/pledge-matrix.md` | topics/pledge-matrix.md |
| topics/SPEC.md | 200 | `topics/tool-use/invocation-patterns.md` | topics/tool-use/invocation-patterns.md |
| topics/agent-architecture/cognitive-architecture-paradigms.md | 270 | `../../notes/agent-interview-questions.md` | notes/agent-interview-questions.md |
| topics/agent-architecture/cognitive-architecture-paradigms.md | 14 | `../../patterns/case-study-bondseek-hallucination.md` | patterns/case-study-bondseek-hallucination.md |
| topics/agent-architecture/cognitive-architecture-paradigms.md | 14 | `../../patterns/case-study-bondseek-multi-agent.md` | patterns/case-study-bondseek-multi-agent.md |
| topics/agent-architecture/cognitive-architecture-paradigms.md | 14 | `../../patterns/case-study-bondseek-rag.md` | patterns/case-study-bondseek-rag.md |
| topics/agent-architecture/cognitive-architecture-paradigms.md | 14 | `./from-zero-to-agent.md` | topics/agent-architecture/from-zero-to-agent.md |
| topics/agent-architecture/from-zero-to-agent.md | 233 | `../../notes/agent-interview-questions.md` | notes/agent-interview-questions.md |
| topics/agent-architecture/from-zero-to-agent.md | 14 | `../../patterns/case-study-bondseek-memory.md` | patterns/case-study-bondseek-memory.md |
| topics/agent-architecture/from-zero-to-agent.md | 14 | `../../patterns/case-study-bondseek-multi-agent.md` | patterns/case-study-bondseek-multi-agent.md |
| topics/agent-architecture/from-zero-to-agent.md | 14 | `../../patterns/case-study-bondseek-rag.md` | patterns/case-study-bondseek-rag.md |
| topics/agent-architecture/from-zero-to-agent.md | 14 | `../../patterns/case-study-bondseek-sse-recovery.md` | patterns/case-study-bondseek-sse-recovery.md |
| topics/agent-architecture/from-zero-to-agent.md | 14 | `../../patterns/case-study-bondseek-user-memory.md` | patterns/case-study-bondseek-user-memory.md |
| topics/agent-architecture/from-zero-to-agent.md | 49 | `../memory-and-state/session-compression-in-agent-systems.md` | topics/memory-and-state/session-compression-in-agent-systems.md |
| topics/agent-architecture/index.md | 31 | `../../resources/papers.md` | resources/papers.md |
| topics/agent-architecture/index.md | 26 | `./cognitive-architecture-paradigms.md` | topics/agent-architecture/cognitive-architecture-paradigms.md |
| topics/agent-architecture/index.md | 9 | `./from-zero-to-agent.md` | topics/agent-architecture/from-zero-to-agent.md |
| topics/agent-architecture/index.md | 27 | `./loop-convergence-control.md` | topics/agent-architecture/loop-convergence-control.md |
| topics/agent-architecture/index.md | 32 | `experiments/` | experiments |
| topics/agent-architecture/index.md | 33 | `patterns/` | patterns |
| topics/agent-architecture/loop-convergence-control.md | 356 | `../../notes/agent-interview-questions.md` | notes/agent-interview-questions.md |
| topics/agent-architecture/loop-convergence-control.md | 95 | `../../patterns/case-study-bondseek-hallucination.md` | patterns/case-study-bondseek-hallucination.md |
| topics/agent-architecture/loop-convergence-control.md | 105 | `../../patterns/case-study-bondseek-memory.md` | patterns/case-study-bondseek-memory.md |
| topics/agent-architecture/loop-convergence-control.md | 92 | `../../patterns/case-study-bondseek-multi-agent.md` | patterns/case-study-bondseek-multi-agent.md |
| topics/agent-architecture/loop-convergence-control.md | 60 | `../../patterns/case-study-bondseek-observability.md` | patterns/case-study-bondseek-observability.md |
| topics/agent-architecture/loop-convergence-control.md | 105 | `../../patterns/case-study-bondseek-rag.md` | patterns/case-study-bondseek-rag.md |
| topics/agent-architecture/loop-convergence-control.md | 170 | `../agent-frameworks/compare-langchain-langgraph.md` | topics/agent-frameworks/compare-langchain-langgraph.md |
| topics/agent-architecture/loop-convergence-control.md | 50 | `../multi-agent-systems/coordination-patterns.md` | topics/multi-agent-systems/coordination-patterns.md |
| topics/agent-architecture/loop-convergence-control.md | 105 | `../tool-use/function-calling.md` | topics/tool-use/function-calling.md |
| topics/agent-architecture/loop-convergence-control.md | 97 | `./from-zero-to-agent.md` | topics/agent-architecture/from-zero-to-agent.md |
| topics/agent-architecture/loop-convergence-control.md | 355 | `./index.md` | topics/agent-architecture/index.md |
| topics/agent-architecture/loop-convergence-control.md | 170 | `topics/human-agent-interaction/` | topics/human-agent-interaction |
| topics/agent-frameworks/AGENTS.md | 48 | `../../patterns/` | patterns |
| topics/agent-frameworks/AGENTS.md | 48 | `../agent-architecture/` | topics/agent-architecture |
| topics/agent-frameworks/AGENTS.md | 48 | `../evaluation-and-benchmarks/` | topics/evaluation-and-benchmarks |
| topics/agent-frameworks/AGENTS.md | 48 | `../tool-use/` | topics/tool-use |
| topics/agent-frameworks/AGENTS.md | 3 | `topics/agent-frameworks/` | topics/agent-frameworks |
| topics/agent-frameworks/compare-langchain-langgraph.md | 242 | `../agent-architecture/index.md` | topics/agent-architecture/index.md |
| topics/agent-frameworks/compare-langchain-langgraph.md | 243 | `../memory-and-state/context-vs-memory.md` | topics/memory-and-state/context-vs-memory.md |
| topics/agent-frameworks/compare-langchain-langgraph.md | 245 | `../multi-agent-systems/index.md` | topics/multi-agent-systems/index.md |
| topics/agent-frameworks/compare-langchain-langgraph.md | 244 | `../tool-use/index.md` | topics/tool-use/index.md |
| topics/agent-frameworks/compare-langchain-langgraph.md | 241 | `index.md` | topics/agent-frameworks/index.md |
| topics/agent-frameworks/index.md | 7 | `./compare-langchain-langgraph.md` | topics/agent-frameworks/compare-langchain-langgraph.md |
| topics/agent-frameworks/index.md | 16 | `agent-architecture/` | topics/agent-architecture |
| topics/agent-frameworks/index.md | 18 | `experiments/` | experiments |
| topics/agent-frameworks/index.md | 17 | `patterns/` | patterns |
| topics/code-generation-agents/AGENTS.md | 65 | `../evaluation-and-benchmarks/` | topics/evaluation-and-benchmarks |
| topics/code-generation-agents/AGENTS.md | 64 | `../llm-reasoning/` | topics/llm-reasoning |
| topics/code-generation-agents/AGENTS.md | 66 | `../tool-use/` | topics/tool-use |
| topics/code-generation-agents/AGENTS.md | 6 | `index.md` | topics/code-generation-agents/index.md |
| topics/code-generation-agents/AGENTS.md | 5 | `topics/code-generation-agents/` | topics/code-generation-agents |
| topics/code-generation-agents/index.md | 16 | `evaluation-and-benchmarks/` | topics/evaluation-and-benchmarks |
| topics/code-generation-agents/index.md | 17 | `experiments/` | experiments |
| topics/code-generation-agents/index.md | 15 | `tool-use/` | topics/tool-use |
| topics/evaluation-and-benchmarks/AGENTS.md | 48 | `../../experiments/` | experiments |
| topics/evaluation-and-benchmarks/AGENTS.md | 48 | `../agent-frameworks/` | topics/agent-frameworks |
| topics/evaluation-and-benchmarks/AGENTS.md | 48 | `../safety-and-alignment/` | topics/safety-and-alignment |
| topics/evaluation-and-benchmarks/AGENTS.md | 3 | `topics/evaluation-and-benchmarks/` | topics/evaluation-and-benchmarks |
| topics/evaluation-and-benchmarks/index.md | 17 | `./method-agent-evaluation-metrics.md` | topics/evaluation-and-benchmarks/method-agent-evaluation-metrics.md |
| topics/evaluation-and-benchmarks/index.md | 21 | `agent-frameworks/` | topics/agent-frameworks |
| topics/evaluation-and-benchmarks/index.md | 22 | `experiments/` | experiments |
| topics/evaluation-and-benchmarks/index.md | 23 | `resources/papers.md` | resources/papers.md |
| topics/evaluation-and-benchmarks/method-agent-evaluation-metrics.md | 263 | `../../notes/agent-interview-questions.md` | notes/agent-interview-questions.md |
| topics/evaluation-and-benchmarks/method-agent-evaluation-metrics.md | 14 | `../../patterns/case-study-bondseek-hallucination.md` | patterns/case-study-bondseek-hallucination.md |
| topics/evaluation-and-benchmarks/method-agent-evaluation-metrics.md | 14 | `../../patterns/case-study-bondseek-observability.md` | patterns/case-study-bondseek-observability.md |
| topics/evaluation-and-benchmarks/method-agent-evaluation-metrics.md | 262 | `../agent-architecture/cognitive-architecture-paradigms.md` | topics/agent-architecture/cognitive-architecture-paradigms.md |
| topics/evaluation-and-benchmarks/method-agent-evaluation-metrics.md | 261 | `../agent-architecture/from-zero-to-agent.md` | topics/agent-architecture/from-zero-to-agent.md |
| topics/fine-tuning-for-agents/AGENTS.md | 82 | `../evaluation-and-benchmarks/` | topics/evaluation-and-benchmarks |
| topics/fine-tuning-for-agents/AGENTS.md | 83 | `../llm-reasoning/` | topics/llm-reasoning |
| topics/fine-tuning-for-agents/AGENTS.md | 81 | `../tool-use/` | topics/tool-use |
| topics/fine-tuning-for-agents/AGENTS.md | 6 | `index.md` | topics/fine-tuning-for-agents/index.md |
| topics/fine-tuning-for-agents/AGENTS.md | 5 | `topics/fine-tuning-for-agents/` | topics/fine-tuning-for-agents |
| topics/fine-tuning-for-agents/index.md | 16 | `evaluation-and-benchmarks/` | topics/evaluation-and-benchmarks |
| topics/fine-tuning-for-agents/index.md | 17 | `resources/papers.md` | resources/papers.md |
| topics/fine-tuning-for-agents/index.md | 15 | `tool-use/` | topics/tool-use |
| topics/human-agent-interaction/AGENTS.md | 66 | `../../patterns/` | patterns |
| topics/human-agent-interaction/AGENTS.md | 65 | `../agent-architecture/` | topics/agent-architecture |
| topics/human-agent-interaction/AGENTS.md | 64 | `../safety-and-alignment/` | topics/safety-and-alignment |
| topics/human-agent-interaction/AGENTS.md | 6 | `index.md` | topics/human-agent-interaction/index.md |
| topics/human-agent-interaction/AGENTS.md | 5 | `topics/human-agent-interaction/` | topics/human-agent-interaction |
| topics/human-agent-interaction/index.md | 17 | `agent-architecture/` | topics/agent-architecture |
| topics/human-agent-interaction/index.md | 16 | `patterns/` | patterns |
| topics/human-agent-interaction/index.md | 15 | `safety-and-alignment/` | topics/safety-and-alignment |
| topics/knowledge-representation/AGENTS.md | 65 | `../agent-architecture/` | topics/agent-architecture |
| topics/knowledge-representation/AGENTS.md | 63 | `../memory-and-state/` | topics/memory-and-state |
| topics/knowledge-representation/AGENTS.md | 64 | `../tool-use/` | topics/tool-use |
| topics/knowledge-representation/AGENTS.md | 6 | `index.md` | topics/knowledge-representation/index.md |
| topics/knowledge-representation/AGENTS.md | 5 | `topics/knowledge-representation/` | topics/knowledge-representation |
| topics/knowledge-representation/index.md | 17 | `experiments/` | experiments |
| topics/knowledge-representation/index.md | 15 | `memory-and-state/` | topics/memory-and-state |
| topics/knowledge-representation/index.md | 16 | `tool-use/` | topics/tool-use |
| topics/llm-reasoning/AGENTS.md | 15 | `../../experiments/` | experiments |
| topics/llm-reasoning/AGENTS.md | 15 | `experiments/` | experiments |
| topics/llm-reasoning/AGENTS.md | 10 | `index.md` | topics/llm-reasoning/index.md |
| topics/llm-reasoning/AGENTS.md | 35 | `resources/papers.md` | resources/papers.md |
| topics/llm-reasoning/AGENTS.md | 3 | `topics/llm-reasoning/` | topics/llm-reasoning |
| topics/llm-reasoning/index.md | 16 | `experiments/` | experiments |
| topics/llm-reasoning/index.md | 15 | `prompt-engineering/` | topics/prompt-engineering |
| topics/llm-reasoning/index.md | 17 | `resources/papers.md` | resources/papers.md |
| topics/memory-and-state/AGENTS.md | 45 | `../../patterns/` | patterns |
| topics/memory-and-state/AGENTS.md | 45 | `../agent-architecture/` | topics/agent-architecture |
| topics/memory-and-state/AGENTS.md | 45 | `../tool-use/` | topics/tool-use |
| topics/memory-and-state/AGENTS.md | 3 | `topics/memory-and-state/` | topics/memory-and-state |
| topics/memory-and-state/context-vs-memory.md | 245 | `../agent-architecture/index.md` | topics/agent-architecture/index.md |
| topics/memory-and-state/context-vs-memory.md | 246 | `../multi-agent-systems/index.md` | topics/multi-agent-systems/index.md |
| topics/memory-and-state/context-vs-memory.md | 12 | `./session-compression-in-agent-systems.md` | topics/memory-and-state/session-compression-in-agent-systems.md |
| topics/memory-and-state/index.md | 14 | `../../patterns/case-study-bondseek-memory.md` | patterns/case-study-bondseek-memory.md |
| topics/memory-and-state/index.md | 13 | `./context-vs-memory.md` | topics/memory-and-state/context-vs-memory.md |
| topics/memory-and-state/index.md | 12 | `./session-compression-in-agent-systems.md` | topics/memory-and-state/session-compression-in-agent-systems.md |
| topics/memory-and-state/index.md | 18 | `agent-architecture/` | topics/agent-architecture |
| topics/memory-and-state/index.md | 20 | `experiments/` | experiments |
| topics/memory-and-state/index.md | 19 | `patterns/` | patterns |
| topics/multi-agent-systems/AGENTS.md | 51 | `../../patterns/` | patterns |
| topics/multi-agent-systems/AGENTS.md | 51 | `../agent-architecture/` | topics/agent-architecture |
| topics/multi-agent-systems/AGENTS.md | 3 | `topics/multi-agent-systems/` | topics/multi-agent-systems |
| topics/multi-agent-systems/coordination-patterns.md | 288 | `../../notes/agent-interview-questions.md` | notes/agent-interview-questions.md |
| topics/multi-agent-systems/coordination-patterns.md | 14 | `../../patterns/case-study-bondseek-multi-agent.md` | patterns/case-study-bondseek-multi-agent.md |
| topics/multi-agent-systems/coordination-patterns.md | 14 | `../../patterns/case-study-bondseek-observability.md` | patterns/case-study-bondseek-observability.md |
| topics/multi-agent-systems/coordination-patterns.md | 14 | `../agent-architecture/cognitive-architecture-paradigms.md` | topics/agent-architecture/cognitive-architecture-paradigms.md |
| topics/multi-agent-systems/coordination-patterns.md | 14 | `../tool-use/function-calling.md` | topics/tool-use/function-calling.md |
| topics/multi-agent-systems/index.md | 17 | `./coordination-patterns.md` | topics/multi-agent-systems/coordination-patterns.md |
| topics/multi-agent-systems/index.md | 21 | `agent-architecture/` | topics/agent-architecture |
| topics/multi-agent-systems/index.md | 23 | `experiments/` | experiments |
| topics/multi-agent-systems/index.md | 22 | `patterns/` | patterns |
| topics/observability-and-tracing/AGENTS.md | 75 | `../agent-architecture/` | topics/agent-architecture |
| topics/observability-and-tracing/AGENTS.md | 76 | `../evaluation-and-benchmarks/` | topics/evaluation-and-benchmarks |
| topics/observability-and-tracing/AGENTS.md | 77 | `../tool-use/` | topics/tool-use |
| topics/observability-and-tracing/AGENTS.md | 6 | `index.md` | topics/observability-and-tracing/index.md |
| topics/observability-and-tracing/AGENTS.md | 5 | `topics/observability-and-tracing/` | topics/observability-and-tracing |
| topics/observability-and-tracing/index.md | 15 | `agent-architecture/` | topics/agent-architecture |
| topics/observability-and-tracing/index.md | 16 | `evaluation-and-benchmarks/` | topics/evaluation-and-benchmarks |
| topics/observability-and-tracing/index.md | 17 | `experiments/` | experiments |
| topics/planning-and-decomposition/AGENTS.md | 52 | `../tool-use/invocation-patterns.md` | topics/tool-use/invocation-patterns.md |
| topics/planning-and-decomposition/AGENTS.md | 10 | `index.md` | topics/planning-and-decomposition/index.md |
| topics/planning-and-decomposition/AGENTS.md | 3 | `topics/planning-and-decomposition/` | topics/planning-and-decomposition |
| topics/planning-and-decomposition/index.md | 15 | `agent-architecture/` | topics/agent-architecture |
| topics/planning-and-decomposition/index.md | 16 | `llm-reasoning/` | topics/llm-reasoning |
| topics/planning-and-decomposition/index.md | 17 | `patterns/` | patterns |
| topics/prompt-engineering/AGENTS.md | 10 | `index.md` | topics/prompt-engineering/index.md |
| topics/prompt-engineering/AGENTS.md | 3 | `topics/prompt-engineering/` | topics/prompt-engineering |
| topics/prompt-engineering/index.md | 17 | `experiments/` | experiments |
| topics/prompt-engineering/index.md | 15 | `llm-reasoning/` | topics/llm-reasoning |
| topics/prompt-engineering/index.md | 16 | `tool-use/` | topics/tool-use |
| topics/safety-and-alignment/AGENTS.md | 47 | `../evaluation-and-benchmarks/` | topics/evaluation-and-benchmarks |
| topics/safety-and-alignment/AGENTS.md | 47 | `../fine-tuning-for-agents/` | topics/fine-tuning-for-agents |
| topics/safety-and-alignment/AGENTS.md | 47 | `../prompt-engineering/` | topics/prompt-engineering |
| topics/safety-and-alignment/AGENTS.md | 3 | `topics/safety-and-alignment/` | topics/safety-and-alignment |
| topics/safety-and-alignment/index.md | 16 | `evaluation-and-benchmarks/` | topics/evaluation-and-benchmarks |
| topics/safety-and-alignment/index.md | 15 | `prompt-engineering/` | topics/prompt-engineering |
| topics/safety-and-alignment/index.md | 17 | `resources/papers.md` | resources/papers.md |
| topics/tool-use/AGENTS.md | 51 | `../agent-architecture/cognitive-architecture-paradigms.md` | topics/agent-architecture/cognitive-architecture-paradigms.md |
| topics/tool-use/AGENTS.md | 10 | `index.md` | topics/tool-use/index.md |
| topics/tool-use/AGENTS.md | 13 | `invocation-patterns.md` | topics/tool-use/invocation-patterns.md |
| topics/tool-use/AGENTS.md | 3 | `topics/tool-use/` | topics/tool-use |
| topics/tool-use/function-calling.md | 454 | `../../notes/agent-interview-questions.md` | notes/agent-interview-questions.md |
| topics/tool-use/function-calling.md | 14 | `../../patterns/case-study-bondseek-multi-agent.md` | patterns/case-study-bondseek-multi-agent.md |
| topics/tool-use/function-calling.md | 453 | `../../patterns/case-study-bondseek-observability.md` | patterns/case-study-bondseek-observability.md |
| topics/tool-use/function-calling.md | 14 | `../../patterns/case-study-bondseek-rag.md` | patterns/case-study-bondseek-rag.md |
| topics/tool-use/function-calling.md | 14 | `../agent-architecture/cognitive-architecture-paradigms.md` | topics/agent-architecture/cognitive-architecture-paradigms.md |
| topics/tool-use/index.md | 17 | `./function-calling.md` | topics/tool-use/function-calling.md |
| topics/tool-use/index.md | 18 | `./invocation-patterns.md` | topics/tool-use/invocation-patterns.md |
| topics/tool-use/index.md | 23 | `agent-architecture/` | topics/agent-architecture |
| topics/tool-use/index.md | 24 | `experiments/` | experiments |
| topics/tool-use/index.md | 22 | `prompt-engineering/` | topics/prompt-engineering |
| topics/tool-use/invocation-patterns.md | 308 | `../../experiments/go-agent/agent/agent.go` | experiments/go-agent/agent/agent.go |
| topics/tool-use/invocation-patterns.md | 323 | `../../experiments/go-agent/agent/api.go` | experiments/go-agent/agent/api.go |
| topics/tool-use/invocation-patterns.md | 312 | `../../experiments/go-agent/agent/fault_test.go` | experiments/go-agent/agent/fault_test.go |
| topics/tool-use/invocation-patterns.md | 265 | `../../experiments/go-agent/agent/sse.go` | experiments/go-agent/agent/sse.go |
| topics/tool-use/invocation-patterns.md | 307 | `../../experiments/go-agent/agent/types.go` | experiments/go-agent/agent/types.go |
| topics/tool-use/invocation-patterns.md | 14 | `../../patterns/case-study-bondseek-sse-recovery.md` | patterns/case-study-bondseek-sse-recovery.md |
| topics/tool-use/invocation-patterns.md | 14 | `../agent-architecture/cognitive-architecture-paradigms.md` | topics/agent-architecture/cognitive-architecture-paradigms.md |
| topics/tool-use/invocation-patterns.md | 14 | `./function-calling.md` | topics/tool-use/function-calling.md |
| topics/tool-use/invocation-patterns.md | 15 | `experiments/go-agent/agent/` | experiments/go-agent/agent |
| topics/tool-use/invocation-patterns.md | 371 | `experiments/go-agent/agent/agent.go` | experiments/go-agent/agent/agent.go |
| topics/tool-use/invocation-patterns.md | 246 | `patterns/case-study-bondseek-multi-agent.md` | patterns/case-study-bondseek-multi-agent.md |

---

> 本报告由脚本生成，禁止手工修改；修正问题后重跑 `node topics/check-pledges.mjs` 更新。
> 验收标准（SPEC §8）：❌ 缺失 = 0 且 ⚠️ 污染 = 0。
