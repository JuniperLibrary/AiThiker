# 多 Agent 系统中的会话压缩：问题、实践与解决方案

> **最后更新:** 2026-08-13
> **状态:** Draft
> **标签:** `session-compression`, `context-management`, `multi-agent`, `state-persistence`
> **引用说明:** 本文档引用的行业实践基于以下来源的通用知识：(1) MemGPT/Letta 论文及开源代码；(2) LangChain/LangGraph 文档中的 Conversation Memory 设计；(3) OpenAI Assistants API Thread 管理文档；(4) StreamingLLM 论文 (Xiao et al., 2023)；(5) 本地安装的 Oh-My-OpenCode (ucode) 插件（v2026.5.7）的 TypeScript 类型声明文件和运行时数据——具体分析见 3.4 节。文中 Token 消耗数值为典型场景估算（标注了"~"前缀），实际值取决于具体任务复杂度。

## 📌 名词速查表（先看这里）

> 本文是"上下文压缩"主题的核心理论文档，英文术语非常密集。核心词先看这里（完整对照见文末"名词标注表"）：

| 英文 | 读作/理解成 | 一句话解释 |
|------|------------|-----------|
| **Session** | "会话" | 一次连续的对话过程（所有交互的时序记录） |
| **Token** | "词元" | 模型处理文本的最小单位（压缩就是省 token） |
| **Context Window** | "上下文窗口" | 模型一次最多能容纳的 token 总量 |
| **压缩 (Compression)** | 浓缩 | 把旧对话变成摘要，腾出窗口空间 |
| **剪枝 (Pruning)** | 裁剪 | 直接丢弃不重要的旧消息 |
| **摘要 (Summarization)** | 总结 | 用模型把旧内容浓缩成一段话 |
| **Multi-Agent** | "多智能体" | 多个 AI 协作完成任务的系统 |
| **KV Cache** | "键值缓存" | 复用已算好的中间结果，减少重复计算 |
| **Streaming** | "流式" | 内容边生成边输出 |

---

## 1. 问题描述扩展

### 1.1 会话膨胀的本质

在多 Agent 系统中，会话（Session）是 Agent 与其环境之间所有交互的时序记录。每条交互（用户消息、Agent 回复、工具调用、中间推理步骤）都在消耗 Token 预算。随着会话拉长，累计上下文会超出模型的上下文窗口上限。

```
会话膨胀速度示例（单轮交互，典型场景）：

用户消息       ~200 tokens
Agent 推理链   ~800 tokens
工具调用       ~300 tokens
工具返回       ~500 tokens
Agent 最终回复 ~400 tokens
─────────────────────────
单轮合计       ~2,200 tokens

以 128K context 模型（如 GPT-4 Turbo）为例：
  ~2,200 tokens/轮 × 50 轮 = ~110,000 tokens
  → 超过 85% 硬阈值（~108,800），触发深度压缩

以 300K context 模型（如本项目中使用的 DeepSeek-V4）为例：
  ~2,200 tokens/轮 × 100 轮 = ~220,000 tokens
  → 超过 70% 软阈值（~210,000），触发初步压缩
```

### 1.2 当前主流策略：渐进式压缩

行业内普遍采用 **阈值驱动的渐进式压缩**（Graduated Compression），通常分三级：

```
阶段 0：无压缩
  └── 会话长度 < soft_threshold（如 60% 窗口）
  └── 全部上下文可用

阶段 1：初步压缩（Light Compression）
  └── soft_threshold < 会话长度 < hard_threshold（如 85% 窗口）
  └── 策略：丢弃早期冗余消息、合并连续工具调用、省略已完成的子任务细节

阶段 2：深度压缩（Full Compression）
  └── 会话长度 >= hard_threshold
  └── 策略：LLM 生成会话摘要、仅保留关键决策节点、丢弃全部原始消息内容
```

### 1.3 压缩触发的核心风险

这是你关注的核心问题——**压缩在执行过程中触发**，导致：

#### 风险 1：信息丢失（Information Loss）

| 丢失类型 | 示例 | 后果 |
|----------|------|------|
| **隐性意图** | 用户说的"最好再看看 X 方案"——这条指令在压缩后被摘要为"讨论了 X" | Agent 后续不再主动提及 X |
| **边界条件** | "除了 case A 之外全都处理"——否定条件在摘要中被遗漏 | Agent 错误地处理了 case A |
| **中间推理** | "我选方案 B 是因为 A 有性能问题"——推理链被截断后只剩结论 | Agent 无法回溯决策依据 |
| **精确数据** | 表名 `order_detail_2026`, API 路径 `/api/v3/users/{id}/orders` | 压缩后泛化为"数据库"、"API 接口" |

#### 风险 2：任务状态丢失（Task State Corruption）

多 Agent 系统中，任务状态可能分布在多个 Agent 的共享上下文中：

```
压缩前（状态正常）：
  Coordinator: 任务 T-42 已分配给 Agent-A 和 Agent-B
  Agent-A: T-42 步骤 3/5 完成，等待 Agent-B 的中间结果
  Agent-B: T-42 步骤 2/5 处理中，预计输出格式为 JSON

压缩触发后：
  Coordinator 的摘要： "任务 T-42 由 Agent-A 和 Agent-B 处理中"
  → 丢失了 Agent-A "步骤 3/5" 的进度标记
  → 丢失了 Agent-B "预计输出格式为 JSON" 的契约
  → 丢失了 "等待中间结果" 的依赖关系
```

#### 风险 3：时序混乱（Temporal Disorientation）

压缩后的摘要丢失了事件的时间顺序，导致 Agent 错误理解因果链：

```
原始事件序列:
  1. 用户要求"先做 A，然后根据 A 的结果做 B"
  2. Agent 执行 A，得到结果 R
  3. Agent 用 R 执行 B

压缩摘要可能变成:
  "用户要求做 A 和 B，Agent 执行了 A 和 B"
  → 丢失了"根据 A 的结果做 B"的因果依赖
```

#### 风险 4：系统性退化（Systematic Degradation）

在长时间运行的 Agent 系统中，**反复压缩**导致逐轮信息衰减：

```
轮次  │ 原始 Token │ 压缩后 Token │ 信息保留率（估算）
──────┼────────────┼──────────────┼──────────────────
  1   │   2,200    │    2,200     │   100%
 10   │  22,000    │   15,000     │    68%   (第一次压缩, 估算)
 20   │  44,000    │   18,000     │    41%   (第二次压缩: 基于摘要再压缩, 估算)
 30   │  66,000    │   20,000     │    30%   (第三次压缩: 摘要的摘要, 估算)
```
> **注**: 上表中的信息保留率为定性估算，实际值取决于压缩算法、对话内容密度和摘要质量。尚无标准化的压缩质量度量方法。

**压缩的压缩（Re-compression）** 是系统性退化的根源——每次压缩都会丢失信息，而下次压缩是基于已丢失信息的摘要再压缩，形成信息衰减的恶性循环。此问题的一个已知缓解方案是 MemGPT 的分层摘要（Hierarchical Summarization），详见 2.3 节。

#### 风险 5：执行中压缩中断（Mid-Execution Compression Hazard）

用户特别关注"在 Agent 执行过程中触发压缩"的问题。这是一个独立于前 4 种风险的**独特失效模式**：

```
Agent 正在执行推理的中间状态:
  ┌─ System Prompt (固定) ─────────────────────┐
  ├─ Session Context (正在被 Agent 读取/写入)   │ ← 压缩在此刻触发!
  ├─ Agent Partial Reasoning (未完成的推理步骤) │ ← 被截断!
  ├─ Intermediate Variables (临时变量)           │ ← 丢失!
  └─ Pending Tool Results (即将返回的工具结果)  │ ← 可能丢失!
```

**该风险的特有特征：**
- **无检查点**：Agent 在执行中没有保存中间状态的能力（除非框架主动注入检查点机制）
- **部分推理丢失**：Agent 已经消耗了 Token 进行了部分推理，但压缩后这些 Token 的产出被浪费
- **恢复困难**：压缩完成后，Agent 从"新的上下文"开始执行，不知道自己之前思考到哪里
- **并发冲突**：压缩操作与 Agent 的流式输出可能并发执行，导致上下文不一致

**缓解方向**：见第 5.4 节——在安全的压缩时间点触发，避免在 Agent 生成推理链时执行压缩。

---

## 2. 行业解决方案调研

### 2.1 压缩策略分类

> **策略速览**（每种策略：核心思想 | 信息保留度 | 成本 | 延迟）：
> - **滑动窗口（Sliding Window）** = 只留最近 N 轮，最省事但忘得快
> - **LLM 摘要（Summarization）** = 让模型把旧内容浓缩成摘要（最常用）
> - **重要性评分（Importance Scoring）** = 给每条消息打分，只留高分
> - **结构化解构（Structural Decomposition）** = 把对话拆成原子事实存起来（保留最好但成本高）
> - **KV Cache 复用** = 复用已算好的中间结果，减少重复计算（省时省电）
> - **Token 级压缩** = 压缩重复的字符模式（类似 ZIP 压缩）

| 策略 | 核心思想 | 信息保留 | 计算成本 | 延迟影响 | 代表实现 |
|------|----------|----------|----------|----------|----------|
| **滑动窗口** | 保留最近 N 轮，丢弃早期 | ⭐⭐ | 极低 | 无 | Claude 上下文管理 |
| **LLM 摘要** | 让 LLM 压缩旧内容为摘要 | ⭐⭐⭐ | 高 | 显著 | MemGPT/Letta |
| **重要性评分** | 为每条消息打分，保留高分 | ⭐⭐⭐⭐ | 中 | 低 | 自定义实现 |
| **结构化解构** | 将消息拆解为原子事实保留 | ⭐⭐⭐⭐⭐ | 高 | 中 | 知识图谱方案 |
| **KV Cache 复用** | 窗口化 KV Cache 减少计算量 | ⭐⭐⭐⭐* | 低 | 无 | StreamingLLM |
| **Agent 自适应压缩** | Agent 主动决定压缩内容 | ⭐⭐⭐⭐ | 中 | 中 | Oh-My-OpenCode |
| **Token 级压缩** | 压缩重复 Token 模式 | ⭐⭐ | 低 | 低 | 自定义实现 |
| **混合策略** | 多种策略按场景组合 | ⭐⭐⭐⭐ | 中 | 中 | 主流生产方案 |

> *KV Cache 复用对窗口内内容无损，但窗口外历史完全丢弃。信息保留评分是从"整体会话"角度评估。*

### 2.1.5 Agent 自适应压缩（Agent-Aware Self-Compression）

**原理**：Agent 在系统指令中获知上下文窗口的限制，自主决定哪些信息需要保留、哪些可以丢弃，并在生成回复前主动标记可压缩区域。

```
系统指令注入:
  "你的上下文窗口剩余约 20% 空间。请评估当前的对话状态，
   标记哪些已完成的子任务可以安全压缩。压缩时保留：用户的关键要求、
   未完成的任务列表、正在等待的外部结果。可压缩：已完成的推理过程、
   已确认的中间结果、客套话。"

Agent 输出压缩标记:
  [COMPRESS_SAFE: turn_005-turn_010, completed_subtask_analysis]
  [PIN: user_requirement_security_first, active_task_db_migration]
```

**实际案例**：Oh-My-OpenCode 插件的 `compaction-context-injector` hook 会在系统指令中注入 `[SYSTEM DIRECTIVE: OH-MY-OPENCODE - COMPACTION CONTEXT]` 标记，提示 Agent 当前上下文已压缩或接近限制，让 Agent 感知到自身的上下文不完整。

**优点**：让最了解内容价值的 Agent 自己决定取舍；无需额外的评分模型
**缺点**：依赖 Agent 的元认知能力；不同模型的"压缩判断"质量差异大

### 2.1.6 Token 级压缩（Token-Level Compression）

**原理**：在 Token 层面应用无损或有损压缩算法（如 gzip、字典编码），减少上下文中的重复模式。

```
原始文本（含重复模式）:
  "调用函数 search(query='AI Agent')，返回 [...] 
   接着调用 search(query='Agent compression')，返回 [...]"

字典编码后:
  [dict: {"f1": "调用函数 search(query='", "f2": "')，返回"}]
  f1AI Agentf2 [...] f1Agent compressionf2 [...]
```

**优点**：实现简单；对高度重复的格式（如系统日志、JSON 工具输出）效果显著
**缺点**：对自然语言对话的压缩率有限；解码增加推理延迟

### 2.1.7 重要性评分的陷阱

重要性评分虽然在信息保留率上评分较高（⭐⭐⭐⭐），但在实际使用中存在一个**严重弱点**：如果一条具有决定性意义的消息因为评分因子匹配不足而被低分丢弃，其后果可能是灾难性的。例如，一条用户随口说的"别用 X 方案"可能评分很低（不含信号词、角色是 user 但看起来像闲聊），但它实际上是对 Agent 行为的决定性约束。因此，重要性评分不推荐作为唯一的压缩策略，更适合作为混合策略中的辅助因子。

### 2.2 滑动窗口（Sliding Window）

**原理**：保留最近的 K 轮完整对话，丢弃更早的历史。如果 K=20，则第 1-10 轮丢弃，第 11-30 轮保留。

```
轮次: [1][2][3]...[10][11][12]...[20][21][22]...[30]
       被丢弃              保留中
```

**优点**：实现简单、零计算开销、可预测的 Token 预算
**缺点**：丢失所有早期上下文，长期运行 Agent 不适用

**改进方案**：

- **分层滑动窗口**（Hierarchical Sliding Window）：保留近 N 轮完整 + 远 M 轮摘要——近期细节不丢，远期趋势不失
  ```
  窗口:
  ┌─ 远层: 轮次 1-50 的摘要（~2K tokens）─┐
  └─ 近层: 轮次 51-60 完整（~20K tokens）─┘
  ```
- **锚点保留（Anchored Sliding Window）**：标记关键消息为"锚点"（如用户的核心要求），窗口滑动时锚点不被丢弃

### 2.3 LLM 摘要压缩（Summarization）

**原理**：使用 LLM 将一段历史对话压缩为简洁摘要。这是目前生产系统中应用最广泛的方法（记住：**摘要 = 让模型读旧内容、写出精简版**）。

```
输入对话（~10K tokens）:
  User: 帮我写一个 Python 脚本，读取 CSV 文件并统计...
  Agent: 我来设计这个脚本...首先需要 pandas 库...
  [工具调用: python 代码执行]
  Agent: 运行结果如下...发现数据中有空值...
  User: 空值怎么处理？
  Agent: 有两种方案...方案 A 删除...方案 B 填充...
  ...

LLM 摘要指令:
  "请将以上对话压缩为 200 字摘要，保留：已完成的任务、未完成的任务、
   用户的关键偏好、当前进度状态。忽略客套话和技术细节。"

输出摘要:
  "用户要求编写 CSV 统计脚本。Agent 完成初版脚本编写并运行，
   发现数据中存在空值。用户询问空值处理方案，Agent 提出了
   删除和填充两种方案，正在等待用户选择。未完成：空值处理方案决策。"
```

**关键挑战**：
- **摘要颗粒度**：太粗丢失信息，太细无法压缩
- **摘要一致性**：多次摘要可能矛盾
- **执行时机**：在 Agent 执行过程中触发压缩会产生显著延迟（1-3 秒 LLM 调用）

#### 分层摘要（Hierarchical Summarization）

MemGPT/Letta 采用的方案——维护一棵摘要树：

```
Round 1-10: 摘要 A
Round 11-20: 摘要 B
Round 21-30: 摘要 C
Round 31-40: 摘要 D
                               ┌───────────┐
                               │ 摘要 A+B  │  ← 第二次聚合
                               └─────┬─────┘
                                     │
                    ┌────────────────┼────────────────┐
                    │                │                │
              ┌─────┴─────┐   ┌─────┴─────┐   ┌─────┴─────┐
              │  摘要 A   │   │  摘要 B   │   │  摘要 C   │  ← 第一次摘要
              └─────┬─────┘   └─────┬─────┘   └─────┬─────┘
                    │                │                │
              [1-10轮完整]    [11-20轮完整]   [21-30轮完整]
              (可丢弃)         (可丢弃)        (保留)
```

### 2.4 重要性评分（Importance Scoring）

**原理**：为每条消息/事件分配一个重要性分数，压缩时按分排序，保留前 N 个最高分项。

```
评分因子:
  ┌── 角色权重: 用户指令 > Agent 回复 > 工具输出 > 推理链
  ├── 时间权重: 近期 > 远期
  ├── 引用计数: 被后续消息引用的消息权重 +1
  └── 语义重要性: 包含"关键"、"重点"、"必须"等信号词的消息权重 +2

评分示例:
  [用户] "帮我做 X"                    → 评分 10 (角色: user, 关键指令)
  [Agent] "好的，开始分析..."           → 评分 5  (角色: assistant, 一般回复)
  [工具] 返回 JSON 结果                → 评分 7  (角色: tool, 数据重要)
  [Agent] "关键发现: Y 有问题"         → 评分 8  (含信号词"关键")
  [用户] "好的，继续"                  → 评分 3  (确认消息)
```

**优点**：比简单滑动窗口更智能，计算成本可控
**缺点**：评分规则的制定和调优复杂；不同场景下的最优评分因子不同

### 2.5 结构化解构（Structural Decomposition）

**原理**：将自然语言对话映射为结构化的事实/状态，压缩时只保留结构化数据。

```
原始对话:
  User: "帮我查一下用户 ID=123 的订单，昨天下的"
  Agent: 查询数据库...
  工具返回: [{"order_id": "ORD-456", "status": "shipped", "total": 299.00}]
  Agent: "用户 123 的订单 ORD-456 已于昨天发货，金额 299 元"
  User: "把这个订单的物流信息也查一下"

解构为原子事实：
  ├─ entity: {type: "user", id: 123}
  ├─ entity: {type: "order", id: "ORD-456", status: "shipped", total: 299.00, user_id: 123}
  └─ intent: {user: 123, action: "query_logistics", target: "ORD-456"}

压缩后恢复（省略推理过程）:
  {entities} + {pending_intents} + {completed_actions}
  → 重建为: "用户 123 的订单 ORD-456（已发货，299元），正在查物流信息"
```

**优点**：极高的信息保留率和一致性
**缺点**：需要额外的结构化解析层，增加了延迟和复杂度

### 2.6 StreamingLLM / KV Cache 复用

**原理**：Transformer（一种神经网络架构）的自注意力机制会为新生成的每个 Token 计算与**所有历史 Token** 的注意力（注意力 = 模型判断"当前这个词和历史上哪些词最相关"的机制）。其复杂度为 O(L²)（L² = 数据量平方，指数级增长），其中 L 为总 Token 数。StreamingLLM 等技术（Xiao et al., 2023）通过将 KV Cache 限制在一个**滑动窗口 + 注意力锚点**的范围内，实现近似线性的计算复杂度。

```
传统的 Full Attention（全量注意力）：
  每次生成新 Token 时，需要与全部 L 个历史 Token 计算注意力
  → O(L²) 复杂度，L 为全量历史 Token 数

StreamingLLM 方式：
  Cache = [Sink Tokens (初始 4 个)] + [Sliding Window (最近 W 个 Token)]
  注意力只在 Cache 范围内计算
  → O(W²) 复杂度，W 为窗口大小（W << L）
  → 注意：复杂度并非 O(n)，而是 O(W²)，但由于 W 固定，可视为 O(1)
```

**关键澄清**：StreamingLLM 并不是"保留最初 4 个 Token 然后滑动"，而是保留**两组 Token**：
- **Attention Sinks**（注意力汇点）：序列开头的 4 个初始 Token，用于吸收多余的注意力分数（一种"注意力缓冲垫"技巧）
- **Sliding Window**（滑动窗口）：最近的 W 个 Token，携带最近的上下文

**优点**：计算效率极高，在推理时**约等于无损**（指对窗口内内容的注意力分布几乎没有改变；但窗口外的历史被完全丢弃，这本身就是信息损失）
**缺点**：只解决了计算效率，没有解决上下文窗口物理上限问题；窗口外的历史被完全丢弃

---

## 3. 会话生命周期与持久化

### 3.1 会话的三态模型

```
            ┌──────────────┐
            │  活跃会话     │  ← 当前正在进行的会话
            │  Active       │     存储在内存 + 频繁落盘
            └──────┬───────┘
                   │ 用户关闭 / 超时
                   ▼
            ┌──────────────┐
            │  归档会话     │  ← 已完成/暂停的会话
            │  Archived     │     存储在磁盘/DB
            └──────┬───────┘
                   │ 用户选择恢复
                   ▼
            ┌──────────────┐
            │  恢复会话     │  ← 重建上下文的会话
            │  Restored     │     从存档读取 + 压缩重建
            └──────────────┘
```

> **三态模型解读**：会话像文件一样有"正在编辑（Active）/ 存盘归档（Archived）/ 重新打开（Restored）"三种状态。用户关闭或超时 → 归档；用户再次打开 → 恢复。
```

### 3.2 会话存储结构（参考格式）

一个典型的会话存储包含以下层次：

```
session/
├── metadata.json              # 会话元数据
│   ├── session_id: "ses_a1b2c3"
│   ├── created_at: "2026-08-13T10:00:00Z"
│   ├── last_modified: "2026-08-13T11:30:00Z"
│   ├── compression_state: "light"  # none | light | full
│   ├── total_tokens: 45230
│   ├── total_turns: 34
│   └── tags: ["project-x", "debugging"]
│
├── turns/                     # 原始交互记录（可变，可能被压缩）
│   ├── turn_001.json          #   {role, content, tool_calls, tokens, timestamp}
│   ├── turn_002.json
│   └── ...
│
├── summaries/                 # 摘要层（压缩后生成）
│   ├── summary_20260813_1.json   # {turns: [1-10], summary_text, tokens_saved}
│   └── summary_20260813_2.json   # {turns: [11-20], summary_text, tokens_saved}
│
├── state/                     # 任务状态（不受压缩影响）
│   ├── tasks_active.json      #   当前进行中的任务队列
│   ├── completed_tasks.json   #   已完成任务列表
│   └── entity_state.json      #   实体状态快照
│
└── artifacts/                 # 产物附件
    ├── generated_code.py
    └── analysis_report.md
```

**关键设计原则：状态与对话分离**

- `state/` 目录下的任务状态**不受压缩影响**
- `turns/` 目录可以被压缩甚至删除
- `summaries/` 是压缩产物，用于重建上下文
- `artifacts/` 是独立产物，永久保留

### 3.3 会话恢复机制（Rehydration）

当用户重新打开一个历史会话时，系统需要将存档的 session 恢复到 Agent 的上下文窗口中。这个过程的专业术语叫 **会话重建（Session Rehydration）**。

```
用户打开历史会话
       │
       ▼
Step 1: 读取 metadata.json → 获取压缩状态
       │
       ▼
Step 2a: 如果状态是 none（未压缩）
         → 加载最近的 N 轮完整对话到上下文
         → 跳过 Step 3
       │
Step 2b: 如果状态是 light（初步压缩）
         → 加载最近 N 轮完整 + 早期锚点
         → 加载所有 summaries
       │
Step 2c: 如果状态是 full（深度压缩）
         → 加载最新的 summary 作为"此时的世界状态"
         → 加载 state/ 中的活跃任务
       │
       ▼
Step 3: 重建 Agent 的上下文
        ┌────────────────────────────────────────┐
        │   System Prompt（固定）                 │
        │   Tool Definitions（固定）               │
        │   ─────────────────────────────────     │
        │   Summary Layer（压缩历史）              │
        │   "会话摘要：用户要求做 X 和 Y..."       │
        │   ─────────────────────────────────     │
        │   Active State（任务状态）               │
        │   "待完成：Y 步骤 3-5"                  │
        │   ─────────────────────────────────     │
        │   Recent Turns（最近完整对话）           │
        │   [用户] "Y 的步骤 3 做完了吗？"        │
        │   [Agent] "快好了，还差..."             │
        └────────────────────────────────────────┘
```

**三种恢复模式对比：**

| 模式 | 使用场景 | 上下文构成 | Token 消耗 | 信息完整度 |
|------|----------|------------|------------|------------|
| **快速恢复** | 用户快速浏览历史 | 仅加载最近 N 轮 | 低 | 低 |
| **标准恢复** | 用户继续之前的工作 | 最近 N 轮 + 状态 + 摘要 | 中 | 高 |
| **完整恢复** | 用户需要精确的历史回顾 | 全部存档消息 | 高 | 极高 |

### 3.4 ucode (Oh-My-OpenCode) 的实际 session 管理分析

> ⚠️ **免责声明**: 以下分析基于本地安装的 ucode 运行时数据和 Oh-My-OpenCode 插件（v2026.5.7）的 TypeScript 类型声明文件（`.d.ts`）。分析推断基于公开可见的接口和运行时文件结构，但**并非 ucode/Oh-My-OpenCode 的内部实现文档**。推断结果可能不完全准确。

#### 存储后端：SQLite 为主，辅以 JSON 文件

通过检查 ucode 的实际运行数据目录，可以确认以下存储结构：

```
~/.local/share/opencode/
├── opencode.db                 # 主存储 — SQLite 数据库，约 170MB
├── opencode.db-shm             # SQLite 共享内存文件
├── opencode.db-wal             # SQLite WAL 日志，约 130MB
├── storage/
│   ├── session_diff/           # session 差异标记文件（每个 2 字节 JSON）
│   │   ├── ses_xxx1.json
│   │   ├── ses_xxx2.json
│   │   └── ... (数百个 session)
│   ├── agent-usage-reminder/   # Agent 使用记录（~125 字节 JSON 每 session）
│   ├── directory-agents/       # 目录 Agent 配置缓存
│   ├── directory-readme/       # 目录 README 缓存
│   └── project/global.json     # 项目全局配置
└── log/                        # 运行日志

~/.local/state/opencode/
├── kv.json                     # 键值存储（当前仅存 theme 设置）
└── prompt-history.jsonl        # 用户输入历史（JSONL 格式，不包含 Agent 回复）

~/.config/opencode/opencode.json  # 主配置（模型、provider、插件）
```

**关键发现：**

1. **主存储为 SQLite**：`opencode.db`（171MB）存储 session 消息、part 数据和元数据。有 `isSqliteBackend()` 检测函数，确认 SQLite 是正式后端。

2. **Session diff 文件**：`storage/session_diff/ses_xxx.json` 每个文件仅 2 字节——它们是**标记文件**（而非实际数据），用于快速索引/过滤 session 列表。

3. **Prompt history 是独立的**：`prompt-history.jsonl` 仅记录用户输入历史（约 50 条，6.5KB），不包含 Agent 回复。用于输入建议/补全功能，而非 session 恢复。

4. **Session ID 格式**：`ses_` 前缀 + 24 字符（base64 编码），如 `ses_006a64196ffenHgHEIIwFCt14p`。

5. **Session Metadata 结构**（来自类型定义 `SessionMetadata`）：
   ```typescript
   interface SessionMetadata {
     id: string;
     version?: string;
     projectID: string;      // 关联到项目
     directory: string;      // 工作目录
     title?: string;
     parentID?: string;      // 父子 session 关系
     time: { created: number; updated: number; };
     summary?: { additions: number; deletions: number; files: number; };
   }
   ```

6. **Session 消息结构**（来自类型定义 `SessionMessage`）：
   ```typescript
   interface SessionMessage {
     id: string;
     role: "user" | "assistant";
     agent?: string;         // 哪个 agent
     time?: { created: number; updated?: number; };
     parts: MessagePart[];   // 消息体由多个 part 组成
   }
   interface MessagePart {
     id: string; type: string;
     text?: string; thinking?: string;
     tool?: string; callID?: string;
     input?: Record<string, unknown>;
     output?: string; error?: string;
   }
   ```
   > 每条消息由一个或多个 `MessagePart` 组成（text 消息体、thinking 推理过程、tool 工具调用），这与 3.2 节提出的"按 turn 和 part 组织"的通用结构相符。

#### 压缩机制：Compaction Context Injector

Oh-My-OpenCode 包含两个独立但互补的模块来处理会话上下文：

**压缩上下文注入模块**（`compaction-context-injector` hook）——负责压缩触发和 Agent 元认知对齐：
```
1. 监控当前上下文窗口使用比例（tail-monitor）
2. 当上下文接近限制时：
   a. 在系统指令中注入 [SYSTEM DIRECTIVE: OH-MY-OPENCODE - COMPACTION CONTEXT]
   b. 提示 Agent 当前上下文已被压缩或接近限制
   c. 让 Agent 感知到自身上下文的"不完整性"
3. 通过 recoverCheckpointedAgentConfig 恢复 Agent 配置检查点
```

**会话恢复模块**（`session-recovery` hook）——这是一个**独立**的 hook，负责从错误/故障中恢复上下文（而非直接负责压缩）：
```
1. findLastUserMessage() — 从消息列表中定位最后一条用户消息作为恢复锚点
2. extractResumeConfig() — 从该消息提取恢复配置（sessionID、agent、model 等）
3. resumeSession() — 执行恢复：重新读取 SQLite 中的历史消息并重建上下文
```

> **注意**：两个 hook 分工不同——`compaction-context-injector` 处理压缩时的 Agent 状态恢复，`session-recovery` 处理错误恢复时的上下文重建。文档中的 `findLastUserMessage`/`extractResumeConfig`/`resumeSession` 属于后者。

**这与 5.2 节"压缩标记"和 2.1.5 节"Agent 自适应压缩"的策略高度吻合**——系统不直接压缩消息，而是通过注入 directive 让 Agent 感知并适应压缩后的上下文。

#### Session 恢复机制

从 `session-recovery` hook 的类型定义可以推断恢复流程：

```
1. findLastUserMessage() — 找到 session 中最后一条用户消息
2. extractResumeConfig() — 提取恢复配置
3. resumeSession() — 执行恢复：
   a. 从 SQLite 读取消息到缓冲
   b. 注入恢复提示到上下文
   c. 恢复 agent 的状态（如果是多 agent 场景）
```

**关键恢复策略**：Oh-My-OpenCode 的恢复不是简单的"加载全部历史消息到上下文"，而是：
- 找到最后一条用户消息作为"恢复锚点"
- 构造一个 `ResumeConfig` 决定加载多少上下文
- 可能使用**懒加载**模式——只加载上下文窗口限制内的消息

#### 关于压缩时的存储行为

从运行时数据推断，ucode/Oh-My-OpenCode 的压缩行为是：

1. **原始数据不删除**：SQLite 数据库保留所有历史消息——压缩只是运行时上下文的截断，不是数据删除
2. **session_diff 文件**（2 字节 JSON）可能用于标记"此 session 是否需要增量重新加载"
3. **摘要层存在但不持久化到文件系统**：`preemptive-compaction.d.ts` 暴露了 `client.session.summarize(...)` 方法——说明系统具备 LLM 摘要压缩的能力。但与 3.2 节的 `summaries/` 目录不同，**文件系统上没有独立的摘要存储文件**。压缩后的"摘要"信息是通过 `[SYSTEM DIRECTIVE: COMPACTION CONTEXT]` 在运行时注入到上下文中的。

> ⚠️ **重要限制**：我们仅能确认**文件系统层面**没有独立的摘要文件。SQLite 数据库（`opencode.db`，171MB）内部是否缓存了摘要数据**无法确认**——在不检查 SQLite 表结构的情况下，不能排除摘要数据被持久化在数据库内部的可能性。因此，更精确的表述是："摘要内容在文件系统上是 transient 的，但可能在 SQLite 内部有缓存"

#### 与通用模型的关键差异

| 维度 | 3.2 节的通用模型 | ucode/Oh-My-OpenCode 的实际实现 |
|------|-----------------|-------------------------------|
| **存储后端** | 文件系统 + JSON | **SQLite 数据库** |
| **消息结构** | turn 为最小单位 | **part 为最小单位**（text + thinking + tool 组合） |
| **摘要持久化** | 独立的 `summaries/` 目录 | **无独立摘要存储**——通过 directive 在运行时注入 |
| **session 标记** | 无 | **session_diff 标记文件**用于快速索引 |
| **压缩触发时机** | 用户定义阈值 | **context window monitor** + tail monitor |
| **恢复策略** | 按压缩状态分级恢复 | **基于最后用户消息的锚点恢复** |

---

## 4. 关键问题解答

### Q1: 压缩前后 session 怎么保存？

> ⚠️ **以下示例为通用设计模式（文件系统 + JSON），非 ucode 的实际实现。** ucode/Oh-My-OpenCode 使用 SQLite 数据库存储 session，见 3.4 节的对比表。本例旨在展示"状态与对话分离"的设计思想，实际数据库实现的结构类似但存储后端不同。

**压缩前（通用设计示例）**：

```json
// session/turns/turn_001.json (完整原始消息)
{
  "turn_id": "turn_001",
  "role": "user",
  "content": "帮我分析一下这个项目的依赖关系",
  "tokens": 18,
  "compressed": false,
  "timestamp": "2026-08-13T10:00:00Z"
}

// session/turns/turn_002.json
{
  "turn_id": "turn_002",
  "role": "assistant",
  "content": "让我先分析 package.json 和 import 语句...",
  "tool_calls": [
    {"name": "read_file", "args": {"path": "package.json"}},
    {"name": "grep", "args": {"pattern": "import", "path": "src/"}}
  ],
  "tokens": 356,
  "compressed": false,
  "timestamp": "2026-08-13T10:00:05Z"
}
```

**压缩后（turn 文件保持不变，新增 summaries/ 和更新 state/）**：

```json
// session/summaries/summary_1.json (压缩产物)
{
  "summary_id": "sum_001",
  "covers_turns": ["turn_001", "turn_002", "turn_003", "turn_004", "turn_005"],
  "original_tokens": 5840,
  "compressed_tokens": 350,
  "compression_ratio": "94%",
  "summary_text": "用户要求分析项目依赖关系。Agent 读取了 package.json 
                  和 src/ 目录下的 import 语句，初步识别出 lodash、express、
                  axios 三个核心外部依赖。正在进行更深入的分析。",
  "compression_method": "llm_summarization",
  "created_at": "2026-08-13T10:15:00Z"
}

// session/state/tasks_active.json (不受压缩影响)
{
  "active_tasks": [
    {
      "task_id": "task-1",
      "description": "分析项目依赖关系",
      "status": "in_progress",
      "progress": "steps 1-2/5 completed",
      "next_step": "生成依赖关系图",
      "dependencies": []
    }
  ]
}
```

**设计选择说明**：上述方案采用"持久化层与运行时层分离"的模式——原始 turn 文件在持久化层保持不变，压缩仅在运行时内存中构建新的上下文。这是方案之一，优点是压缩可逆、策略可替换。另一些系统（尤其是存储受限场景）可能会直接在持久化层截断或删除原始消息。两种选择各有优劣：前者保留数据完整性但消耗更多存储，后者节省空间但丢失历史回滚能力。

### Q2: 用户重新打开历史会话怎么恢复？

参见上述 3.3 节——这是一个三阶段过程：

1. **读取元数据** → 确定压缩状态
2. **按状态选择重建策略** → none: 加载完整 / light: 摘要+最近 / full: 摘要+状态
3. **重建上下文窗口** → System Prompt + Summary Layer + Active State + Recent Turns

### Q3: 可以在自己的项目目录中保存 session 吗？

**完全可以**。以下是 AiThiker 项目可用的方案：

#### 方案 A：项目级 session 目录（推荐）

```
AiThiker/
├── .sessions/                   # 按项目隔离的 session 存储
│   ├── ses_001/                 # 每个 session 一个子目录
│   │   ├── metadata.json
│   │   ├── turns/
│   │   ├── summaries/
│   │   └── state/
│   ├── ses_002/
│   └── .sessions_index.json     # session 索引（可选）
├── topics/
├── experiments/
└── notes/
```

**优点**：与项目绑定，git 可管理，转移方便
**缺点**：session 可能包含敏感信息，需要 `.gitignore` 过滤

#### 方案 B：用户级全局 session 目录

```
~/.aithiker/sessions/
├── ses_001/
├── ses_002/
└── index.json                   # session 索引（含项目路径映射）
```

**优点**：session 统一管理，不污染项目目录
**缺点**：需要额外的索引来映射 session <-> 项目

#### `.gitignore` 建议

```gitignore
# 排除 session 数据（包含对话内容，可能冗余）
.sessions/
# 或更精细的控制：
.sessions/*/turns/    # 排除原始对话
.sessions/*/summaries/ # 排除摘要
.sessions/*/metadata.json  # 保留元数据（可选）
```

---

## 5. 防止压缩导致信息丢失的工程实践

### 5.1 状态-对话分离架构

这是最重要的原则——**解耦任务状态和对话记录**：

```
┌────────────────────────────────────────────┐
│                  Session                    │
│                                            │
│  ┌─────────────┐    ┌──────────────────┐   │
│  │ 对话层       │    │ 状态层            │   │
│  │ (ChatLog)   │    │ (StateStore)     │   │
│  ├─────────────┤    ├──────────────────┤   │
│  │ 用户消息     │    │ 活跃任务         │   │
│  │ Agent 回复   │    │ 实体信息         │   │
│  │ 工具调用     │    │ 决策记录         │   │
│  │ 推理过程     │    │ 全局变量         │   │
│  └──────┬──────┘    └──────┬───────────┘   │
│         │                   │               │
│   可压缩/可丢弃         永不压缩          │
│   压缩后不影响状态      状态变更驱动压缩   │
└────────────────────────────────────────────┘
```

**实现要点**：

- 状态层使用**结构化格式**（JSON/YAML），而非自然语言
- 状态层不参与 Token 预算的压缩决策
- Agent 执行阶段的关键状态变更先写入状态层，再进入对话层

### 5.2 压缩标记与版本控制

每条消息/摘要携带压缩标记，使得 Agent 能够在后续推理中感知压缩：

```json
{
  "turn_id": "turn_010",
  "content": "用户要求确认部署方案",
  "compression_watermark": {
    "compressed": true,
    "original_turns": ["turn_001", "turn_002", "turn_003"],
    "compression_method": "llm_summarization",
    "loss_warning": "summary may have omitted: specific version numbers, 
                     rejected alternatives, user's tone/preferences",
    "agent_visible": true
  }
}
```

**Agent 可见的压缩标记**：在上下文中告知 Agent "你看到的这段内容是压缩摘要，原始内容已丢弃，注意可能的遗漏"——这是一种**元认知对齐**，让 Agent 知道自己的上下文不完整，从而在推理中更加保守。

### 5.3 原子事实保留（Critical Fact Pinning）

在系统 prompt 中定义一个"关键事实池"，Agent 在对话过程中主动将关键信息"钉入"池中：

```json
{
  "pinned_facts": [
    {"id": "pf_001", "fact": "用户要求优先处理安全性", "source_turn": "turn_005", "priority": "high"},
    {"id": "pf_002", "fact": "排除方案 C（成本太高）", "source_turn": "turn_012", "priority": "high"},
    {"id": "pf_003", "fact": "数据库连接串: postgres://user:pass@host/db", "source_turn": "turn_008", "priority": "critical"}
  ]
}
```

**关键事实池**在 Agent 的系统 prompt 中单独占用一块固定的 Token 预算，**永远不受压缩影响**。

### 5.4 压缩触发时间点选择

不要在 Agent 执行过程中压缩——这是信息丢失的要因。推荐的触发点：

```
✅ 用户的自然停顿后（用户输入结束，Agent 尚未开始回复）
✅ 工具调用完成、Agent 生成回复前
✅ 主动轮结束（Agent 回复完成后，下一轮用户消息到来前）
✅ 达到预警阈值时立即触发（而非达到硬上限才触发）

❌ Agent 正在执行推理时
❌ 工具正在返回大量数据时
❌ 两轮交互之间极短的时间窗口（可能导致压缩与执行并发冲突）
```

### 5.5 压缩后的正确性校验

每次压缩后增加一个**压缩验证步骤**：

```
1. 压缩前：保存当前状态快照 S_before
2. 执行压缩，得到压缩后的上下文 C_compressed
3. 用 C_compressed 回答关键验证问题：
   - "当前完成的任务有哪些？"
   - "用户的最高优先级要求是什么？"
   - "还有哪些待办项？"
4. 将答案与 S_before 对比
5. 如果偏差超过阈值 → 回退压缩，调整策略
```

---

## 6. 未来方向与开放问题

1. **自适应压缩**：根据对话的语义密集度动态调整压缩率——"低信息密度"段落高压缩，"高信息密度"段落低压缩/不压缩

2. **Agent 感知压缩**：Agent 主动参与自己的上下文管理——Agent 标记重要信息"请不要压缩"，或主动请求"我快超出上下文了，请触发压缩"

3. **多 Agent 上下文隔离**：在多 Agent 系统中，为每个 Agent 维护独立的上下文窗口，只共享"公开状态层"——大幅降低单个 Agent 的上下文压力

4. **Agent 感知的元认知压缩**：如 Oh-My-OpenCode 的做法——不直接压缩上下文，而是通过 `[SYSTEM DIRECTIVE: COMPACTION CONTEXT]` 告知 Agent 上下文已被压缩，让 Agent 自身做出适应性调整。这可以看作是"让 Agent 知道自己不知道"的元认知对齐策略

5. **压缩评估指标**：需要标准化的压缩质量评估指标：
   - **重建保真度（Reconstruction Fidelity）**：从压缩摘要能否重建原始意图
   - **任务维持率（Task Persistence Rate）**：压缩后任务状态的正确率
   - **幻觉引入率（Hallucination Introduction Rate）**：压缩摘要引入虚假信息的比例

---

## 关联文档

- `../memory-and-state/AGENTS.md` — 记忆系统目录约定
- `../agent-architecture/index.md` — Agent 架构中的状态管理
- `../multi-agent-systems/index.md` — 多 Agent 协作中的上下文共享
- `../patterns/` — 会话压缩设计模式（待创建）
- `../../experiments/` — 压缩策略实验记录（待创建）

---

## 📖 名词标注表（完整中英对照）

> 本文（会话压缩专题）全部关键英文术语，看懂这张表 = 读懂本文。

| 英文 | 中文理解 | 详细解释 |
|------|---------|---------|
| **Session** | 会话 | 一次连续的对话过程，包含所有交互记录 |
| **Token** | 词元/令牌 | 模型处理文本的最小单位；压缩就是省 token |
| **Context Window** | 上下文窗口 | 模型一次最多容纳的 token 总量（本文以 128K/300K 为例） |
| **Context** | 上下文 | 喂给模型的所有内容（消息历史+系统提示+资料） |
| **LLM** (Large Language Model) | 大语言模型 | ChatGPT 这类文本生成模型 |
| **Agent** | 智能体 | 能自主思考、调用工具、完成任务 AI 程序 |
| **Multi-Agent** | 多智能体 | 多个 Agent 协作的系统（本文讨论的场景） |
| **压缩 (Compression)** | 浓缩 | 把旧对话变成摘要，腾出窗口空间 |
| **剪枝 (Pruning)** | 裁剪 | 直接丢弃不重要的旧消息 |
| **摘要 (Summarization)** | 总结 | 用模型把旧内容浓缩成一段话 |
| **滑动窗口 (Sliding Window)** | 滚动窗口 | 只保留最近 N 轮的策略 |
| **重要性评分 (Importance Scoring)** | 打分保留 | 给每条消息打分，只留高分 |
| **结构化解构 (Structural Decomposition)** | 拆解事实 | 把对话拆成"实体/意图"等原子事实存起来 |
| **KV Cache** | 键值缓存 | 缓存已算好的中间结果，避免重复计算 |
| **StreamingLLM** | 流式大模型技术（专有名词） | 论文技术：窗口化注意力，支持无限长流式对话 |
| **Attention (自注意力)** | 注意力机制 | 模型判断"当前词与历史上哪些词相关"的机制 |
| **Attention Sinks** | 注意力汇点 | 序列开头的 4 个 token，吸收多余注意力分数 |
| **复杂度 O(L²) / O(W²)** | 计算量估算 | 数据量 L 时计算量按 L 的平方增长；W<<L 表示窗口小很多 |
| **阈值 (Threshold)** | 门槛 | 触发条件（如 60% 软阈值 / 85% 硬阈值） |
| **软阈值 / 硬阈值** | 预警线 / 极限线 | 软=先轻度压缩；硬=必须深度压缩 |
| **渐进式压缩 (Graduated)** | 分级压缩 | 分阶段：先修剪、再摘要、最后全量压缩 |
| **分层摘要 (Hierarchical Summarization)** | 摘要的摘要 | MemGPT 用摘要树：二级摘要盖一级摘要，减少二次压缩损失 |
| **信息衰减 (Degradation)** | 越压越丢 | 反复压缩导致信息逐轮丢失（摘要的摘要更模糊） |
| **KV Cache 复用** | 缓存复用 | 用缓存窗口避免重复计算（StreamingLLM） |
| **Agent 自适应压缩** | 自主压缩 | Agent 自己决定哪些内容可压缩（元认知） |
| **元认知 (Meta-cognition)** | 知道自己想到什么 | Agent 感知"自己上下文不完整"的能力 |
| **Compaction** | 压缩/整并 | ucode 中"压缩上下文"的专业说法 |
| **Hook** | 钩子/接口 | 在特定时机插入自定义逻辑的扩展点 |
| **[SYSTEM DIRECTIVE]** | 系统指令标记 | 注入系统提示词的特殊控制指令 |
| **Watermark** | 水印/标记 | 标记"这是被压缩过的摘要"，提示 Agent |
| **PIN / Pinned Facts** | 钉住的事实 | 永远不被压缩的关键事实（像钉在墙上） |
| **锚点 (Anchor)** | 固定点 | 窗口滑动时也不被丢弃的关键消息 |
| **持久化 (Persistence)** | 存盘 | 把状态保存到磁盘/数据库，进程挂了不丢 |
| **归档 (Archive) / 恢复 (Restore)** | 存档 / 重开 | 会话三态：活跃→归档→恢复 |
| **Rehydration** | 会话重建（补水） | 从存档重建上下文的动作 |
| **Metadata** | 元数据 | 描述数据的数据（如会话标题、创建时间） |
| **Artifact** | 产物/附件 | 会话中生成的结果文件（代码、报告） |
| **Checkpoint** | 检查点 | 状态快照，用于恢复现场 |
| **SQLite** | 轻量数据库（专有名词） | 单文件的嵌入式数据库（ucode 用它存会话） |
| **WAL 日志** | 预写日志 | 数据库先记日志再改数据，崩溃可恢复 |
| **Base64 编码** | 文本编码 | 二进制数据转成 ASCII 文本的方式 |
| **KV 存储** | 键值存储 | 简单的 key→value 数据库 |
| **Token 级压缩** | 字符压缩 | 压缩重复字符模式（类似 ZIP） |
| **重建保真度 (Reconstruction Fidelity)** | 恢复准确度 | 从摘要能否还原原始意图的评估指标 |
| **任务维持率 (Task Persistence)** | 任务不丢率 | 压缩后任务状态保持正确的比例 |
| **幻觉引入率 (Hallucination Rate)** | 胡编率 | 压缩摘要中虚假信息的比例 |
| **原子事实 (Atomic Fact)** | 最小事实 | 不可再拆的最小信息单元（如"订单ORD-456已发货"） |
| **Entity / Intent** | 实体 / 意图 | 结构化解构的两类原子事实：事物 / 想做的事 |
| **多轮推理 (Multi-turn)** | 多轮思考 | 分多步推理，每步基于前一步结论 |
| **推理链 (Reasoning Chain)** | 思考链路 | 从问题到结论的完整推理过程 |
| **时序 (Temporal)** | 时间顺序 | 事件发生的时间先后（压缩可能打乱因果） |
| **因果依赖 (Causal Dependency)** | 因果关联 | "先 A 后 B"的先后依赖关系（如先查再报） |

---

> **思考：** 会话压缩本质上是"在有限信息通道中做取舍"的问题。没有完美的压缩策略，只有适合场景的权衡。关键不在于"怎么压缩"，而在于"压缩时知道什么是重要的"。分离状态与对话、让状态结构化和可回溯，是比任何压缩算法都更根本的解决思路。
