---
title: 循环收敛控制：重复检索与多 Agent 交接治理
tags: [agent-architecture, loop-control, convergence, duplicate-search, handoff, degradation, observability, bondseek]
created: 2026-11-03
updated: 2026-11-03
status: draft
---

## 🎯 TL;DR 冲突摘要（先看这段）

> **这篇文档解决的核心矛盾：Agent 每一轮都"看起来在工作"，但它不知道自己查过什么、也不知道查到什么程度算够；多 Agent 协作又没有唯一负责人和交接状态，于是任务要么在同一个抽屉里反复翻找，要么在几个 Agent 之间来回推。** 破局思路一句话：**把"停不停"从模型的自我判断里拿走，交给代码**——用硬上限兜底、用动作指纹去重、用可量化的收敛判据决定"够了"，再用 Supervisor 权责结构从源头禁止回派；实在收不了尾，就诚实返回部分结果或交给人。读完全文带走一个判据：**如果每轮没有新增证据或产出，那它就是在绕圈；步数涨得再多也不算进展。**

**这题到底在防什么：** 防的不是"模型想多做几步"，而是三类失控——同一动作无新信息地重复执行、任务所有权在 Agent 之间来回转移、以及"完成"没有可判定的终点。三者都会烧钱、拖长响应，最后还给用户一个看起来完整、实际没查干净的答案。

---

## 1. 两类循环：定义与根因

> 这一段讲：重复搜索和多 Agent 交接，表面都叫"循环"，但根因不同，处理手段也不能互换。

先把两个词说清楚。**Agent**（智能体）可以理解成"会自己决定下一步做什么、还能动手调用外部能力"的程序；普通程序按写死的步骤跑，Agent 每轮让模型看一眼现状再决定下一步。**工具调用**（Function Calling）是模型不直接动手，而是输出"请执行某个工具、参数是什么"，由外部代码真正执行，再把结果喂回去——这样模型负责决策、代码负责执行，能力开放但边界可控。**上下文**（Context）是模型这一轮能看到的全部文字（对话、工具结果、系统说明），长度有上限，所以每一轮塞什么进去都要挑。

### 1.1 自我循环：一个 Agent 反复搜同一件事

生活画面：一个人反复拉开**同一个抽屉**找同一份文件，每次看到的还是那几张纸；他不记得自己刚拉过，因为抽屉内容没变，也没人在旁边记一笔"你查过了"。

对应到 Agent，根因有五类：

| 根因 | 运行时的样子 | 缺了什么 |
|------|-------------|---------|
| **无状态、无记录** | 同一工具、同一参数反复执行，结果一模一样 | 没有"已执行动作"的记录 |
| **换词重搜** | "XX 债券评级"改成"XX 债项级别"再搜一次 | 只做精确相等判断，抓不住改述 |
| **目标不可达** | 库里根本没有这份资料，模型不肯承认查不到 | "确认无结果"的退出路径 |
| **错误信息无用** | 工具报错只回一句"参数错误"，模型只能盲试 | 写给模型看的错误说明 |
| **没有"够了"的定义** | 永远觉得还差一点，继续搜 | 可判定的收敛条件 |

其中"换词重搜"最容易被低估：在模型看来这是新问题，在系统看来仍是同一件事。只比较参数字符串，会漏掉大量重复。

### 1.2 交接循环：几个 Agent 把活推来推去

生活画面：两个同事互相把活推给对方——"这块你更专业""可数据在你那儿"，活转一圈回到原点，谁都没真正开始干。

先解释两个词。**子 Agent**（SubAgent）是被主 Agent 派出去干专项任务的 Agent，有自己的提示词、工具集和轮数上限。**Supervisor**（监工模式）是一种组织方式：一个主 Agent 负责理解需求、派活、收尾，其他 Agent 只干专项活、不指挥别人。

交接循环的根因有四类：

1. **权责不清。** 没有唯一负责人，每个 Agent 都默认"这事该对方收尾"；子 Agent 如果也能把任务再派回去，环就形成了。
2. **交接丢状态。** 只递了原始任务单，没递"已经试过哪几种查法、卡在哪"，接收方只能从头再来，于是又搜一遍。这不是记性差，是交接包（handoff，把任务和现状转交给另一个 Agent）缺了"已尝试清单"。
3. **验收标准主观。** A 觉得不够好就退回 B，B 改完再退回 A；"好"没有可判定标准，循环就没有终点。
4. **拓扑鼓励打转。** Peer 模式（Agent 平级直接对话、没有中心）像一间没有主持人的会议室：谁都能发言，没人负责散会。参见 [`coordination-patterns.md`](../multi-agent-systems/coordination-patterns.md) §4.3 与文末"反模式"。

两类循环的共同点是：**每一轮都在消耗资源，但没有新增信息。** 后面所有检测与收敛手段，都建立在这一点上。

---

## 2. 怎么发现它在绕圈

> 这一段讲：运行中靠哪些信号判断"它在原地打转"，而不是等用户投诉或账单暴涨才发现。

先解释**调用树**：普通日志是一条条平铺的事件，看不出"谁调用了谁"；调用树把每次工具调用按父子关系记下来，能看出某个 `read_file` 是主 Agent 调的，还是某个子 Agent 调的。BondSeek 用 `ToolCallRecorder` 记录这棵树，见 [`case-study-bondseek-observability.md`](../../patterns/case-study-bondseek-observability.md) §4 横切关注点 3：工具调用树。

### 2.1 硬信号：可以直接写成规则的

- 同一工具名 + 归一化后的同一组参数，在一次任务里重复出现 ≥N 次；
- 同一子 Agent 在一次任务里被调用超过 N 次；
- 交接边 `A → B → A` 重复出现，或交接深度超过预设层数；
- 调用树节点数、轮数持续上涨，但产出文件数、检索到的新证据数不涨；
- 连续两轮检索结果集合重叠率过高（例如 >80%），说明没查到新东西。

### 2.2 软信号：用来解释"为什么贵、为什么慢"

token（模型计费和计量的文字单位）消耗持续上涨、步骤数上涨、自反思轮数上涨，但质量分数不升。**自反思**指模型生成答案后，再让模型自己当质检员检查一遍，不合格就带着具体问题重做——它解决"模型不知道自己错了"，但不解决"模型不知道自己已经查过了"。

### 2.3 BondSeek 能看到什么（以及没做什么）

**文档明写的：** 调用树用于故障定位、成本审计、质量复盘，其中"质量复盘"明确写了"看调用树哪里绕了远路"（可观测性 §4.3）；"反思轮数"被记成指标 `debug.ReflectionRounds++`，检索中间数字通过 `QueryDebugInfo` 返回（同文 §5.2、§6）；请求审计记录"谁在什么时候调了什么"（同文 §2）。

**文档没写的：** 在线检测"同一动作重复 N 次"并自动判定循环的阈值与规则。BondSeek 具备"能看见"的地基（调用树 + 指标 + 审计日志），但"自动识别绕圈并处置"这一步属于推断，不是已落地事实。别把"能回放"讲成"能自动刹车"。

---

## 3. 怎么把它停下来

> 这一段讲：发现绕圈之后的处置顺序——先用最可靠的硬上限，再用去重处理"换词重搜"，然后用收敛判据解决"不知道够没够"，最后用权责结构从源头消灭交接环。

### 3.1 硬上限：成本最低，也最不容易被模型绕过

**熔断**（Circuit Break）指达到预设条件就强制切断，不再继续。BondSeek 文档里明确写了四道：

| 防护 | 机制 | 定位 |
|------|------|------|
| 最大迭代轮数 | 主 Agent 12/18 轮；技术面专家 40 轮（实测典型任务约 27 步后留余量） | [多 Agent 案例](../../patterns/case-study-bondseek-multi-agent.md) §1.2、§4.1、§8.4 细节 1 |
| 工具 3 连败硬拦截 | 同一工具连续失败 3 次即停止执行，并注入"停止重试，说明不可用后完成任务" | 同上 §4.2 三个工程级防护 |
| 预算熔断 | token 超限后强制 `tool_choice="none"`，禁止再调工具、逼它收尾 | 同上 §4.3 中间件机制 |
| 自反思轮数上限 | 充分性检查受 `MaxSupplementRounds` 限制；幻觉检测受 `MaxRegenerateRounds` 限制 | [防幻觉案例](../../patterns/case-study-bondseek-hallucination.md) §2.2、§2.3 |

上限的代价必须讲清楚：它只防失控，不保证任务完成。任务本来就复杂时，到轮数会被误判为失败。BondSeek 用"宽容完成"缓解——达到上限但已有产出文件也算成功（多 Agent §4.2；[总纲](./from-zero-to-agent.md) ⑦ 防失败）。代价是可能掩盖"有文件但结论不对"，文档自己也承认这一点。

### 3.2 动作指纹去重：解决"换个说法又搜一遍"

**指纹**（Fingerprint）在这里指给一次动作算出的短标识：把工具名、归一化后的参数、必要时加上关键结果 ID 拼起来做哈希。同一个指纹第二次出现，就直接返回"这一步已经做过，这是上次的结果或摘要"，不再真正执行。

归一化要处理大小写、空格、同义改写；更稳的是叠加"结果集指纹"——返回的证据 ID 集合高度重叠，就按同一次算。

**这是业界建议，不是 BondSeek 现有实现。** 知识库里出现的"去重"都不是执行层去重：一处是上下文裁剪时"去重工具只留最后一次"（[记忆案例](../../patterns/case-study-bondseek-memory.md) 阶段 1 剪枝），另一处是 RAG 多路召回后"按 ChunkID 去重合并"（[RAG 案例](../../patterns/case-study-bondseek-rag.md) 的关系合并与并行检索部分）——它们是压缩与结果合并，不阻止同一动作再次执行。[`function-calling.md`](../tool-use/function-calling.md) §5.1 把"相同动作熔断"列为缓解手段，但没有落到 BondSeek 的代码或案例，属于理论建议。

代价同样成立：缓存占内存、要设过期时间；语义去重会误杀"补充了新信息之后的合理重搜"；去重范围必须按任务隔离，不能全局去重——不同用户问同一个问题，本来就应该各查一次。所以更稳的用法是"配额"而不是"一律禁止"。

下面是一段 Go 风格伪代码，**示意写法，不是 BondSeek 现有实现**，挂在 `ExecutionEngine` 的工具执行处与 `BeforeReasoning` 钩子上：

```go
// 示意：动作指纹去重 + 单任务检索配额。非 BondSeek 现有实现。
type LoopGuard struct {
    seen        map[string]string // fingerprint -> 上次结果摘要
    searchUsed  int
    searchLimit int
}

func (g *LoopGuard) BeforeTool(ctx context.Context, call ToolCall) (skip bool, cached string) {
    fp := fingerprint(call.Name, normalize(call.Args)) // 归一化参数后哈希
    if prev, ok := g.seen[fp]; ok {
        return true, prev // 命中：不执行，直接把上次结果当 Observation 喂回
    }
    if call.Name == "web_search" && g.searchUsed >= g.searchLimit {
        return true, "检索配额已用完，请基于现有证据作答或说明缺口"
    }
    return false, ""
}

func (g *LoopGuard) AfterTool(call ToolCall, result ToolResult) {
    fp := fingerprint(call.Name, normalize(call.Args))
    g.seen[fp] = result.Output
    if call.Name == "web_search" {
        g.searchUsed++
    }
}
```

### 3.3 收敛判据：把"够不够"变成可比较的数字

生活里的对应是：出门买菜前先写清单，买齐就回家，而不是在菜市场一直逛。Agent 也需要一张"证据清单"。

BondSeek 有一个接近的现成机制：**充分性检查**。检索完先让模型给资料打 5 个维度、总分 0–10 的分，8 分以上算"够"；不够就让模型生成"还缺什么"的问题，做一次补充检索，补充检索只取一半候选（防止淹没主结果），并且限制轮数。见防幻觉案例 §2.2 循环 1：信息充分性检查。它的思路是"先问够不够，再开始生成"，本质是把"停不停"变成"分数 + 阈值 + 轮数上限"。

多 Agent 一侧也有轻量版本：主 Agent 的系统提示词写了"调用前自检——本轮调过 visualization_expert 吗？没有就回去"，见多 Agent 案例 §6.1 OrchestratorStaticPrompt 拆解。这是把流程完成度写成可核查的清单，但属于提示词软约束，模型可能跳过，不能当硬保证。补充说明，**RAG**（检索增强生成）指"先去资料库找证据，再让模型基于证据回答"，充分性检查就是它的检索后质检环节。

我还会再加一条硬判据：**连续两轮没有新增证据（没有新 chunk、新产出文件、新指标），就判定无增益，直接收尾。** 这条不需要模型同意。

### 3.4 权责结构：用拓扑消灭交接环，而不是事后检测

最有效的办法往往不是"检测到 A→B→A 就打断"，而是**一开始就不允许这条边存在**。

BondSeek 的做法是把每个子 Agent 包装成一个普通工具，主 Agent 用"工具调用"派活，子 Agent 在全新子会话里执行，只回"摘要 + 产出文件路径"，不把对话让渡出去。见多 Agent案例 §3 SubAgentTool、§3.3 父子会话机制、§3.4 结果瘦身，以及 [`coordination-patterns.md`](../multi-agent-systems/coordination-patterns.md) §3.4 BondSeek 落地。

为什么能治交接循环：子 Agent 只能"返回结果"，不能"把任务转交出去"，所以不存在 A→B→A 的交接边。这是用结构消灭循环，比事后检测更可靠。

代价是明确的：主 Agent 成为瓶颈和单点，它挂了整个流程就停（coordination-patterns §3.3）；Peer 模式那种灵活碰撞没有了；层级越深，延迟和信息失真越多（同文 §5.3）。

如果业务上确实必须交接（例如跨模块转交给另一个 Agent），交接包至少要带四样东西：**已尝试清单、已有证据、未完成项、验收标准**，再给交接次数一个硬上限。BondSeek 文档只写到 `dispatch` 工具与 `task_description`，没有写交接配额，也没有写"已尝试清单"——**这是知识库的缺口，不应讲成已有实现。**

---

## 4. 停下来之后怎么收场

> 这一段讲：刹车踩下去以后，怎么给用户一个诚实的交代，而不是假装成功。

- **部分结果 + 明说缺什么。** 防幻觉案例 §2.4 里，有用性分数低于阈值就把状态标成 `partial`，同时返回"建议你补充什么"，置信度也一起返回，下游可据此提示风险；§5.1 三层防线图写得更直接：全部失败时"status/confidence 如实上报（不装成功）"。
- **宽容完成，但标注边界。** 有产出文件就算完成，避免"跑满轮数 = 失败"的误判；代价是有文件不等于结论正确，总纲 ⑦ 的代价段已承认。
- **失败说明写成人话。** 多 Agent案例 §6.1 的系统提示词规定：工具反复报错就直接说关键错误；达到最大迭代就说明任务未完成并列出已产出文件；没有产物就说清失败阶段和原因，不要声称已完成。
- **交给人。** 先解释 **HITL**（Human-in-the-Loop，人在环路：执行到关键节点暂停，等人确认或补充信息再继续）。这几篇文档里没有 BondSeek 的人机接管实现；知识库只在 [`compare-langchain-langgraph.md`](../agent-frameworks/compare-langchain-langgraph.md) 的交互对比里提到 LangGraph 有 `interrupt/resume` 原语，`topics/human-agent-interaction/` 目前只有目录约定。因此"收敛不了就转人工"是建议，不是 BondSeek 现状。

---

## 5. BondSeek 事实核对表

> 这一段讲：把上面的说法按"文档明写 / 我据机制推断 / 知识库缺口"三分类归位，避免把推断讲成事实。

| 结论 | 性质 | 章节定位 |
|------|------|---------|
| 主 Agent 12/18 轮，技术面专家 40 轮（实测 27 步后留余量） | **文档明写** | 多 Agent案例 §1.2、§4.1、§8.4 细节 1 |
| 同一工具连续失败 3 次硬拦截，并注入"停止重试" | **文档明写** | 多 Agent案例 §4.2 三个工程级防护 |
| token 超预算强制 `tool_choice="none"` 收尾 | **文档明写** | 多 Agent案例 §4.3 中间件机制 |
| 充分性/幻觉检测有轮数上限，有用性不足返回 `partial` | **文档明写** | 防幻觉案例 §2.2、§2.3、§2.4 |
| 调用树能看层级、耗时、绕远路；反思轮数进指标 | **文档明写**（作为观测用途） | 可观测性案例 §4、§4.3、§5.2、§6 |
| Supervisor + 子 Agent 工具化 + 只回摘要，结构上没有回派边 | **文档明写** | 多 Agent案例 §3、§3.3、§3.4；coordination-patterns §3.4 |
| 达到上限但有产出文件算成功（宽容完成） | **文档明写** | 多 Agent案例 §4.2；总纲 ⑦ |
| 相同动作指纹去重、单任务检索配额 | **我据机制推断** | 知识库无 BondSeek 实现；仅 function-calling §5.1 列为理论缓解 |
| 在线循环检测的阈值与规则 | **我据机制推断** | 可观测性只写到回放/可视化/成本审计，未写自动判定 |
| 交接次数上限、交接包里的"已尝试清单" | **知识库缺口** | 多 Agent案例只写到 `dispatch` 与 `task_description` |
| 收敛不了转人工（HITL） | **知识库缺口 / 建议** | 这几篇无 BondSeek 实现；仅框架对比文档提到 interrupt/resume |

一句话：**BondSeek 的强项是"硬上限 + 失败降级 + 结构上禁止回派"，缺口正好是"重复搜索的执行层去重"和"交接次数/交接状态"的治理。**

---

## 6. 行业通行方案（BondSeek 之外）

> 这一段讲：BondSeek 之外，主流框架和观测工具把"防绕圈"做到了哪一步。下面不按厂商名单罗列，而是按本文已有的四层治理对号入座：**检测 → 硬上限 → 去重与收敛判据 → 权责结构**。先给结论：**框架默认配置基本只提供"硬刹车"，能挡住无限执行，挡不住"白跑"；去重和"够了没有"的判断，大多要自己补。** 以下默认值均为 2026-11-03 检索官方文档所得，版本差异单独标注。

### 6.1 默认配置能挡住什么、挡不住什么

**挡得住"停不下来"：**

- 图执行超过步数上限，抛异常（LangGraph）
- 连续自动回复到顶就停（AutoGen / AG2）
- 单个 Agent 迭代到顶，强制交"best answer"（CrewAI）
- 一次运行的总轮数到顶，抛错（OpenAI Agents SDK）

**挡不住"白跑"：**

- 两轮检索返回同一批文档，token 照烧
- A→B→A 把同一任务来回传，每一棒都"合法"
- 摘要越写越长，但结论没变

原因很直接：默认参数数的是**次数、时长、token**，不是**每轮有没有新增信息**。次数合规，不等于收敛。还有三个坑：

1. **默认值可能偏小。** LangGraph SDK 的 `recursion_limit` 默认 25 个 superstep，长任务很容易撞上；LangSmith Agent Server 自托管环境变量 `LANGGRAPH_DEFAULT_RECURSION_LIMIT` 的默认值是 10011，部署形态不同，数值差了两个数量级。
2. **默认值可能偏大，且口径模糊。** AutoGen 0.2 的 `max_consecutive_auto_reply` 构造参数传 `None` 时会落到类属性 100；它只数"连续自动回复"，人一插话计数就重置。
3. **版本会漂移。** CrewAI 当前文档 `max_iter` 默认 20，较早文档写 25；AutoGen 0.2 的 `GroupChat.max_round` 默认 10，0.5 的 `BaseGroupChat.max_turns` 默认 `None`，只有 `MagenticOneGroupChat` 是 20。照抄旧博客会配错。

### 6.2 检测：先把循环画出来（对应本文 §2）

- **Langfuse Agent Graphs**：聚合视图把同名步骤合并成带计数的节点（`retrieve_docs (3/3)`），重复调用同一工具的循环直接画成环；展开视图再还原成逐个调用的 DAG。它解决"看见"，不负责"停下"。
- **LangSmith**：记录完整执行树（LLM、工具、检索），配合轨迹评估和离线评估。官方文档强调"看整条轨迹"，但截至 2026-11 检索，我没有找到明确声明"自动识别重复调用/循环"的内置能力；通常靠人看 trace，或自己写 evaluator 与告警。**这一条是"未找到"，不是"确认不存在"。**
- **OpenTelemetry GenAI 语义约定**：统一 `invoke_agent`、`chat`、`execute_tool` 等 span 与 `gen_ai.*` 属性，让不同框架的调用可比较；但它没有"循环/重复"的语义，检测得自己对 span 做聚合查询（例如同 `gen_ai.tool.name` + 参数哈希的计数、交接边成环）。规范已迁到独立仓库维护。

### 6.3 硬上限：框架自带的刹车（对应本文 §3.1）

- **LangGraph**：`recursion_limit` 限制图的 superstep 数，超过抛 `GraphRecursionError`，SDK 配置默认 25。它同时用显式 state / node / edge 约束"下一步谁能跑"，子图是父图里的一个节点，不存在"Agent 自己找人聊"的自由边。代价是图要预先设计，拿灵活性换可预测性。
- **AutoGen 0.2 / AG2**：`max_consecutive_auto_reply` 限制连续自动回复，源码类属性为 100，构造参数传 `None` 时用它；`GroupChat.max_round` 默认 10；`speaker_selection_method` 默认 `auto`（让 LLM 选下一个发言者），另有 `manual` / `random` / `round_robin`。它挡的是"聊不完"，不挡"同一件事反复聊"。
- **AutoGen AgentChat 0.5**：`BaseGroupChat.max_turns` 默认 `None`（不设上限，必须自己配 termination condition）；`MagenticOneGroupChat` 默认 20 轮、`max_stalls=3`。`SelectorGroupChat.allow_repeated_speaker` 默认 `False`，但它只是把上一个发言者从候选里拿掉，模型仍可能选同一个人（会打 warning）。
- **CrewAI**：`max_iter` 当前默认 20（较早文档 25），到顶强制给"best answer"；`max_rpm` 默认 `None`（不限速）；`allow_delegation` 默认 `False`——等于默认关掉"Agent 把活转给另一个 Agent"这条边。一旦打开 delegation，就要自己补交接预算。
- **OpenAI Agents SDK**：`max_turns` 默认 10，超过抛 `MaxTurnsExceededError`；一个 turn 是一次模型调用（含工具），handoff 也计入。handoff 在 SDK 里就是一个工具（`transfer_to_xxx`），默认把累积对话历史整体交给下一个 Agent，可用 `input_filter` 裁剪；A→B→A 仍可能发生，SDK 只保证总轮数上限，不保证收敛。guardrails 的 tripwire 是策略闸门：输入/输出/工具检查不过就立刻抛异常停执行，它管安全，不管重复。
- **Anthropic 多 Agent 研究系统**：没有框架级硬上限，文章给的是提示词里的规模规则——简单事实 1 个 Agent、3–10 次工具调用；直接对比 2–4 个子 Agent、每个 10–15 次；复杂研究 10+ 个子 Agent。早期曾出现"简单问题开 50 个子 Agent"的错误。子 Agent 只回压缩后的 findings，或用文件 / artifact 传引用，避免把长输出抄回主上下文。代价是多 Agent 比普通 chat 多约 15 倍 token。

### 6.4 去重与收敛判据：默认几乎是空白（对应本文 §3.2 / §3.3）

- **语义缓存**：GPTCache 把 query 做 embedding，在向量库里找相似问题，超过阈值直接返回缓存答案；阈值依赖 `similarity_evaluation`，README 示例是 `similarity_threshold=0.2`（距离型，越小越相似），没有万能默认值。误命中是主要代价：问题相似但约束不同，会拿到错误答案。
- **幂等键**：Stripe 的做法是客户端生成 key，重试同 key 返回第一次结果，v1 至少保留 24 小时。工具调用可以借这个思路，用 `(task_id, tool_name, normalized_args)` 做幂等键；它防"同一请求重放"，不防"换词重搜"。
- **工具调用配额**：LangChain 的生产 Deep Agents 运行时内置 `ToolCallLimitMiddleware`，按工具调用次数设限。它是纯计数，不做语义相似判断，适合当第二道闸而不是唯一手段。
- **终止条件**：AutoGen 提供 `MaxMessageTermination`、`TextMentionTermination`、`TokenUsageTermination`、`TimeoutTermination`、`HandoffTermination`、`ExternalTermination`，可用 `|` 组合。它们能表达"多少条、多少 token、多久、出现某个词、转交给某人"，但表达不了"证据够了没有"。
- **收敛质量**：这一层框架基本不管。LangGraph 要自己在 conditional edge / state 里写"够了就 END"；AutoGen 要自己写 termination function；CrewAI 靠 `max_iter` 到顶交答案。Anthropic 的做法是把"按复杂度定子 Agent 数量"写进提示词，再由 lead agent 判断是否继续研究——是建议，不是硬保证。

### 6.5 权责结构：状态机、handoff 与 orchestrator-worker（对应本文 §3.4）

- **LangGraph 状态机**：谁下一个执行由 edge 决定，子图作为节点嵌进父图，输入输出有 schema。它从结构上不允许"两个 Agent 私下自由聊"，交接状态可以显式放进 state（比如 `tried_actions`）。代价是前期要把状态和边设计对，否则只是把混乱写进图里。
- **OpenAI handoffs**：适合分诊、客服这类"转到更专业的人"的场景；代价是默认全量历史随行、上下文膨胀，而且没有内建的"已尝试清单"，A→B→A 主要靠 `max_turns` 兜底。
- **AutoGen GroupChat**：Peer 式自由群聊，默认让 LLM 选下一个发言者；灵活，但没有全局收敛责任人，容易变成"没有主持人的会议室"。
- **Anthropic orchestrator-worker**：lead agent 派活、子 Agent 只回摘要或产物引用，结构上接近 BondSeek 的 Supervisor + 工具化，但没有"禁止回派"的硬约束，靠提示词和架构约定。

### 6.6 对照表

| 机制 | 代表实现 | 典型默认值 / 上限 | 代价或局限 | 适用场景 |
|------|---------|------------------|-----------|---------|
| 图步数上限 | [LangGraph `recursion_limit`](https://reference.langchain.com/python/langgraph-sdk/schema/Config/recursion_limit) | SDK 25 个 superstep；Agent Server 环境默认 10011 | 只防无限执行；异常要捕获；不判断质量 | 图 / 状态机式 Agent |
| 连续自动回复上限 | [AutoGen / AG2 `max_consecutive_auto_reply`](https://docs.ag2.ai/latest/docs/user-guide/advanced-concepts/ending-a-chat/) | 100（类默认；传 `None` 落到它） | 只数连续回复；人插话重置；不识别语义重复 | 双人 / 小群聊 |
| 群聊轮数 | [AutoGen `GroupChat.max_round`](http://microsoft.github.io/autogen/0.2/docs/reference/agentchat/groupchat/) / [AgentChat `max_turns`](http://microsoft.github.io/autogen/0.5.2/reference/python/autogen_agentchat.teams.html) | 0.2：`max_round=10`；0.5：`BaseGroupChat=None`，`MagenticOne=20` + `max_stalls=3` | 封顶不封"重复"；`auto` 选人仍可来回 | 角色扮演式协作 |
| 单 Agent 迭代上限 | [CrewAI `max_iter`](https://docs.crewai.com/en/concepts/agents) | 20（当前文档；较早文档 25） | 到顶交 best answer，正确性无保证 | 任务型 Crew |
| 总轮数（含 handoff） | [OpenAI Agents SDK `max_turns`](https://openai.github.io/openai-agents-python/ref/result/) | 10，超过抛 `MaxTurnsExceededError` | 历史默认全量传递；不保证交接收敛 | 客服 / 分诊 |
| 策略闸门 | [OpenAI guardrails tripwire](https://openai.github.io/openai-agents-python/guardrails/) | 无默认阈值，需自定义 | 命中即停；管安全不管重复 | 合规 / 敏感操作 |
| 语义缓存 | [GPTCache](https://github.com/zilliztech/GPTCache) | 阈值按评估器；README 示例 0.2（距离型） | 误命中、跨用户串味、要设 TTL | 高频相似查询 |
| 幂等键 | [Stripe idempotency key](https://docs.stripe.com/api/idempotent_requests) | v1 至少保留 24 小时 | 防重放，不防换词 | 有副作用的工具 |
| 工具调用配额 | [LangChain `ToolCallLimitMiddleware`](https://www.langchain.com/conceptual-guides/runtime-behind-production-deep-agents) | 按配置 | 纯计数，无语义判断 | 长任务 / Deep Agents |
| 终止条件 | [AutoGen termination](http://microsoft.github.io/autogen/0.4.0/user-guide/agentchat-user-guide/tutorial/termination.html) | 六种条件自行组合 | 语法层，表达不了"证据够" | 群聊 / 长任务 |
| 子 Agent 规模 | [Anthropic orchestrator-worker](https://www.anthropic.com/engineering/built-multi-agent-research-system) | 3–5 并行；复杂 10+；对比 2–4 个 × 10–15 次调用（提示词建议） | 非强制；约 15× token | 开放式研究 |
| 循环可视化 | [Langfuse Agent Graphs](https://langfuse.com/docs/observability/features/agent-graphs) | 聚合视图计数 / 画环 | 只看见，不停 | 观测 / 复盘 |
| 标准 span | [OpenTelemetry GenAI semconv](https://opentelemetry.io/blog/2026/genai-observability/) | `invoke_agent` / `chat` / `execute_tool` | 无循环语义，需自定义查询 | 跨框架观测 |

### 6.7 从零搭的最小可行组合（按优先级）

1. **全局预算，而不是每个 Agent 各自的上限。** 一次任务的总轮数、总 token、总工具调用数，跨所有子 Agent 汇总，到顶强制收尾。**最常被漏的就是这条**——每个子 Agent 的 `max_iter` 都合规，合起来能把一次任务跑到几十轮。
2. **单一 owner + 结构化交接包。** 一个任务只有一个收尾人；必须交接时，包里带"目标、已尝试、已有证据、未完成、验收标准"，并给交接次数上限。
3. **动作指纹 + 幂等键。** `(task_id, tool, normalized_args)` 命中就返回上次结果；检索类工具再加单任务配额。按任务隔离，设 TTL。
4. **可判定的收敛条件。** 事前写"要哪几条证据算够"；再加一条"连续两轮无新增证据就停"。
5. **观测与告警。** trace 上聚合"同工具 + 同参数"计数、交接边环检测；没有告警，前面四层都只能事后复盘。
6. **HITL 兜底。** 收敛不了就暂停等人，或返回部分结果 + 缺失项。LangGraph 的 `interrupt()` + `Command(resume=...)` 是可用的原语。

如果只能补一条，补第 1 条；如果还能补第二条，补交接包里的"已尝试清单"。这两条最便宜，也最常缺席。

> **来源:** [LangGraph Config.recursion_limit](https://reference.langchain.com/python/langgraph-sdk/schema/Config/recursion_limit)
> **来源:** [LangGraph Graph API](https://docs.langchain.com/oss/python/langgraph/graph-api)
> **来源:** [LangGraph Subgraphs](https://docs.langchain.com/oss/python/langgraph/use-subgraphs)
> **来源:** [LangGraph Interrupts](https://docs.langchain.com/oss/python/langgraph/interrupts)
> **来源:** [LangSmith Agent Server 环境变量（recursion limit 默认 10011）](https://docs.langchain.com/langsmith/env-var-self-hosted)
> **来源:** [AutoGen 0.2 ConversableAgent（MAX_CONSECUTIVE_AUTO_REPLY）](http://microsoft.github.io/autogen/0.2/docs/reference/agentchat/conversable_agent/)
> **来源:** [AutoGen 0.2 GroupChat（max_round / speaker selection）](http://microsoft.github.io/autogen/0.2/docs/reference/agentchat/groupchat/)
> **来源:** [AutoGen 0.5.2 teams（max_turns / max_stalls）](http://microsoft.github.io/autogen/0.5.2/reference/python/autogen_agentchat.teams.html)
> **来源:** [AutoGen Termination 条件](http://microsoft.github.io/autogen/0.4.0/user-guide/agentchat-user-guide/tutorial/termination.html)
> **来源:** [AG2 Ending a chat](https://docs.ag2.ai/latest/docs/user-guide/advanced-concepts/ending-a-chat/)
> **来源:** [CrewAI Agents（max_iter / max_rpm / allow_delegation）](https://docs.crewai.com/en/concepts/agents)
> **来源:** [OpenAI Agents SDK Running agents](https://openai.github.io/openai-agents-python/running_agents/)
> **来源:** [OpenAI Agents SDK Results（max_turns 默认 10）](https://openai.github.io/openai-agents-python/ref/result/)
> **来源:** [OpenAI Agents SDK Handoffs](https://openai.github.io/openai-agents-python/handoffs/)
> **来源:** [OpenAI Agents SDK Guardrails](https://openai.github.io/openai-agents-python/guardrails/)
> **来源:** [Anthropic: How we built our multi-agent research system（2025-06-13）](https://www.anthropic.com/engineering/built-multi-agent-research-system)
> **来源:** [Langfuse Agent Graphs](https://langfuse.com/docs/observability/features/agent-graphs)
> **来源:** [LangSmith 查看 traces](https://docs.langchain.com/langsmith/view-traces)
> **来源:** [OpenTelemetry GenAI Observability（2026）](https://opentelemetry.io/blog/2026/genai-observability/)
> **来源:** [OpenTelemetry GenAI Semantic Conventions 仓库](https://github.com/open-telemetry/semantic-conventions-genai)
> **来源:** [GPTCache](https://github.com/zilliztech/GPTCache)
> **来源:** [Stripe Idempotent requests](https://docs.stripe.com/api/idempotent_requests)
> **来源:** [LangChain: The runtime behind production deep agents（2026-04-20）](https://www.langchain.com/conceptual-guides/runtime-behind-production-deep-agents)

---

## 7. 常见追问怎么答

> 这一段讲：面试官大概率会从上面几个取舍点往下追。

**问：让模型自己判断"该不该继续搜"不行吗？**
> 它对自己"查过了没有、够了没有"的自我判断不可靠，而且看不到真实预算。BondSeek 的底层原则是"LLM 不可信，确定性规则兜底"（[总纲](./from-zero-to-agent.md) 哲学一）。分工应是：模型判断"质量够不够"（可采样打分），代码判断"还能不能再来一轮"（计数器、指纹、配额）。

**问：去重会不会误杀合理的重搜？**
> 会，所以要限定范围。指纹按"任务 + 归一化参数 + 结果集变化"算；用配额而不是一律禁止；补充检索允许发生，但计入同一个预算。真正要防的是"没有新增信息的重复"。

**问：上限设多少合适？**
> 用真实任务测出来的步数乘 1.3–1.5，不拍脑袋。BondSeek 技术面专家实测 27 步、设 40 轮；主 Agent 只做调度，所以是 12/18 轮。角色复杂度不同，上限就该不同。

**问：多 Agent 让它们自由互聊不是更灵活吗？**
> 灵活，但没有全局视图，谁来收敛是难题；知识库把 Peer 的缺点直接写成"易失控、可能互相打转不收敛"（coordination-patterns §4.3）。生产环境用 Supervisor + 工具化包装 + 黑盒摘要，是拿灵活性换可控性。

**问：怎么区分"死循环"和"任务真的复杂"？**
> 看每轮有没有新增信息。步数涨、费用涨，但新证据、新产出文件不涨，就是绕圈；如果每轮都在产出可验证的新结果，那是任务本身复杂，应该调上限而不是打断。

---

## 8. 30 秒面试版

> "我会先把'停不停'从模型手里拿走，交给代码。重复搜索和多 Agent 交接的根因是两回事：前者是模型不知道自己查过、也不知道什么算够，后者是任务没有唯一负责人、交接时又丢了'试过什么'。所以我分三层处理。**第一层是硬上限**：每个角色有最大轮数，同一工具连续失败三次直接拦掉，token 超预算就禁止再调工具——这些 BondSeek 都实现了。**第二层是动作指纹去重和收敛判据**：同一工具加归一化参数只执行一次，命中就返回上次结果；检索前先定义'要哪几条证据算够'，连续两轮没有新增证据就收尾。这一层里，BondSeek 有充分性打分和阈值这套相近机制，但执行层去重和检索配额是我按机制补的，知识库没有落地记录。**第三层是权责结构**：一个任务只有一个负责人，用 Supervisor 把子 Agent 包成工具，只回摘要、不允许回派，从结构上消灭 A→B→A 的交接环。收不了尾就返回部分结果加缺失项，或者转人工，绝不假装完成。"

---

## 反模式（本主题避免）

- **只靠模型自觉停**——模型既看不到预算，也不可靠地记得自己查过什么
- **只设总轮数，不设单动作配额**——总轮数会被"换词重搜"合法消耗掉
- **精确参数去重**——改述、同义词、时间词变化都会漏过
- **全局去重**——不同用户/不同任务问同一个问题，本就应该各查一次
- **用提示词"禁止重复"当硬约束**——软约束会被模型跳过，必须有代码兜底
- **把"宽容完成"当成功**——有产出文件不等于结论正确，必须标注边界
- **多 Agent 自由互聊又没有收敛机制**——消息爆炸，且没人负责散会
- **交接只传原始任务**——接收方不知道已尝试什么，必然重做
- **把"能回放调用树"讲成"能自动检测循环"**——观测能力和处置能力是两件事

---

## 关联文档

- [`function-calling.md`](../tool-use/function-calling.md) — 工具调用闭环与工程坑；§5.1 无限循环、§6 工具执行失败处理
- [`case-study-bondseek-multi-agent.md`](../../patterns/case-study-bondseek-multi-agent.md) — 统一 ExecutionEngine 的轮数/3 连败/预算熔断；§3 工具化包装、§4 引擎防护、§8.4 迭代上限依据
- [`case-study-bondseek-hallucination.md`](../../patterns/case-study-bondseek-hallucination.md) — 自反思三重循环的重试上限与 `partial` 退出；§2.2–§2.4、§5.1
- [`case-study-bondseek-observability.md`](../../patterns/case-study-bondseek-observability.md) — 调用树、反思轮数指标、检索调试信息；§4、§5.2、§6
- [`coordination-patterns.md`](../multi-agent-systems/coordination-patterns.md) — Supervisor/Peer/Hierarchical 的收敛能力对比；§3.2–§3.4、§4.3、§5.3
- [`from-zero-to-agent.md`](./from-zero-to-agent.md) — 总纲 ⑦ 防失败与降级；哲学一"LLM 不可信 → 规则兜底"、哲学五"增强不拖垮主链路"
- [`index.md`](./index.md) — 本目录入口
- [`agent-interview-questions.md`](../../notes/agent-interview-questions.md) — 面试话题索引（第 19 题）
