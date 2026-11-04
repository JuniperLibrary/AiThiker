---
title: 调用模式与重试语义：Invocation Patterns 全解
tags: [tool-use, invocation-patterns, retry, backoff, jitter, retry-after, rate-limit, idempotency, streaming, sse, parallel-tool-calls, go-agent]
created: 2026-11-04
updated: 2026-11-04
status: drafted
---

# 调用模式与重试语义：Invocation Patterns 全解

> **这是什么:** Agent 调用 LLM / 工具时的**调用形态选择**（同步、异步、流式、并行）与**失败后的重试语义**（分类 → 退避 + 抖动 → 尊重 Retry-After → 幂等 → 上限）。面试题 #18 的深水区答案素材。
> **为什么重要:** Agent 的可靠性大半落在"失败了怎么办"上——重试太少会把瞬时抖动变成用户可见的失败，重试太多会打爆自己依赖的服务。不懂重试语义的工程师只会 `for i := 0; i < 3; i++`。
> **本文定位:** 每条论断挂一手来源（RFC/WHATWG 规范原文、AWS 博客原文、OpenAI/Anthropic SDK 源码、本库 `experiments/go-agent` 一手代码与实测），证据分级见 §9 主张-证据矩阵。未核验处显式标 `pending_verification`。
> **关联文档:** [函数调用原理](./function-calling.md)（§6 错误分类与"失败是 Observation"原则）| [认知架构范式](../agent-architecture/cognitive-architecture-paradigms.md)（ReAct 的 Action 失败即观察）| [SSE 流式断线恢复](../../patterns/case-study-bondseek-sse-recovery.md)（重连后的消息去重）
> **代码参考:** `experiments/go-agent/agent/{agent.go,api.go,types.go,fault_test.go}`（2026-11-04 读取并实测）

## 📌 名词速查表（先看这里）

| 英文 | 读作/理解成 | 一句话解释 |
|------|------------|-----------|
| **Backoff（退避）** | "退后一步" | 失败后等一会儿再重试，且等待时间逐次变长（指数退避），避免立刻又打回去 |
| **Jitter（抖动）** | "加随机噪声" | 在退避时间上加随机量，让一群同时失败的客户端**错开**重试时刻，而不是齐射（重试风暴） |
| **Retry-After** | "多久后再来" | 服务器在响应头里直接告诉客户端"等 N 秒再发新请求"（RFC 9110 §10.2.3） |
| **Idempotent（幂等）** | "重复做效果相同" | 同一请求发多次与发一次对服务器效果相同——重试安全的前提（RFC 9110 §9.2.2） |
| **429** | "发太多了" | Too Many Requests：单位时间内请求过多（RFC 6585 §4） |
| **Retry storm / 风暴** | "踩踏事故" | 大量客户端在同一时刻重试，把刚恢复的服务再次打挂 |
| **emittedAny** | "已发不重试" | 流式调用已向用户吐出内容后禁止重试——重试会导致重复输出（本库 go-agent 实现） |
| **`parallel_tool_calls`** | "并行调工具" | 模型一次返回多个 tool_call、客户端并发执行（OpenAPI 规范字段，默认 true） |
| **SSE `retry` 字段** | "断线等 N 毫秒" | 服务器在事件流里下发的重连间隔（毫秒），区别于 HTTP 的 Retry-After（秒）（WHATWG SSE §9.2.5） |
| **Last-Event-ID** | "续传水位线" | 重连时浏览器自动带上的请求头，值为最后收到的事件 id，供服务端断点续传（WHATWG SSE §9.2.4） |

## 1. 五种调用形态：先选形态，再谈重试

调用的第一步不是"失败了怎么办"，而是**选对形态**——形态决定了失败的可观测性与可重试性边界。

| 形态 | 适用 | 成功判据 | 失败/重试的特殊性 | 一手依据 |
|---|---|---|---|---|
| **同步（request-response）** | 工具执行、短 LLM 调用 | 拿到完整响应体 | 最简单：失败即抛错，按 §2 分类重试 | go-agent `ChatCompletion`（`agent.go:150`） |
| **异步（accepted）** | 批处理、长任务 | 先拿 **202 Accepted**，再轮询状态 | 202 ≠ 成功："request has been accepted for processing, but the processing has not been completed"，且"**no facility in HTTP for re-sending a status code from an asynchronous operation**"——异步任务的结果不能靠重发原请求拿回，必须轮询状态端点 | RFC 9110 §15.3.3 ✅ |
| **流式（streaming/SSE）** | 首字节延迟敏感的 LLM 对话 | 事件流完整收到 `done`/`finish_reason` | **已吐出内容后不可重试**（§6）；断线恢复走协议级重连 + 断点续传（§6.2） | WHATWG SSE §9.2 ✅ + go-agent `agent.go:181` ✅ |
| **并行（parallel tool calls）** | 多个独立工具一次调完 | 每个 tool_call 各自收到结果 | **部分失败是常态**：结果必须按 `tool_call_id` 回填，成功/失败逐个上报（§7） | OpenAPI spec ✅ |
| **批量/后台（background）** | 大规模离线任务 | 任务 id + 状态查询 | 失败重试在任务层做，不在请求层 | `pending_verification`（OpenAPI `background` 字段语义未逐条核验） |

> **面试一句话:** "形态选择的本质是**失败语义的选择**——同步失败立刻可见可重试，异步失败要轮询，流式失败有'已发不可撤回'的硬边界，并行失败是部分成功。"

## 2. 重试的判定：先分类，再重试

**不加区分地一律重试是错的。** 判定顺序：服务器指令 > 错误分类 > 默认策略。

### 2.1 分类表（可重试 / 不可重试）

| 类别 | 典型状态/错误 | 处理 | 一手依据 |
|---|---|---|---|
| 瞬时·网络 | 连接拒绝、连接中断、超时 | ✅ 退避重试 | go-agent `retryable()`：`ClassNetwork, ClassTimeout` → true（`agent.go:251-257`）✅ |
| 瞬时·限流 | **429** | ✅ 退避重试（优先服从 Retry-After） | 两家 SDK `_should_retry` 均含 `status_code == 429 → True` ✅ |
| 瞬时·服务端 | **408**（请求超时）、**409**（锁冲突）、**≥500** | ✅ 退避重试 | openai/anthropic SDK `_should_retry` 对 408、409、429 与 ≥500 一致返回 True ✅ |
| 客户端·参数/格式 | 其他 4xx（400/404/422…） | ❌ 不重试——**反馈给模型自改** | go-agent `ClassClient` 注释"其他 4xx（不可重试）"（`types.go:122`）✅；SDK `_should_retry` 最后 `return False` ✅ |
| 协议·解析 | JSON/SSE 解析失败、行超长 | ❌ 不重试（重试拿到的还是坏数据） | go-agent `ClassProtocol`（`types.go:123`）✅ |
| 安全·预算 | 上下文预算不足 | ❌ 请求根本不发出 | go-agent `ClassBudget`"请求未发出、历史不动"（`types.go:125`、`agent.go:262-277`）✅ |

### 2.2 服务器指令优先于本地策略（两个真实实现）

两家 SDK 的判定函数都是**三层结构**，优先级从高到低：

1. **显式头**：`x-should-retry: true/false`（非标准头）——服务器直接说了算；
2. **Retry-After 上限检查**（仅 openai）：服务器要求等太久就放弃（见 §5.2）；
3. **状态码白名单**：408 / 409 / 429 / ≥500 → 重试，否则不重试。

```python
# openai-python src/openai/_base_client.py（2026-11-04 读取 main 分支）
def _should_retry(self, response) -> bool:
    retry_after = self._parse_retry_after_header(response.headers)
    if retry_after is not None and retry_after > MAX_RETRY_AFTER_DELAY:  # 120s
        return False                       # 服务器要我等 >2 分钟 → 放弃重试
    if response.headers.get("x-should-retry") == "true":  return True   # 显式指令
    if response.headers.get("x-should-retry") == "false": return False
    if response.status_code == 408:  return True   # 请求超时
    if response.status_code == 409:  return True   # 锁冲突
    if response.status_code == 429:  return True   # 限流
    if response.status_code >= 500: return True    # 服务端错误
    return False                                   # 其余（含参数错）不重试
```

> **来源:** [openai-python `_base_client.py`](https://github.com/openai/openai-python/blob/main/src/openai/_base_client.py)（anthropic-sdk-python 同构，仅无 Retry-After 上限层）✅ 2026-11-04 核验

### 2.3 429 的语义盲区（面试深水区）

RFC 6585 §4 对 429 的定义有一条常被忽略的声明：

> "Note that this specification **does not define how the origin server identifies the user, nor how it counts requests**."
> —— [RFC 6585 §4](https://www.rfc-editor.org/rfc/rfc6585.txt) ✅ 2026-11-04 核验

这句话的推论：**客户端无法从 429 本身区分"我发太快了（等几秒就好）"和"我的配额用光了（等到明天也没用）"**——计数窗口、限流主体（按用户/按资源/按 IP）全部由服务器私有定义。所以：

- 429 + `Retry-After: 3600` → 大概率是窗口型限流，**服从**（RFC 6585 示例即此形态）；
- 429 无 Retry-After、错误体提示 quota exhausted → 持久配额耗尽，**重试无效**，应降级/告知用户；
- 两家 SDK 的 `_should_retry` 对 429 一律 `True`——**SDK 假设的是窗口型限流**，这是它们的默认立场，不是普适真理（反例见 §10 卡片）。

429 的响应规范要求："SHOULD include details explaining the condition, and MAY include a Retry-After header"；且"**Responses with the 429 status code MUST NOT be stored by a cache**"（RFC 6585 §4）✅。

## 3. 退避与抖动：公式、默认值与实测

### 3.1 指数退避的默认值（两家 SDK 源码，逐字核验）

| 常数 | openai-python | anthropic-sdk-python | 含义 |
|---|---|---|---|
| `DEFAULT_MAX_RETRIES` | **2** | **2** | 默认最多重试 2 次（共 3 次请求） |
| `INITIAL_RETRY_DELAY` | **0.5** s | **0.5** s | 第 1 次重试前等 0.5s |
| `MAX_RETRY_DELAY` | **8.0** s | **8.0** s | 退避封顶 8s |
| `MAX_RETRY_AFTER_DELAY` | **120** s（`2 * 60`） | 无此常数 | 服务器 Retry-After 超过 120s 就不伺候了 |

> **来源:** [openai-python `_constants.py`](https://github.com/openai/openai-python/blob/main/src/openai/_constants.py)、[anthropic-sdk-python `_constants.py`](https://github.com/anthropics/anthropic-sdk-python/blob/main/src/anthropic/_constants.py) ✅ 2026-11-04 核验

退避公式（`_calculate_retry_timeout`，两家同构）：

```
sleep = min(INITIAL_RETRY_DELAY * 2^n, MAX_RETRY_DELAY) * (1 - 0.25 * random())
#      = min(0.5 * 2^n, 8.0)  *  [0.75, 1.0]
```

- n = 已重试次数 → 实际等待序列约 **0.375~0.5s → 0.75~1s → 封顶 6~8s**；
- **抖动只削减 0~25%**（`jitter = 1 - 0.25 * random()`）——这是"保守抖动"，不是全抖动（见 §3.2）。

> **来源:** openai/anthropic `_base_client.py::_calculate_retry_timeout` ✅ 2026-11-04 核验（两家函数体逐字对照）

本库 go-agent 的实现（另一组参数）：

```go
// experiments/go-agent/agent/agent.go:16-17, 181-191（2026-11-04 读取）
maxRetries     = 2                    // 最多重试 2 次 → 共 3 次请求
retryBaseDelay = 200 * time.Millisecond
// ...
if attempt >= maxRetries || emittedAny || ctx.Err() != nil || !retryable(lastErr) {
    break                             // 上限 / 已发出内容 / 截止 / 不可重试 → 停
}
select {
case <-time.After(retryBaseDelay << uint(attempt)):   // 200ms << attempt → 200ms, 400ms
case <-ctx.Done():
    lastErr = ctx.Err()
}
```

即退避序列为 **200ms → 400ms**（`<<` 位移实现指数），无抖动、无 Retry-After——这是**刻意的简化**，代价见 §10 卡片。

### 3.2 为什么要抖动：AWS 2015 的对照实验结论

[AWS Architecture Blog《Exponential Backoff And Jitter》（Marc Brooker, 2015-03-04）](https://aws.amazon.com/blogs/architecture/exponential-backoff-and-jitter/)用仿真对比了四种策略（原文结论逐字核验 ✅ 2026-11-04）：

| 策略 | 公式（原文定义） | 原文结论 |
|---|---|---|
| 无抖动指数退避 | `sleep = base * 2^n` | "**the clear loser**"——工作量最大、耗时最长，"takes so much more time we have to leave it off the graph" |
| **Equal Jitter** | 保留一半退避 + 随机：`temp/2 + rand(0, temp/2)` | "the loser" among jittered——比 Full Jitter 做更多工作且耗时长得多 |
| **Full Jitter（全抖动）** | `sleep = rand(0, temp)` | 工作量最少；比 Decorrelated **略多耗时** |
| **Decorrelated Jitter** | 基于上次随机值递增 | 工作量比 Full 多，耗时略少；两者"present a substantial decrease in client work and server load" |

原文收束句："**The return on implementation complexity of using jittered backoff is huge, and it should be considered a standard approach for remote clients.**" ✅

> **对照读法:** openai/anthropic SDK 的 `1 - 0.25*random()` 是介于"无抖动"与"Equal Jitter"之间的**微抖动**——只解决"完全同刻"问题，不解决"分布展宽"问题；AWS 的 Full Jitter 才是重载场景的标准答案。两家 SDK 选微抖动是**保守默认**（LLM API 单客户端重试占比小），不是最优解。

### 3.3 实测：本库 go-agent 的退避分布（N=20）

测量日期 2026-11-04，环境 Windows / go test，命令与结果见 §11。关键分布：

| 测试 | N | min | p50 | p95 | max | 语义 |
|---|---|---|---|---|---|---|
| `TestFaultRateLimitThenSuccess`（429×2→成功） | 20 | 600ms | **600ms** | **600ms** | 610ms | 200+400ms 退避后第 3 次成功，attempts=3 |
| `TestFaultConnectionRefused`（连接拒绝） | 20 | 600ms | 600ms | 600ms | 630ms | 同上，网络类可重试 |
| `TestFaultRetriesStopAfterContextDeadline`（ctx 截止） | 20 | 50ms | 50ms | 50ms | 50ms | 截止 50ms ≪ 200ms 退避 → 立即停，不空等 |
| `TestFaultStreamNotRetriedAfterEmit`（已发不重试） | 20 | 0ms | 0ms | 0ms | 0ms | **零重试**，直接失败 |

**p50 = p95 = 600ms（零方差）**——这不是荣耀，是**无抖动的直接证据**：固定退避下每次重试时刻完全可预测，并发场景必然对齐。AWS 2015 的"clear loser"结论对本实现同样成立（本实测只覆盖单客户端，多客户端对齐效应未测——`pending_verification`，见 §11）。

## 4. Retry-After：服务器说了算（但有两种形式、两家两种态度）

### 4.1 规范原文（RFC 9110 §10.2.3）

```
Retry-After = HTTP-date / delay-seconds
delay-seconds  = 1*DIGIT          ; 非负十进制整数，单位秒

例：Retry-After: Fri, 31 Dec 1999 23:59:59 GMT   （HTTP-date 形式）
    Retry-After: 120                               （秒数形式，delay = 2 分钟）
```

- 发送时机：**503** 时表示"服务预计不可用多久"；**3xx** 时表示"至少等多久再发重定向请求"；
- 429 场景由 RFC 6585 §4 授权："MAY include a Retry-After header indicating how long to wait" ✅

> **来源:** [RFC 9110 §10.2.3](https://www.rfc-editor.org/rfc/rfc9110.txt)、[RFC 6585 §4](https://www.rfc-editor.org/rfc/rfc6585.txt) ✅ 2026-11-04 逐字核验

### 4.2 两家 SDK 的态度完全不同（源码级差异，面试可用）

| | openai-python | anthropic-sdk-python |
|---|---|---|
| 是否服从 Retry-After | ✅ 服从，**但仅当 ≤120s** | ✅ 无条件服从 |
| 超限行为 | `_should_retry` 直接 `return False`——**放弃重试**（"Not retrying because Retry-After exceeds the maximum of 120 seconds"） | 上限仅 `min(retry_after, 4_294_967.0)`——那是 Windows sleep 平台上限（~50 天），注释原话："**If the API asks us to wait a certain amount of time, just do what it says**" |
| 解析实现 | `_parse_retry_after_header`：优先 `retry-after-ms`（毫秒，非标准头），再 `retry-after`；**用 `float(value)` 解析** | 同名函数（未逐行核验——`pending_verification`） |
| HTTP-date 形式 | `float("Fri, 31 Dec...")` 抛 `ValueError` → 跳过 → **回退到本地指数退避** | `pending_verification` |

> **来源:** openai `_base_client.py::_should_retry / _calculate_retry_timeout / _parse_retry_after_header`、anthropic `_base_client.py::_calculate_retry_timeout` ✅ 2026-11-04 逐字核验

**工程推论（可证伪）:** 规范给了两种形式，但主流 SDK 的实用解析只认数字——**服务器若发 HTTP-date 形式，客户端会静默退回本地退避**。这不是 bug 容忍，是解析器与规范的缝隙；写网关/代理时若要转发 Retry-After，注意 `retry-after-ms` 这个非标准头只有 openai SDK 认。

## 5. 幂等：重试的前提条件

### 5.1 规范怎么说（RFC 9110 §9.2.2）

> "A request method is considered **idempotent** if the intended effect on the server of multiple identical requests with that method is the same as the effect for a single such request."（PUT、DELETE 与安全方法是幂等的）
> "Idempotent methods are distinguished because **the request can be repeated automatically** if a communication failure occurs before the client is able to read the server's response."
> "**A client SHOULD NOT automatically retry a request with a non-idempotent method** unless it has some means to know that the request semantics are actually idempotent…"
> —— [RFC 9110 §9.2.2](https://www.rfc-editor.org/rfc/rfc9110.txt) ✅ 2026-11-04 逐字核验

推论链：**LLM Chat Completions 是 POST（非幂等方法）→ 规范默认不许自动重试 → 能重试是因为应用层知道"重发同一 prompt 不改变世界"**（副作用只在计费/限流层）。而一旦请求里带着**外部副作用**（发邮件、下单），自动重试就必须配幂等键。

### 5.2 三层幂等保障（真实实现对照）

| 层 | 机制 | 一手位置 |
|---|---|---|
| **SDK 层** | 重试时附带幂等键头：`f"stainless-python-retry-{uuid.uuid4()}"` | openai `_base_client.py::_idempotency_key` ✅ |
| **会话状态层** | user 消息**只入历史一次**；失败整轮回滚到 `base`（`a.messages[:base]`），绝不产生半截状态 | go-agent `agent.go:250`（注释"幂等前提：user 消息只入历史一次"）、`agent.go:194-208`（结局收敛：成功提交 / 失败回滚）✅ |
| **观测层** | `TurnResult.Attempts` 记录实际请求次数，重试可观测、可断言 | go-agent `types.go:111` + `fault_test.go:58`（断言 attempts=3）✅ |

> **金句:** "**重试的安全性不是来自'多试几次'，而是来自'试几次效果一样'——幂等是重试的许可证，没有它，退避公式再漂亮也是在放大事故。**"

## 6. 流式调用：已发不可重试 + 协议级重连

### 6.1 应用层：`emittedAny` 硬边界

流式与同步的根本区别：**第一个字节吐给用户后，"重试"就不再是一个可选项**——重试意味着用户看到两遍内容（或内容错乱）。

```go
// experiments/go-agent/agent/agent.go:162-183（2026-11-04 读取）
res, err := a.client.StreamChatCompletion(ctx, newReq(true), func(s string) error {
    if !started {
        ttftMs = msSince(callStart)
        started = true
        emittedAny = true               // 第一个内容块 → 置位
    }
    return emit(s)
})
// ...
if attempt >= maxRetries || emittedAny || ctx.Err() != nil || !retryable(lastErr) {
    break                               // 已发出内容 → 无条件停止重试
}
```

BondSeek 生产实现同样有此规则（转述记录："**已向客户端发送过内容则不重试**（防止重复输出）"）——但该转述来自 `patterns/case-study-bondseek-multi-agent.md:247`，属 **secondary/hearsay**，原始 trace 可访问性仍是 SPEC §9 待核 #1 ⏳，本库不以其作论据。

### 6.2 协议层：WHATWG SSE 的重连语义（原文核验）

| 机制 | 规范原文语义 | 用途 |
|---|---|---|
| **`retry` 事件字段** | "If the field value consists of only ASCII digits, then interpret the field value as an integer in base ten, and **set the event stream's reconnection time to that integer**"（单位毫秒） | 服务器动态下发重连间隔——**流内版的 Retry-After** |
| **初始重连时间** | "A reconnection time, in milliseconds. This must initially be an implementation-defined value, probably in the region of a few seconds" | 未收到 `retry` 前用浏览器自定义值（通常几秒） |
| **`Last-Event-ID` 头** | "If the EventSource object's last event ID string is not the empty string … **Set (`Last-Event-ID`, lastEventIDValue) in request's header list**" | 重连时浏览器自动携带最后事件 id → 服务端**断点续传**（配 `id` 字段使用） |
| **fail 与 reconnect 的区别** | "**Once the user agent has failed the connection, it does not attempt to reconnect**" | 规范区分"可恢复断线"（自动重连）与"致命失败"（不重连）——重试策略必须区分两者 |

> **来源:** [WHATWG SSE §9.2](https://html.spec.whatwg.org/multipage/server-sent-events.html) ✅ 2026-11-04 逐字核验

**三个易混点（面试追问高发）:**

1. **HTTP `Retry-After`（秒）≠ SSE `retry` 字段（毫秒）**——单位不同、通道不同（响应头 vs 事件流内）、语义都是"隔多久再连"；
2. **`Last-Event-ID` 是重连去重/续传的协议钩子**，但"去重"本身要服务端配合（按 id 幂等回放）——协议只保证"带上水位线"，不保证"不重复"；
3. **协议层重连与应用层重试是两套预算**：SSE 断线浏览器自动重连，应用层若再叠加自己的重试循环，会出现双层重连风暴（见 §12 红队 Q3）。

本库 [`sse.go`](../../experiments/go-agent/agent/sse.go) 对 `retry` 字段的处理是**按规范消费、本用途只用 data**：`case "event", "id", "retry":` 忽略字段值但事件仍派发（`sse.go:25-72`、`sse_test.go:53-54` 断言 `retry: 1000` 时事件照常派发）✅——这是"协议字段按规范解析、按需消费"的最小实现示范。

## 7. 并行调用与部分失败

### 7.1 `parallel_tool_calls`：默认开启的并发

OpenAPI 规范中该字段的两处定义（逐字核验 ✅ 2026-11-04）：

```yaml
ParallelToolCalls:
  description: Whether to enable parallel function calling during tool use.
  type: boolean
  default: true
# 另一处（Chat Completions 请求参数）：
#   description: Whether to allow the model to run tool calls in parallel.
#   default: true
```

> **来源:** [openai-openapi `openapi.yaml`](https://github.com/openai/openai-openapi/blob/master/openapi.yaml) ✅

**默认 true 的工程含义**：模型一次可能返回 N 个 tool_call，客户端**并发执行**——于是失败不再是"一次调用失败"，而是"**N 次里挂了 k 次**"的部分失败。

### 7.2 部分失败的协议判据：`tool_call_id` 绑定

```yaml
# OpenAPI spec（逐字核验 ✅ 2026-11-04）
tool_call_id:
  type: string
  description: Tool call that this message is responding to.
required: [role, content, tool_call_id]    # tool 结果消息的必填三元组
```

- 每条工具结果**必须**指回它响应的 `tool_call_id`——这是模型把 N 个结果对号入座的唯一依据；
- 部分失败的正确姿势：**成功的照常回填，失败的也回填**（错误文本作为该 tool_call 的结果）——而不是整个回合作废重来。错误结果是给模型的 Observation，模型看到后可换工具/改参数/放弃（与 [function-calling.md §6](./function-calling.md) 的"失败是 Observation"原则闭环）；
- `pending_verification`：模型侧收到"部分 tool_call 失败"后的具体再决策行为（是否必然重试失败项、是否会改参数）——OpenAI/Anthropic 文档页在本环境不可达（`docs.openai.com` DNS 解析失败、Anthropic 文档返回 SPA 壳），未逐条核验，**不写入论断**。

## 8. 落地实例：go-agent 的完整重试闭环（一手代码 + 实测）

### 8.1 结构（每一行都可打开核对）

| 组件 | 位置 | 职责 |
|---|---|---|
| 错误分类 | [`agent/types.go` L116-126](../../experiments/go-agent/agent/types.go) | 10 类：network/timeout/canceled/rate_limit/server/client/protocol/empty/budget |
| 可重试判定 | [`agent/agent.go` L251-257](../../experiments/go-agent/agent/agent.go) | `retryable()`：仅 network/timeout/server/rate_limit → true |
| 退避循环 | [`agent/agent.go` L181-191](../../experiments/go-agent/agent/agent.go) | 上限 2 次 + `emittedAny` + ctx + 分类四重闸门；`200ms << attempt` |
| 状态收敛 | [`agent/agent.go` L194-208](../../experiments/go-agent/agent/agent.go) | 成功提交成对消息 / 失败回滚到 base / ctx 中止单独归类 |
| 可观测 | [`agent/agent.go` L233-245](../../experiments/go-agent/agent/agent.go) | 结构化日志含 `attempts`、`err_class`、`outcome`——**重试次数是第一等公民** |
| 故障注入测试 | [`agent/fault_test.go`](../../experiments/go-agent/agent/fault_test.go) | 429/连接拒绝/客户端错/流中断/ctx 截止 8 类故障场景 |

### 8.2 与 SDK 默认值的对照（设计取舍，面试可讲）

| | go-agent | openai/anthropic SDK |
|---|---|---|
| 重试次数 | 2（共 3 次请求） | 2（共 3 次请求） |
| 退避序列 | 200ms → 400ms（固定） | ~0.5s → ~1s → 8s 封顶（带 0~25% 抖动） |
| Retry-After | **不解析**（简化） | openai ≤120s 服从 / anthropic 无条件服从 |
| 流式已发 | ✅ `emittedAny` 硬停 | SDK 层无此概念（发生在应用层） |
| 幂等 | 消息回滚 + attempts 断言 | `stainless-python-retry-{uuid}` 幂等头 |
| 重试层数 | **仅 1 层**（[`api.go`](../../experiments/go-agent/agent/api.go) 本身不重试） | 仅 1 层（SDK 内部） |

### 8.3 调用模式代码示例（AGENTS.md 质量检查项）

```go
// 同步调用 + 分类重试的最小骨架（形态：同步；语义：go-agent 同款）
result, err := agent.Turn(ctx, "帮我查一下天气")
// 内部已按 §2 分类：429/5xx/网络 → 退避重试；400/422 → 直接返回给上层反馈模型
// result.Attempts > 1 即发生了重试，可直接进日志/指标

// 流式调用 + 已发不重试（形态：流式；语义：agent.go:162-183）
err := client.StreamChatCompletion(ctx, req, func(delta string) error {
    emittedAny = true                 // 第一个 delta 后：任何后续错误都不再重试
    return write(delta)               // 推给客户端
})
if err != nil && !emittedAny {
    // 还没吐过内容 → 可按退避策略安全重试（幂等前提：prompt 未变）
}

// 并行工具调用（形态：并行；语义：openapi tool_call_id 绑定）
for _, tc := range resp.ToolCalls {          // N 个 tool_call 并发执行
    go func(tc ToolCall) {
        out, err := execute(tc)              // 单个失败不拖垮整批
        if err != nil {
            out = fmt.Sprintf("tool error: %v", err)  // 失败也回填，作为 Observation
        }
        results <- ToolResult{ID: tc.ID, Output: out}  // 按 tool_call_id 对号回座
    }(tc)
}
```

## 9. 主张-证据矩阵（SPEC §5.2）

> 图例：✅ verified（打开原文核对）| 🟡 secondary（本库转述，非一手）| ⏳ hearsay（凭记忆/未核验）；验证日期均为 **2026-11-04**。

| # | 主张 | 原文位置 | 等级 | 验证日期 | 反例/边界 |
|---|---|---|---|---|---|
| 1 | 429 = 单位时间请求过多；SHOULD 说明细节、MAY 带 Retry-After；MUST NOT 缓存 | RFC 6585 §4 | ✅ | 2026-11-04 | 规范不定义计数/识别方式 → 限流主体不可知 |
| 2 | `Retry-After = HTTP-date / delay-seconds`，秒数为非负整数 | RFC 9110 §10.2.3 | ✅ | 2026-11-04 | 主流 SDK 实用解析只认数字，HTTP-date 会静默回退 |
| 3 | 202 = 已受理未完成；HTTP 无法重发异步操作的状态码 | RFC 9110 §15.3.3 | ✅ | 2026-11-04 | 异步结果必须轮询，重试原请求拿不到结果 |
| 4 | 幂等方法可自动重试；非幂等方法 SHOULD NOT 自动重试 | RFC 9110 §9.2.2 | ✅ | 2026-11-04 | LLM POST 是非幂等方法，重试靠应用层语义豁免 |
| 5 | 无抖动指数退避是"clear loser"；抖动应是远程客户端标准做法 | AWS 博客（Brooker, 2015）四种策略对照段 | ✅ | 2026-11-04 | 仿真基于 OCC 假设，单客户端场景收益小 |
| 6 | SDK 默认重试 2 次、0.5s 起、8s 封顶、抖动 0~25% | [openai `_constants.py`](https://github.com/openai/openai-python/blob/main/src/openai/_constants.py) + `_calculate_retry_timeout` | ✅ | 2026-11-04 | 版本钉死 2026-11-04 main 分支，升级需复核 |
| 7 | SDK 可重试状态码 = {408,409,429,≥500}，其余不重试；`x-should-retry` 头优先 | 两家 `_should_retry` 函数体 | ✅ | 2026-11-04 | 429 一律 True 是"窗口型限流"假设 |
| 8 | openai 对 Retry-After >120s 放弃重试；anthropic 无条件服从（上限仅平台上限） | openai `_should_retry`、anthropic `_calculate_retry_timeout` | ✅ | 2026-11-04 | anthropic `_parse_retry_after_header` 逐行未核（⏳） |
| 9 | SSE `retry` 字段（纯 ASCII 数字）设重连时间（毫秒）；fail 后不再重连 | WHATWG SSE §9.2.5 / §9.2.3 | ✅ | 2026-11-04 | 浏览器初始值 implementation-defined |
| 10 | 重连时非空 last event ID → 自动携带 `Last-Event-ID` 头 | WHATWG SSE §9.2.3 | ✅ | 2026-11-04 | 只带水位线，去重要服务端配合 |
| 11 | `parallel_tool_calls` 默认 true；tool 结果消息必填 `tool_call_id` | [openai-openapi `openapi.yaml`](https://github.com/openai/openai-openapi/blob/master/openapi.yaml)（ParallelToolCalls / tool message） | ✅ | 2026-11-04 | 模型侧部分失败再决策行为未核（⏳ pending） |
| 12 | go-agent：429/网络/5xx 可重试，4xx/协议错不可重试 | `experiments/go-agent/agent/agent.go:251-257`、`types.go:116-126` | ✅ | 2026-11-04 | 单实现样本，非通用标准 |
| 13 | go-agent：已发出内容（emittedAny）后无条件停止重试 | [`agent/agent.go` L181](../../experiments/go-agent/agent/agent.go) + [`fault_test.go`](../../experiments/go-agent/agent/fault_test.go)（StreamNotRetriedAfterEmit） | ✅ | 2026-11-04 | — |
| 14 | 退避序列 200ms→400ms，N=20 实测 p50=p95=600ms（零方差） | [`agent/agent.go` L17,185](../../experiments/go-agent/agent/agent.go) + 2026-11-04 实测（§11） | ✅ | 2026-11-04 | 单机单客户端，未测并发对齐 |
| 15 | BondSeek：3 次退避 200/400/600ms、已发不重试、工具 3 连败硬降级 | `patterns/case-study-bondseek-multi-agent.md:247-248`（转述） | 🟡 | — | 原始 trace 不可访问（SPEC §9 待核 #1），不作论据 |
| 16 | openai SDK 重试时附带 `stainless-python-retry-{uuid}` 幂等键 | openai `_base_client.py::_idempotency_key` | ✅ | 2026-11-04 | 幂等键是否被服务端识别未核（⏳） |

**矩阵无空行**；16 项中 14 项 ✅ 可由他人打开原文/代码核对，1 项 🟡 明确降级，2 处 ⏳ 已就地标注。

## 10. 失效边界卡（SPEC §5.3）

### 卡片 1：指数退避 + Retry-After 何时不成立？

**主张:** "失败了就指数退避 + 尊重 Retry-After"是通用正确答案。
**失效条件（任一即失效）:**

| # | 失效条件 | 一手反例/证据 | 失效后果 |
|---|---|---|---|
| 1 | **无抖动 + 多客户端** | AWS 2015：无抖动指数退避 "the clear loser"；本实现实测 p50=p95=600ms 零方差（§3.3） | 并发客户端退避时刻对齐 → 恢复瞬间二次踩踏 |
| 2 | **非幂等副作用** | RFC 9110 §9.2.2："SHOULD NOT automatically retry a non-idempotent method" | 重发 = 重复下单/重复发信 |
| 3 | **持久配额耗尽型 429** | RFC 6585 §4 不定义计数方式 → 客户端无法区分窗口型/配额型 | 退避 N 次后仍失败，纯烧延迟（窗口型假设下 SDK 一律 `429→True`） |
| 4 | **服务器要求等待 > 客户端预算** | openai `_should_retry`：Retry-After >120s → `return False` | 两种坏结局：要么挂 2 分钟，要么放弃——必须二选一 |
| 5 | **流式已吐出内容** | go-agent `emittedAny`；SSE "failed → does not attempt to reconnect" | 重试 = 用户看到重复/错乱输出 |
| 6 | **双层重试叠加** | SDK 内部 2 次 × 应用层 2 次 → 最多 (2+1)² = 9 次请求 | 重试风暴的成因之一（本库 go-agent 故意单层：[`api.go`](../../experiments/go-agent/agent/api.go) 不含重试） |
| 7 | **解析失败静默回退** | openai `_parse_retry_after_header`：`float(value)` 遇 HTTP-date → `ValueError` → 跳过 | 服务器的明确指令被忽略，退到本地猜测值 |

**已知批评/负面结果:** openai-python issue [#1102 "Functionality to adjust exponential backoff associated with max_retries option"](https://github.com/openai/openai-python/issues/1102)（2024-01 开放至今）——真实用户对默认退避不可调的不满，一手失效资料 ✅；issue [#4234](https://github.com/openai/openai-python/issues/4234)（2026-11-03）仍在补 Retry-After 秒数形式的测试覆盖——**连 SDK 自己的 Retry-After 测试都是近期才补的** ✅。

**何时卡片成立（正面边界）:** 单客户端、窗口型限流、幂等副作用、同步/流式未 emit 阶段——这四条同时满足时，"分类 + 退避 + Retry-After + 上限"就是完整策略。

## 11. 量化与最小复现（SPEC §5.4 / D4-D5）

**测量报告（2026-11-04）**

- 环境：Windows / Go（`experiments/go-agent`，`go.mod` 钉死依赖）
- 命令（第三方干净机器一条命令可跑）：

```bash
go test ./agent/ -run TestFault -count=20 -v
```

- 结果：**8 个故障测试 × 20 轮 = 160/160 PASS，0 FAIL**（TestFaultConnectionRefused / RateLimitThenSuccess / ClientErrorNoRetry / StreamErrorChunk / TruncatedStream / StreamNotRetriedAfterEmit / LineTooLong / RetriesStopAfterContextDeadline）
- 关键分布（N=20，毫秒）：

| 测试 | min | p50 | p95 | max | 语义 |
|---|---|---|---|---|---|
| RateLimitThenSuccess | 600 | 600 | 600 | 610 | 429×2 → 退避 200+400ms → 第 3 次成功（attempts=3 断言） |
| ConnectionRefused | 600 | 600 | 600 | 630 | 网络类同路径 |
| RetriesStopAfterContextDeadline | 50 | 50 | 50 | 50 | ctx 截止先到 → 立即停，不空等 200ms |
| StreamNotRetriedAfterEmit | 0 | 0 | 0 | 0 | 已 emit → 零重试 |

- **效果量声明:** 本组测量无对照变体（如"无 emittedAny 守卫"），故只报分布、**不报 effect size**——变体对照属 `pending_verification`（见下）。
- `pending_verification` 清单：
  1. 多客户端并发下无抖动退避的对齐效应（需 N 客户端 harness，未建）；
  2. 加 Full Jitter 变体后的 p95 对比（需改 `retryBaseDelay` 注入点做 A/B，未做）；
  3. 内网模型接口真实 429 场景的 Retry-After 分布（SPEC §9 待核 #4，未测）。

## 12. 红队自检（SPEC §5.6：预演面试官挑战）

**Q1：429 也可能是配额用光了，重试有什么用？**
A：好问题——这正是 §2.3 的语义盲区。RFC 6585 明说不定义计数方式，所以客户端**无法从状态码区分**窗口型（等 5 秒就好）和配额型（等到明天也没用）。判据只能靠：Retry-After 的量级（3600s 级 → 窗口型）、错误体文案（quota exhausted → 配额型）。SDK 的 `429→True` 是对窗口型的乐观假设；生产代码应该解析错误体，配额型直接降级而不是白烧两次退避。→ 已补证：RFC 6585 §4 原文（✅）。

**Q2：你自己写的实现没有抖动，AWS 说无抖动是 clear loser，怎么解释？**
A：承认，这正是 §3.3 实测 p50=p95=600ms 零方差的含义——单客户端测试里无抖动不显现问题，并发下退避时刻会对齐。go-agent 是教学实现，选了最简形式；生产实现应至少做 openai 式 0~25% 微抖动，重载场景直接上 AWS Full Jitter。已登记为改进项 + pending 变体对照实验。→ 不辩解，给失效条件 + 改进路径。

**Q3：SDK 内部重试 2 次，你的 Agent 层又重试 2 次，是不是最多 9 次请求？**
A：算术对——(2+1)²=9，这就是双层重试叠加。原则是**重试只放一层**：要么 SDK 内部重试、应用层快速失败，要么关掉 SDK 重试（`max_retries=0`）由应用层接管。go-agent 选了后者：[`api.go`](../../experiments/go-agent/agent/api.go) 本身零重试（错误分类后立即上抛），重试全部集中在 [`agent.go`](../../experiments/go-agent/agent/agent.go) 一层，全链路最多 3 次请求，可由 `attempts` 字段断言。→ 已补证：代码单层结构（✅）。

**Q4：服务器发 `Retry-After: Fri, 31 Dec 1999 23:59:59 GMT`（HTTP-date 形式），你的客户端怎么办？**
A：规范允许两种形式（RFC 9110 §10.2.3），但 openai SDK 的 `_parse_retry_after_header` 用 `float(value)` 解析，HTTP-date 会 `ValueError` 被跳过——**静默回退到本地指数退避**。即：规范给了精确指令，实用解析器只认秒数形式。写代理/网关时要么补 HTTP-date 解析，要么明确声明只支持 delay-seconds。→ 已补证：SDK 解析代码（✅）；anthropic 侧解析实现未逐行核（⏳）。

**Q5：SSE 的 `retry` 字段和 HTTP 的 `Retry-After` 是一回事吗？**
A：语义同源（"隔多久再连"），三处不同：**单位**（毫秒 vs 秒）、**通道**（事件流内字段 vs 响应头）、**作用域**（只管 EventSource 重连 vs 任何后续请求）。另外 SSE 重连还带 `Last-Event-ID` 水位线做续传，HTTP Retry-After 没有对应物。→ 已补证：WHATWG §9.2 原文（✅）。

**Q6：并行工具调用挂了一半，怎么办？**
A：协议层已给出判据：tool 结果消息必填 `tool_call_id`（OpenAPI spec ✅），所以**成功的照常回填、失败的以错误文本回填**——每个 tool_call 都有交代，模型看到部分失败的 Observation 后自行决策（换工具/改参数/放弃）。整批作废重来是错的：既浪费已成功的调用，也剥夺了模型的决策权。模型侧具体再决策行为未核验（pending），只讲到协议保证这一层。

**Q7：为什么重试次数默认只有 2 次？多试几次不好吗？**
A：上限是延迟预算和风暴风险的折中。2 次重试 = 最坏 3 次请求、约 1.5s 额外延迟（SDK 参数）；每加一次，尾延迟线性涨、恢复瞬间的请求量指数涨。openai issue #1102（开放 2 年+）就是用户嫌默认不可调——所以正确答案不是"更多"，是"**可调 + 与 Retry-After/截止时间联动**"（go-agent 的 ctx 截止闸门就是联动：截止先到就停，不管还剩几次重试）。

## 13. 面试三层回答（SPEC §5.5，每层挂证据链）

### 🕐 30 秒版

> "重试我按四步做：**先分类**——408/409/429/5xx 可重试，其他 4xx 和协议错不重试（参数错要反馈给模型改，不是重试）；**指数退避 + 抖动 + 上限**——主流 SDK 默认 0.5 秒起、8 秒封顶、最多 2 次，AWS 2015 的仿真证明无抖动是 'clear loser'；**尊重服务器的 Retry-After**——openai SDK 遇到要等超过 120 秒的直接放弃重试；**幂等是前提**——user 消息只入历史一次，失败整轮回滚。两个硬边界：流式**已发出内容就不重试**，防止重复输出；重试**只放一层**，避免 SDK 和应用层叠加成 9 次请求。"

证据链：RFC 6585 §4 / RFC 9110 §9.2.2、§10.2.3 ｜ openai·anthropic SDK 源码（[`_constants.py`](https://github.com/openai/openai-python/blob/main/src/openai/_constants.py) + [`_base_client.py`](https://github.com/openai/openai-python/blob/main/src/openai/_base_client.py)）｜ AWS 博客 2015 ｜ go-agent [`agent.go`](../../experiments/go-agent/agent/agent.go) L181,251

### 🕒 3 分钟版

**① 形态决定失败语义（30s）**：同步失败立刻可重试；异步拿 202 之后要轮询（RFC 9110 §15.3.3：HTTP 没有办法重发异步操作的状态码）；流式有"已发不可撤回"边界；并行是部分失败（`parallel_tool_calls` 默认 true，结果按 `tool_call_id` 回填，每个 tool_call 都要有交代）。

**② 判定：服务器指令 > 分类 > 默认（40s）**：`x-should-retry` 头优先，其次 Retry-After 上限检查，最后状态码白名单 {408,409,429,≥500}——这是 openai 和 anthropic 两家 SDK 源码里逐字相同的结构。分类的关键不是状态码本身而是语义：429 的计数方式 RFC 明说不定义，所以"429 就重试"是窗口型限流假设，配额型 429 重试无效，要靠错误体区分。

**③ 退避 + 抖动（40s）**：公式 `min(0.5·2ⁿ, 8s) × (1−0.25·rand)`，两家 SDK 同款；AWS 2015 对照四种策略，无抖动指数退避是 clear loser，Full Jitter 工作量最少，结论是抖动应该是远程客户端的标准做法。**我自己的实现有可证伪的测量**：go-agent 固定 200ms→400ms，N=20 实测 p50=p95=600ms 零方差——这就是无抖动的代价，单客户端测不出来，并发下会对齐。

**④ Retry-After（30s）**：规范两种形式（HTTP-date / 秒数）；openai 服从但上限 120 秒、超了放弃；anthropic 无条件服从（注释原话"服务器让等就等"）；实用解析器只认数字，HTTP-date 会静默回退本地退避。

**⑤ 幂等 + 两层防线（30s）**：RFC 9110：非幂等方法 SHOULD NOT 自动重试——LLM 调用是 POST，能重试是因为副作用只在计费层；带外部副作用的写操作必须幂等键（SDK 重试自动带 `stainless-python-retry-{uuid}`）。状态层：失败整轮回滚到 base，不留半截状态；观测层：`attempts` 进结构化日志，重试可断言（测试里断言 429×2 后 attempts=3）。

**⑥ 收尾（10s）**：一句话——"**分类给重试发许可证，退避和抖动控制重试的代价，Retry-After 是服务器的否决权，幂等是重试的安全带，上限和截止时间是最后的刹车。**"

证据链：全部逐条对应 §9 矩阵 16 行（14 项 ✅）。

### 🌊 追问深水区版

按 §12 红队 Q1-Q7 展开，每问三段式：**先给判据（挂原文）→ 承认自己实现的边界 → 给改进路径**。重点备三张牌：

1. **"你的实现没抖动"** → 零方差实测自证 + AWS 结论 + Full Jitter 改进项 + pending 变体实验（不掩饰，把弱点变成"我有测量"）；
2. **"双层重试叠加"** → (2+1)²=9 的算术 + 单层原则 + go-agent [`api.go`](../../experiments/go-agent/agent/api.go) 零重试的代码证据；
3. **"429 双语义"** → RFC 6585 "does not define how the origin server identifies the user, nor how it counts requests" 原文引用 + Retry-After 量级/错误体的区分判据 + SDK 乐观假设的承认。

---

## 关联文档

- [函数调用原理](./function-calling.md) — §6 错误处理的分类表与"失败是 Observation"原则（本文的重试语义是它的深水区展开）
- [认知架构范式](../agent-architecture/cognitive-architecture-paradigms.md) — ReAct 的 Action 失败即观察，模型在 loop 内保留决策权
- [SSE 流式断线恢复案例](../../patterns/case-study-bondseek-sse-recovery.md) — `Last-Event-ID` 水位线 + 回放去重的生产形态（转述，待核 #1）
- **一手代码:** [`experiments/go-agent/agent/agent.go`](../../experiments/go-agent/agent/agent.go)（重试循环）｜[`fault_test.go`](../../experiments/go-agent/agent/fault_test.go)（故障注入）｜[`types.go`](../../experiments/go-agent/agent/types.go)（错误分类）
- **外部原文:** [RFC 9110](https://www.rfc-editor.org/rfc/rfc9110.txt) ｜ [RFC 6585](https://www.rfc-editor.org/rfc/rfc6585.txt) ｜ [WHATWG SSE](https://html.spec.whatwg.org/multipage/server-sent-events.html) ｜ [AWS Exponential Backoff And Jitter (2015)](https://aws.amazon.com/blogs/architecture/exponential-backoff-and-jitter/) ｜ [openai-python](https://github.com/openai/openai-python) ｜ [anthropic-sdk-python](https://github.com/anthropics/anthropic-sdk-python) ｜ [openai-openapi](https://github.com/openai/openai-openapi)

> **最后验证日期:** 2026-11-04（全部 ✅ 条目打开原文核对；⏳/pending 项见 §9、§11）
