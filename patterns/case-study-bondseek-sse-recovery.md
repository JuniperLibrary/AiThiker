---
title: BondSeek SSE 流式断线恢复系统（事件序号驱动的回放机制）
tags: [case-study, sse, streaming, disconnect-recovery, replay-log, seq, redis-stream, golang]
created: 2026-10-21
updated: 2026-10-21
status: draft
---

# BondSeek SSE 流式断线恢复系统（事件序号驱动的回放机制）

> **案例对象:** [bondseek-ai](https://github.com/) — 债券行业多租户 AI SaaS 投研平台（Go 1.24 微服务）
> **分析范围:** SSE（Server-Sent Events，服务器推送事件流）断线恢复的完整实现：ReplayLogStore（回放日志）/ OpenBlockStore（未封口块快照）/ CoreHydrator（水位线驱动的水合器）
> **资料代码:** `bondseek-common/agentcore/`（replay_log_store.go 167 行 + open_block_store.go 160 行 + wss_hydrator.go 226 行）（2026-10-21 代码级深挖）
> **关联文档:** [多 Agent 编排](./case-study-bondseek-multi-agent.md)、[RAG 检索系统](./case-study-bondseek-rag.md)、[整体案例分析](./case-study-bondseek-memory.md)
> **面试话题索引:** [agent-interview-questions.md](../notes/agent-interview-questions.md) — 第 10 题"SSE 流式对话断线如何恢复"（本文即答案素材）

## 📌 名词速查表（先看这里）

> 本文讲流式断线恢复，核心概念是"事件、序号、水位线"。先看这里（完整对照见文末"名词标注表"）：

| 英文 | 读作/理解成 | 一句话解释 |
|------|------------|-----------|
| **SSE** | "服务器推送事件流" | 服务器把消息一段段推给浏览器的实时通道（类似打字机效果） |
| **Streaming** | "流式" | 结果边生成边发送，不用等全部生成完 |
| **Reconnect** | "重连/断线恢复" | 网络断了之后，把断掉期间漏掉的消息补回来 |
| **Replay** | "回放" | 重连后把历史事件重新按顺序发一遍 |
| **seq** | "序号" | 每条事件的全局唯一递增编号（断线恢复的地基） |
| **Cursor** | "游标/读到哪了" | 客户端已收到的最大序号（下次从这里继续） |
| **Watermark** | "水位线" | 服务端已可靠下发的进度标记（本质也是 seq） |
| **NATS** | "消息队列（专有名词）" | 服务间实时事件广播的通信组件 |
| **Redis Stream** | "Redis 流（专有名词）" | Redis 里的一种有序消息列表数据结构 |
| **Hydrate** | "水合/灌水" | 重新上线时先把缓存补齐、再接上实时流（比喻：先浇水再放鱼） |
| **Live** | "实时通道" | 正在发生的实时事件流（相对历史的补发/快照） |

---

## 🎯 TL;DR 冲突摘要（先看这段）

> **这篇文档解决的核心矛盾：实时的（Live 推送）和可靠的（不丢不重不错序）天生打架。** 广播通道（NATS）是"瞬态"的——发出去就没了，断线的人就漏了；纯粹存历史又做不到实时。BondSeek 的破局思路**极其朴素又极其有效：给每条事件一个全局递增的序号（seq），所有难题（丢/重/乱序）瞬间退化成"比较两个整数"**。读完全文你只需要带走一个画面：**"客户端断线恢复 = 用『我收到第几号了』这个整数，从日志里把后面的补回来。"**

---

## 1. 问题域：流式对话为什么会"漏"

### 1.1 流式对话的事件流长什么样

Agent 流式输出时，前端会收到一串事件：

```
tool_start(调用指标计算) → text_delta("根据计算，该券的") → text_delta("久期是3.2年") → text_complete → done
```

每个事件都是"一段进度"。用户正在盯着的就是一个**不断刷新的进度流**。

### 1.2 断线时的三种故障（这是理解全文的钥匙）

| 故障 | 通俗解释 | 用户看到 |
|------|---------|---------|
| **丢事件 (loss)** | 断线那几秒的事件发出了，但你不在线没收到 | 中间一段话没了 |
| **重事件 (duplicate)** | 重连后从头推，你之前看到的又看一遍 | 重复说话 |
| **乱序 (out-of-order)** | 慢的事件后到、快的事件先到 | 逻辑颠倒（先有结论后有数据） |

> **记住这三兄弟：丢、重、乱。** 后面所有机制都是为剿灭它们设计的。

### 1.3 两个通道的特性（为什么必须双通道配合）

| 通道 | 特性 |
|------|------|
| **NATS Core（实时广播）** | 快、实时，但**瞬态**——广播后即焚，断线的人收不到（开发注释原文："NATS Core live 通道是瞬态广播（不持久、不保序）"） |
| **Redis Stream（持久日志）** | **持久、保序**，但**慢**（要落盘）——不能当实时通道用 |

**结论：实时用 NATS，可靠用 Redis，两者靠 seq 对齐。**

---

## 2. ReplayLogStore：会话级"带序号"的有序持久事件流（replay_log_store.go）

> **一句话：** 每个会话维护一条 Redis 事件流，每条事件带着一个**全局单调递增的序号（seq）**。这是断线恢复的"唯一事实源"。

### 2.1 核心结构：stream（事件流）+ seq（计数器）双 key

```go
// streamKey:  agent:replay:log:<conversationID>   ← 事件本身（Redis Stream）
// seqKey:     agent:replay:seq:<conversationID>   ← 序号计数器（普通 String）
```

**"先写后发"铁律**：任何事件必须先写进 ReplayLogStore、再发 NATS live（注释明确："生产端保证「先写 replay 后发 live」，因此 live 已出现的 seq 一定可从 replay log 读到"）。**这条铁律是整个机制的基石**——replay log 一定比 live 全。

### 2.2 原子分配序号：一条 Lua 脚本同时搞定"编号 + 落盘"

```go
// replayAppendScriptLua —— 原子完成：INCR seq → XADD（携带 seq）→ 双 key 续期
var replayAppendScript = redis.NewScript(`
local seq = redis.call("INCR", KEYS[2])                    -- 1. 序号递增（原子）
redis.call("XADD", KEYS[1], "MAXLEN", "~", ARGV[1], "*",  -- 2. 事件落盘（带 MAXLEN 截断）
        "seq", seq, "payload", ARGV[2])
redis.call("EXPIRE", KEYS[1], ARGV[3])                     -- 3. TTL 续期
redis.call("EXPIRE", KEYS[2], ARGV[3])
return seq
`)
```

**为什么必须 Lua**：如果不原子，"INCR 拿号"和"XADD 落盘"是两步，中间进程崩了就出现**"拿了号但没落盘"** → 编号不连续 → 重放漏事件。Lua 把"拿号 + 记档"变成**一个原子操作**，保证"**分配的 seq 一定落盘**"（注释原文）。

### 2.3 生命周期参数（工程细节）

| 参数 | 默认值 | 为什么 |
|------|--------|--------|
| TTL | **2 小时** | "覆盖断线重连追赶窗口"——超过 2 小时还没重连的会话，放弃追回是合理的（用户早走了） |
| MAXLEN | **2000 条** | 单会话最多保留 2000 条事件（`MAXLEN ~` 近似截断）——防会话时间太长日志无限膨胀 |

### 2.4 读取：按"游标"往后翻

```go
// ReadAfter：读取 seq > cursor 的全部事件（按 seq 升序）
func (s *ReplayLogStore) ReadAfter(ctx, conversationID, cursor int64) ([]json.RawMessage, error) {
	// Redis Stream ID 为时间戳形式，无法按自定义 seq 过滤，
	// 这里读取全量后按 seq 过滤（Stream 已被 MAXLEN 截断，规模可控）
	entries := XRange(stream, "-", "+")
	for ... {
		seq, payload := parseReplayEntry(entry)
		if seq <= cursor { continue }   // 跳过已读过的
		payload = injectSeq(payload, seq)  // 把 seq 注入事件 JSON 顶层
		payloads = append(payloads, payload)
	}
}
```

**注意 `injectSeq`**：重放的载荷会被注入 `seq` 字段（`SSEEvent.seq`）→ **重放的事件和 live 事件拥有完全相同的形状**，前端处理逻辑统一。

---

## 3. OpenBlockStore：未封口块快照（open_block_store.go）

> **一句话：** 存"还未写完的对话块"的临时快照（如一段还没讲完的话、一个还没完成的工具状态），供 legacy 模式的重连使用。

- **未封口块（Open Block）**：流式输出里"还在写、还没结束"的内容块
- 存储结构：Redis Set 索引 + 每个块一个 key（`agent:open:block:<conv>:<blockNo>`），TTL 自动清理
- 提供 `Upsert / Delete / List / ClearConversation` 四个操作
- **在 seq 感知模式下的角色退居二线**：`ReplayAfter` 非空时**忽略 `LoadInitial`**（wss_hydrator.go:30-31），OpenBlock 只服务 legacy 链路

> **理解分工的一句话**（replay_log_store.go:20-22 注释原文）：
> "OpenBlockStore 只覆盖「未封口块」快照，**封口后增量补不全**；ReplayLogStore 覆盖会话窗口内**全部事件**（MAXLEN 截断 + TTL 过期）。"

---

## 4. CoreHydrator：水位线驱动的"水合器"（wss_hydrator.go）⭐

> **一句话：** 这就是断线恢复的"主控程序"——负责把"历史补发"和"实时流"无缝衔接，对外只暴露一个统一的"事件流出水口"。

### 4.1 两种模式（演进史）

| 模式 | 触发 | 流程 | 缺陷 |
|------|------|------|------|
| **legacy**（旧链路） | `ReplayAfter == nil` | 先订阅 live → 推 OpenBlock 快照 → 直发 live | 快照和 live 之间有**重复/缺口窗口**（注释原文："存在重复/缺口窗口，仅用于无 seq 的旧链路"） |
| **seq 感知**（当前链路） | `ReplayAfter != nil` | 先订阅 live 缓冲 → 从 replay log 按序补发 → live 按 seq 去重 | 无（唯一事实源） |

### 4.2 seq 感知模式的五步水合流程

```
Step 1  前端重连，携带 InitialCursor（客户端已收到的最大 seq；首次订阅传 0）
        └─ "我知道我收到第几号了，从后面补"
Step 2  先 ChanSubscribe 订阅 NATS live（缓冲 256 槽）
        └─ ⭐ 先订阅再回放：回放期间到达的 live 事件缓冲在 coreCh，不会丢失
Step 3  从 replay log 读取 seq > InitialCursor 的事件 → 按序下发 → 推进 watermark
Step 4  发 OnHydrated 信号（"补发完毕，以下是实时"）
Step 5  进入实时循环：live 消息按 seq 与 watermark 比较处理
```

### 4.3 实时循环里的"三兄弟绞杀"（streamSeqAware:108-178）⭐⭐

```go
watermark := cfg.InitialCursor   // 水位线 = 已可靠下发的最大 seq

for {
	case msg := <-coreCh:
		seq := payloadSeq(payload)

		if seq == 0 { send(payload); continue }          // 无 seq（降级期）：直接下发

		if seq <= watermark { continue }                  // 🔴 杀"重"：已重放过，丢弃

		if seq == watermark+1 {                           // 🟢 顺延：正好是下一号
			send(payload); watermark = seq
			flushHydratePending(pending, &watermark)      //   试着连发缓冲里的
			continue
		}

		// 🟡 缺口（seq > watermark+1）：先缓冲
		pending[seq] = payload
		if len(pending) >= hydratePendingCap {            //   缓冲爆了 → 从 replay log 补发
			refillFromReplayLog(pending, &watermark)
		}
}
```

| live 消息的 seq | 处理 | 消灭的故障 |
|----------------|------|-----------|
| `seq <= watermark` | 丢弃 | 🔴 重复 |
| `seq == watermark + 1` | 下发并推进水位线 | 🟢 正常 |
| `seq > watermark + 1` | 缓冲待命（缺口） | 🟡 乱序 |
| 缓冲 64 条仍没补齐 | 从 replay log 补发 | 🟡 乱序/丢失 |

### 4.4 "缺口补发"为什么不可能失败

```go
// refillFromReplayLog：从 replay log 读取 watermark 之后的事件补发
payloads, err := cfg.ReplayAfter(ctx, *watermark)
for _, payload := range payloads {
	delete(pending, seq)  // replay 与缓冲重复时以 replay 为准
	send(payload); watermark = seq
}
flushHydratePending(pending, &watermark)
```

**为什么不可能失败**：因为 §2.1 的"**先写 replay 后发 live**"铁律——live 里出现过的 seq，replay log 里一定已经有。所以"缺口"永远能在日志里补齐，**乱序问题在数学上不可能发生**。

---

## 5. 一句话总结整个机制

> **生产端"先写日志、后发广播"，客户端"记录自己收到哪了、重连时从那里续"——丢、重、乱三兄弟，全部被"一个单调递增的整数（seq）+ 水位线比较"这一个操作消灭。** 这不是花哨的技巧，而是把分布式系统里最难的"可靠传输"问题，**降维成"整数比较"**的工程智慧。

### 面试回答框架（30 秒版）

> "我们的 SSE 断线恢复基于**会话级事件序号（seq）**。架构分三层：ReplayLogStore 用 Redis Stream 持久化带单调递增 seq 的事件（先写 replay 后发 live）；CoreHydrator 负责水合——重连时前端携带断线前的最大 seq 作为游标，服务端先从日志按序补发游标之后的事件，再转接到实时流；实时流里的消息用**水位线去重**——seq 小于等于水位线的一律丢弃，等于水位线+1 的正常下发并推进，出现缺口就先缓冲，缓冲超限就从日志补发。因为'先写后发'保证了日志一定比实时流全，所以缺口永远可以补齐。整个系统把流式传输的丢、重、乱三个问题收敛成水位线整数比较一个操作。"

---

## 反模式（本主题避免）

- **直接拿 NATS 当持久化**——广播是瞬态的，必须另设持久日志（Redis Stream）
- **先发 live 再写日志**——反了！必须"先写日志后发 live"，否则日志永远比实时流少
- **"拿号"和"落盘"分开两步**——中间崩溃会产生"有号无档"，必须用 Lua 原子操作
- **无 seq 的重放**——没有序号的回放必然重复/缺口（legacy 模式的教训）
- **让前端判断事件是否重复**——服务端用水位线搞定，前端只负责渲染

---

## 📖 名词标注表（完整中英对照）

> 本文（SSE 断线恢复）全部关键英文术语，看懂这张表 = 读懂本文。

| 英文 | 中文理解 | 详细解释 |
|------|---------|---------|
| **SSE** (Server-Sent Events) | 服务器推送事件流 | HTTP 长连接，服务端向浏览器持续推事件 |
| **Streaming** | 流式输出 | 内容边生成边发送，而非全部生成完再发 |
| **Token Streaming** | 令牌流式 | LLM 按 token 粒度逐块输出 |
| **Reconnect** | 重连 | 断线后客户端重新建立连接 |
| **断线恢复 (Recovery)** | 恢复中断 | 重连后补齐断线期间漏掉的消息 |
| **Replay** | 回放/重放 | 把历史事件按原顺序重新发送一遍 |
| **Replay Log** | 回放日志 | 持久化的事件日志（Redis Stream） |
| **seq** (sequence) | 序号 | 每条事件全局唯一的递增编号 |
| **单调递增 (Monotonic)** | 只增不减 | seq 只会越来越大，不会回退 |
| **Cursor** | 游标 | 客户端"读到哪了"的标记（最大 seq） |
| **Watermark** | 水位线 | 服务端已可靠下发的进度（本质=seq） |
| **去重 (Dedup)** | 重复剔除 | `seq <= watermark` 的丢弃逻辑 |
| **乱序缓冲 (Pending)** | 待命队列 | 缺口事件先存着，补齐后按序连发 |
| **补发 (Refill)** | 缺口填充 | 从日志读取缺口区间的事件 |
| **Hydrate** | 水合/补齐 | 先补缓存历史再接实时流的过程 |
| **NATS Core** | 消息总线（专有名词） | 服务间实时事件广播通道（瞬态） |
| **Redis Stream** | Redis 有序流（专有名词） | Redis 的持久有序消息结构 |
| **Lua Script** | Redis 脚本（专有名词） | 在 Redis 内原子执行的一组命令 |
| **原子操作 (Atomic)** | 不可分割 | 要么全成要么全不成，中间无窗口 |
| **Open Block** | 未封口块 | 还在写、未完结的对话内容块 |
| **XADD** | 追加事件（Redis 命令） | 向 Redis Stream 尾部添加一条记录 |
| **INCR** | 自增（Redis 命令） | 计数器 +1 |
| **MAXLEN** | 最大长度 | Stream 最多保留多少条（近似截断） |
| **TTL** | 生存时间 | 数据在多长时间后自动过期（2 小时） |
| **live 通道** | 实时通道 | 当前正在发生的事件流 |
| **瞬态广播** | 发了就没了 | 不在线的订阅者收不到 |

---

## 关联文档

- [`case-study-bondseek-multi-agent.md`](./case-study-bondseek-multi-agent.md) — 多 Agent 编排（流式事件来自各专家的 ExecutionEngine 回调）
- [`case-study-bondseek-rag.md`](./case-study-bondseek-rag.md) — RAG 检索（RetrieveCallbacks 各阶段进度也是 SSE 事件）
- [`case-study-bondseek-memory.md`](./case-study-bondseek-memory.md) — 记忆系统（对话结束事件同链路）
- [`notes/agent-interview-questions.md`](../notes/agent-interview-questions.md) — 面试话题索引（第 10 题）
