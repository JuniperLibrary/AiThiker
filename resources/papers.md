# 原始论文与规范索引

> **用途:** topics/ 各话题主张的一手来源台账（topics/SPEC.md §4 原始素材清单、§9 待核 #6 的补建产物）。
> **状态规则:** 每条标注验证状态。⏳ = 条目信息（含 arXiv 编号）凭记忆登记，**未打开原文核对**；✅ = 已打开原文核对过编号与结论。任何话题文档引用某篇论文的具体论断前，该条目必须先变成 ✅，否则论断标 `pending_verification`。
> **评价标注:** ⭐必读 / 推荐 / 参考。

---

## Agent 架构与推理

| 论文 | 链接 | 发表 | 一句话总结 | 评价 | 状态 |
|---|---|---|---|---|---|
| ReAct: Synergizing Reasoning and Acting in Language Models (Yao et al.) | arXiv:2210.03629 | 2022-10 | 推理轨迹与动作交替，支撑 ReAct 范式 | ⭐必读 | ⏳ |
| Reflexion: Language Agents with Verbal Reinforcement Learning (Shinn et al.) | arXiv:2303.11366 | 2023-03 | 用语言反馈做"口头强化学习"，Reflection 范式来源 | ⭐必读 | ⏳ |
| Chain-of-Thought Prompting Elicits Reasoning in LLMs (Wei et al.) | arXiv:2201.11903 | 2022-01 | CoT 原始论文 | ⭐必读 | ⏳ |
| Self-Consistency Improves Chain of Thought Reasoning (Wang et al.) | arXiv:2203.11171 | 2022-03 | 多路径采样 + 多数投票 | 推荐 | ⏳ |
| Tree of Thoughts (Yao et al.) | arXiv:2305.10601 | 2023-05 | 搜索式推理（BFS/DFS + 状态评估） | 推荐 | ⏳ |
| Lost in the Middle (Liu et al.) | arXiv:2307.03172 | 2023-07 | 长上下文中部信息丢失，记忆/压缩话题依据 | ⭐必读 | ⏳ |

## 工具使用

| 论文 | 链接 | 发表 | 一句话总结 | 评价 | 状态 |
|---|---|---|---|---|---|
| Toolformer (Schick et al.) | arXiv:2302.04761 | 2023-02 | 自监督学习何时/如何调用 API | ⭐必读 | ⏳ |

## 记忆与多智能体

| 论文 | 链接 | 发表 | 一句话总结 | 评价 | 状态 |
|---|---|---|---|---|---|
| MemGPT (Packer et al.) | arXiv:2310.08560 | 2023-10 | 类操作系统分页的记忆管理 | ⭐必读 | ⏳ |
| Generative Agents (Park et al.) | arXiv:2304.03442 | 2023-04 | 记忆流 + 反思 + 规划的拟人智能体 | 推荐 | ⏳ |
| Voyager (Wang et al.) | arXiv:2305.16291 | 2023-05 | 技能库式终身学习 Agent | 推荐 | ⏳ |
| AutoGen (Wu et al.) | arXiv:2308.08155 | 2023-08 | 多 Agent 对话编排框架论文 | 推荐 | ⏳ |
| MetaGPT (Hong et al.) | arXiv:2308.00352 | 2023-08 | 角色分工 + SOP 的多 Agent 协作 | 参考 | ⏳ |

## 评估基准

| 论文 | 链接 | 发表 | 一句话总结 | 评价 | 状态 |
|---|---|---|---|---|---|
| GAIA (Mialon et al.) | arXiv:2311.12983 | 2023-11 | 通用 AI 助手基准，工具/多步/网页 | ⭐必读 | ⏳ |
| SWE-bench (Jimenez et al.) | arXiv:2310.06770 | 2023-10 | 真实 GitHub issue 修复基准 | ⭐必读 | ⏳ |
| AgentBench (Liu et al.) | arXiv:2308.03688 | 2023-08 | 八环境 Agent 综合评测 | 推荐 | ⏳ |
| τ-bench (Yao et al.) | arXiv:2406.12045 | 2024-06 | 工具-用户交互的客服域基准 | 推荐 | ⏳ |
| WebArena (Zhou et al.) | arXiv:2307.13854 | 2023-07 | 真实网站任务基准 | 参考 | ⏳ |

## 规范原文（非论文，同为判据）

| 规范 | 链接 | 用途 | 状态 |
|---|---|---|---|
| MCP Specification | modelcontextprotocol.io | 工具/资源协议判据 | ⏳ |
| OpenTelemetry GenAI 语义约定 | opentelemetry.io | 追踪字段判据 | ⏳ |
| WHATWG SSE | html.spec.whatwg.org/multipage/server-sent-events.html | 流式协议判据（§9.2 retry 字段 / Last-Event-ID / fail 不重连） | ✅ 2026-11-04 |
| JSON Schema | json-schema.org | 工具参数 schema 判据 | ⏳ |
| RFC 9110 Semantics | rfc-editor.org/rfc/rfc9110.txt | §9.2.2 幂等方法、§10.2.3 Retry-After、§15.3.3 202 | ✅ 2026-11-04 |
| RFC 6585 Additional HTTP Status Codes | rfc-editor.org/rfc/rfc6585.txt | §4 429 限流语义（含"不定义计数方式"盲区） | ✅ 2026-11-04 |

## 工程一手资料（源码 / 博客，非论文非规范）

| 资料 | 链接 | 用途 | 状态 |
|---|---|---|---|
| openai-python `_base_client.py` / `_constants.py` | github.com/openai/openai-python | 重试判定/退避公式/Retry-After 上限/幂等键的实现判据 | ✅ 2026-11-04 |
| anthropic-sdk-python `_base_client.py` / `_constants.py` | github.com/anthropics/anthropic-sdk-python | 与 openai 对照的重试实现（Retry-After 无条件服从） | ✅ 2026-11-04 |
| openai-openapi `openapi.yaml` | github.com/openai/openai-openapi | `parallel_tool_calls`、`tool_call_id` 绑定的协议判据 | ✅ 2026-11-04 |
| Exponential Backoff And Jitter（Marc Brooker, 2015） | aws.amazon.com/blogs/architecture/exponential-backoff-and-jitter/ | 四种抖动策略对照实验，"no-jitter = clear loser" | ✅ 2026-11-04 |
| openai-python issues #1102 / #4234 | github.com/openai/openai-python/issues | 一手失效/需求资料：默认退避不可调、Retry-After 测试近期才补 | ✅ 2026-11-04 |

## 安全

| 资料 | 链接 | 用途 | 状态 |
|---|---|---|---|
| OWASP Top 10 for LLM Applications | owasp.org | 威胁分类判据 | ⏳ |
| MITRE ATLAS | atlas.mitre.org | 对抗战术/技术判据 | ⏳ |

---

> **维护约定:** 新增条目必须填链接与状态；编号/链接错误属于事实性错误，发现即修。最后全量核对日期：2026-11-04（部分核对——`invocation-patterns.md` 引用的规范与工程资料 6+5 条已 ✅，论文条目仍全部 ⏳）。
