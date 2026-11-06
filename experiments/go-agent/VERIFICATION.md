# Go Agent 验证报告（SPEC v1.0 对照）

**日期:** 2026-11-03
**对照题目:** [`SPEC.md`](SPEC.md) v1.1（含退出内网范围变更）
**环境:** Windows / go1.26.5 / 无 gcc → `-race` 待 CI；无网（`E2E_REAL` 门禁未开）→ 全部验证离线完成
**复现:** `cd experiments/go-agent && go test -count=1 -timeout 120s ./...`（任何人可重跑）

---

## 0. 总览

| 证据 | 结果 | 来源 |
|---|---|---|
| 编译 + vet | ✅ 零告警 | `go build ./... && go vet ./...` |
| 单元/集成测试 | ✅ 38 项全绿（约 9s） | `go test -count=1 ./...` |
| 覆盖率 | 93.2%（agent 包） | `go test -cover ./...` |
| SSE 解析器 fuzz | ✅ 30s / 1,185,206 次执行 / 0 crash | `go test -fuzz FuzzParseSSE -fuzztime 30s` |
| 变异测试 | ✅ 2/2 注入变异被测试杀死 | 见第 5 节 |
| 仓库密钥 | ✅ 清零（grep 机械证明） | 两个已知密钥（前缀 `NnVk…` / `N9r_…`）在全仓库 grep 无匹配 |
| `-race` | ⚠️ 本机无 gcc，无法运行（见第 7 节环境限制） | — |

> 说明：本报告数字为**本次运行实测**；重跑应在相同命令下得到等价结果（时间类数字可有少量浮动）。

---

## 1. 轮次状态机（D1）

**不变量**（`assertInvariant`，每轮结束后断言）：

- I1 idle：`messages = [system] + (user, assistant)*` 严格成对交替
- I2 pending：I1 前缀 + 恰好一条尾部 user

**状态收敛语义**（`runTurn`）：

| 失败路径 | 结局 | 历史 |
|---|---|---|
| 网络/超时/5xx/协议错误 | `rolled_back` | 回滚到轮次前长度（不留孤儿 user，半截内容不入库） |
| ctx 取消/截止 | `aborted` | 同上回滚 |
| `finish_reason=length` | `committed` + `Truncated=true` | 成对入库，截断对调用方**可见** |
| 预算拒绝 | 未发出请求 | 历史不动 |

**机械证明**：

- `TestPropertyRandomInterleavings`：固定种子（42）随机 100 轮 成功/500/取消 交错，每轮后断言不变量 + 长度增量（成功 +2，其余 +0）✅
- `TestRollbackOnServerError` / `TestCancelMidStreamAborted` / `TestMidStreamDisconnectRollback` 等 ✅
- 变异测试：回滚逻辑注入 off-by-one 变异 → 5 个测试失败（被杀）✅

---

## 2. 协议符合性矩阵（D2）

判据：WHATWG EventSource 规范 + OpenAI Chat Completions 行为。

| 规范条款 | 实现位置 | 测试 | 结果 |
|---|---|---|---|
| `data:` 后可选一个空格（`data:x` 合法） | sse.go `strings.TrimPrefix(value, " ")` | TestSSEConformance/data 后无空格、Replay no_space.txt | ✅ |
| `data:` 后只剥离一个空格 | 同上 | TestSSEConformance/只剥离一个空格 | ✅ |
| CRLF 行终止 | sse.go `readSSeLine`（bufio.ReadLine 语义） | TestSSEConformance/CRLF、Replay CRLF 变体 | ✅ |
| 同一事件多条 data 行以 `\n` 连接 | sse.go flush `strings.Join` | TestSSEConformance/多条 data、Replay multiline.txt | ✅ |
| `:` 注释行忽略 | sse.go 注释分支 | TestSSEConformance/注释行、Replay comments.txt | ✅ |
| 无 data 行的事件不派发 | sse.go flush 空缓冲检查 | TestSSEConformance/无 data 行的事件 | ✅ |
| 空 data 字段派发空字符串 | 同上 | TestSSEConformance/空 data 字段 | ✅ |
| event/id/retry 字段忽略 | sse.go switch | TestSSEConformance/字段忽略 | ✅ |
| 未知字段忽略 | sse.go default | TestSSEConformance/未知字段 | ✅ |
| EOF 丢弃未派发残留 | sse.go EOF 分支 | TestSSEConformance/EOF 丢弃 | ✅ |
| 超长行显式错误（替代 v0 的 64KB 静默死流） | sse.go `errSSELineTooLong` | TestFaultLineTooLong（5MB 行） | ✅ |
| 100KB 行长行正常解析 | 同上 | TestSSEReadLineBoundaries | ✅ |
| 流内 error 对象显式上报（v0 静默跳过） | api.go `chunk.Error` | TestFaultStreamErrorChunk、Replay error_chunk.txt | ✅ |
| 畸形 JSON 显式上报（v0 `continue` 吞掉） | api.go ClassProtocol | TestRollbackOnMalformedJSON | ✅ |
| `finish_reason=length` 可见（v0 不区分） | agent.go `Truncated` | TestTruncatedFinishLengthVisible | ✅ |
| 无 finish/[DONE] 即 EOF → 流截断错误 | api.go EOF 检查 | TestFaultTruncatedStream、Replay truncated_no_done.txt | ✅ |
| `[DONE]` 约定识别 | api.go `errStreamDone` | Replay basic.txt | ✅ |
| **范围外（明确声明）**：单独 `\r` 行终止、event/id/retry 语义 | — | — | 声明 |
| **待核**：真实 SSE 字节格式（data 空格、CRLF、error chunk 形状）、`stream_options.include_usage` | — | 待对目标服务端抓包 | ⏳ |
| **范围变更**：`chatId`/`detail` 网关私有字段已移除 | types.go 仅保留标准字段 | TestRequestOnlyStandardFields（原始 body 断言无 `chatId`/`detail`） | ✅ |

---

## 3. 取消、超时、资源（D3）

| 项 | 机制 | 测试 | 结果 |
|---|---|---|---|
| 连接超时 | Transport DialContext 5s | — | ✅（实现） |
| 首字节超时 | ResponseHeaderTimeout 30s | TestHeaderTimeout（mock 挂住响应头，100ms 注入） | ✅ |
| 流空闲超时 | 空闲看门狗（每 idle/4 轮询，默认 60s） | TestIdleTimeout（150ms 注入，0.15s 中断） | ✅ |
| 总时长/取消 | ctx 贯穿请求，Ctrl-C → aborted | TestCancelNonStreamAborted、TestCancelMidStreamAborted | ✅ |
| goroutine 泄漏 | 取消路径泄漏检测（预热基线 + 5 轮取消后收敛计数） | TestNoGoroutineLeak | ✅ |
| v0 的 channel 版本缺陷（消费方退出→生产者永久阻塞） | **消除**：改为回调式 `emit`，生产者随调用栈返回 | —（结构保证） | ✅ |

---

## 4. 上下文预算与 token（D4）

- 服务端 `usage` 回填为真值：`TurnResult.Usage`、`GetStats().Usage`（非流式实测回填，流式留 `stream_options` 待核）
- 估算与真值并列展示：`/stats` 同时打印"估算 Token"与"服务端真值"
- 预算预检：`est(历史+输入) + max_tokens > budget` → 显式 `ClassBudget` 拒绝，请求不发出、历史不动（`TestBudgetRejected` 断言 mock 零调用）
- 默认 `budget=0`（不限制）→ 显式声明于 CLI 输出与 `/stats`，不是静默无限
- **待核**：估算 vs 真值的误差分布（需真实网关 N≥20 轮对拍）

---

## 5. 故障矩阵（D7）

| 注入故障 | 期望类别 | 测试 | 结果 |
|---|---|---|---|
| 连接拒绝 | network + 回滚 | TestFaultConnectionRefused | ✅ |
| 429 限流 ×2 → 成功 | 退避重试后 committed，attempts=3 | TestFaultRateLimitThenSuccess | ✅ |
| 429/5xx 重试幂等 | 3 次请求载荷逐字节一致（user 只入历史一次） | 同上（captured 快照断言） | ✅ |
| HTTP 400 | client + 不重试（attempts=1） | TestFaultClientErrorNoRetry | ✅ |
| HTTP 500 | server + 3 次重试后回滚 | TestRollbackOnServerError | ✅ |
| 畸形 JSON | protocol + 回滚 | TestRollbackOnMalformedJSON | ✅ |
| 空 choices | empty + 回滚 | TestEmptyChoices | ✅ |
| 流中途断连 | rolled_back，半截内容不入库 | TestMidStreamDisconnectRollback | ✅ |
| 流内 error 对象 | server + 回滚 | TestFaultStreamErrorChunk | ✅ |
| 流输出后断连 | **不重试**（用户已见内容） | TestFaultStreamNotRetriedAfterEmit | ✅ |
| 超长行（5MB） | protocol 显式错误 | TestFaultLineTooLong | ✅ |
| ctx 截止 | 退避立即停止（<500ms） | TestFaultRetriesStopAfterContextDeadline | ✅ |

## 变异测试（验证测试有效性）

| 变异 | 杀死测试数 | 杀死者 |
|---|---|---|
| 回滚 off-by-one（`[:base]` → `[:base+1]`，孤儿 user 残留） | 5 | TestRollbackOnServerError、TestRollbackOnMalformedJSON、TestCancelMidStreamAborted、TestMidStreamDisconnectRollback、TestPropertyRandomInterleavings |
| 解析器退回 v0 前缀式（只认 `"data: "` 带空格） | 2 | TestSSEConformance/data 后无空格、TestReplayFixtures/data 后无空格 |

> 结论：注入的 2 个真实类变异均被测试杀死。变异 2 的杀死者只有 no_space 用例——说明 CRLF/多行用例对该变异**不敏感**（该变异不改变这些路径的行为），这是诚实结论而非缺陷。

---

## 6. 并发（D5）

- `TestConcurrentTurnsSerialized`：6 个并发轮次，服务端收到的第 k 个请求恰好携带 2k+2 条消息（sys + 2k 成对历史 + 当前 user）——串行化的机械证明 ✅
- `TestConcurrentTurnsAndReads` / `TestConcurrentStreamAndChat`：读写并发结构安全 ✅
- ⚠️ `-race` 未运行（环境限制，见下）

---

## 7. 环境限制与待核清单更新

### 环境限制（本机无法完成，非实现缺陷）

| 项 | 原因 | 补齐动作 |
|---|---|---|
| `go test -race` | 本机无 gcc（Windows race detector 需 cgo） | CI 或有 gcc 的机器上跑 |
| 真实端点抓包/测量/usage 对拍 | `E2E_REAL=1` 门禁未开（需 API_KEY + MODEL_NAME） | 有网环境执行 |
| openai-go 差分对拍 | 需下载第三方依赖（零依赖约束仅限运行时；此处为网络限制） | 有网环境执行 |

### SPEC 第 9 节待核清单状态

| # | 项 | 状态 |
|---|---|---|
| 1 | 流式 usage（include_usage） | ⏳ 待核（解析器已实现，需目标服务端确认） |
| 2 | 真实 SSE 字节格式 | ⏳ 待核（解析器按规范子集实现 + fuzz + fixture；需抓包对照） |
| 3 | 服务端 429/500/超时返回体 | ⏳ 待核（错误分类已按状态码实现；体格式不影响分类） |
| 4 | 模型 tokenizer 离线可用性 | ⏳ 待核（现以 usage 为真值来源） |
| 5 | max_tokens 触发 length | ✅ 已验证机制（mock 注入 length 可见）；真实触发待 e2e |
| 6 | 超长行服务端行为 | ✅ 已验证客户端机制（4MB 上限 + 5MB 注入行显式报错） |
| 7 | chatId/detail 字段出处 | ✅ **已解决**：退出内网范围，私有字段移除（`TestRequestOnlyStandardFields`） |

---

## 8. 测量（D6 离线部分）

- 结构化日志（stderr，slog）：`turn_id / outcome / err_class / ttft_ms / duration_ms / attempts / finish_reason / truncated / prompt_tokens / completion_tokens / history_len` —— 密钥绝不入日志（API 密钥仅用于请求头）
- mock 测量 N=20（`TestMeasurementMock`）：TTFT 0–11ms、总时长 0–55ms（**仅流程开销，非真实模型延迟**；真实网关 N≥20 待核）
- 观察点：测试日志中 `history_len` 逐轮 +2，证实历史成对增长（见测试输出）

---

## 9. 结构变更（相对 v0）

| 文件 | 变化 |
|---|---|
| `agent/types.go` | +TurnState/TurnOutcome/TurnResult、错误分类（9 类）、`ClassifiedError`、`StreamChunk.Error/Usage`；**移除 `chatId`/`detail` 私有字段**（退出内网范围） |
| `agent/sse.go` | **新增**：规范子集 SSE 解析器（可 fuzz） |
| `agent/api.go` | 重写：ctx 贯穿、超时分级、空闲看门狗、错误分类、流截断检测 |
| `agent/agent.go` | 重写：轮次状态机 + 回滚、轮次信号量串行化、预算预检、usage 回填、slog；**移除 `chatID` 概念**（`ResetChatID` 随之删除） |
| `config/config.go` | **移除硬编码密钥与内网默认**：`API_KEY`/`MODEL_NAME` 必填，`ENDPOINT` 为完整 URL（默认公网 OpenAI 端点）；移除 `CHAT_ID`；+`CONTEXT_BUDGET_TOKENS` |
| `main.go` | ctx/signal、回调式流式、错误类别展示、截断警告、token 真值展示 |
| `agent/*_test.go` | 重写/新增：mock 基础设施、状态机、故障矩阵、SSE 符合性、fuzz、超时、并发、重放、测量、字段面断言 |
| `agent/testdata/sse/*.txt` | **新增**：6 个固化 SSE 字节 fixture（离线线缆证据） |
| `agent/e2e_real_test.go` | 真实端点门禁 e2e（`E2E_REAL=1`，无内网默认） |
| `test-curl.sh`、`resources/api-curl-commands.md` | 密钥移除（SPEC 安全修正） |
| v0 的 `ResetFChatID` 拼写 bug | 随 `chatID` 概念一并移除（v0 测试因该拼写根本无法编译——`ResetChatID undefined`，即 v0 验证体系本身是坏的） |

---

## 10. 变更记录

| 日期 | 说明 |
|---|---|
| 2026-11-03 | 首版：SPEC v1.0 的离线验证全量交付（38 测试 / fuzz / 变异 2 项 / 覆盖 93.2%）；内网抓包、race、差分对拍标记待核 |
