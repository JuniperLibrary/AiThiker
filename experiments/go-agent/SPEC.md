# 多轮对话 Go Agent —— 深度规格（题目）

**版本:** v1.1（题目重写 + 范围变更）
**日期:** 2026-11-03
**状态:** 已实现（离线验证交付，见 [`VERIFICATION.md`](VERIFICATION.md)）；内网抓包/-race/差分对拍待核
**前置文档:** `../go-multi-turn-agent-design.md`（v0 现状记录）、本文件为新题目，后续实现以本文件为准
**实验路径:** `experiments/go-agent/`

---

## 0. 一句话题目

把"能跑的多轮对话 CLI"升级为**失败语义明确、协议字节级符合、可被第三方独立复核**的多轮对话 Agent 内核：任何一轮对话在任何失败路径下都满足历史不变量，任何一行收到的字节都能追溯到规范条款，任何结论都能被机器（而非作者）证明。

> **深度优先，不是规模优先。** 本题目明确拒绝"更大"（见第 3 节非目标），只做"更深"。

---

## 1. 背景与现状差距

现有 v0 实现已具备：CLI 交互、流式/非流式双模式、RWMutex 消息历史、零依赖编译、9 个单元测试、一次手工 3 轮 e2e。

但按本领域专家标准，存在以下**结构性差距**（本题目要解决的正是这些）：

| # | 差距 | 现状证据（代码位置） | 专家标准 |
|---|---|---|---|
| 1 | **失败时历史被污染** | `agent.go:66-83` 先 append user 再请求，失败后孤儿 user 消息留在历史，下一轮发出连续两条 user；`agent.go:133-136` 流中断时把半截回复当完整回复入库 | 轮次状态机：pending → committed / rolled back，任何失败路径下历史不变量 `history = [system?] + (user, assistant)*` 成对成立，用测试证明 |
| 2 | **SSE 解析器不符规范** | `api.go:112` 只认 `"data: "` 前缀（规范允许 `data:` 无空格），忽略注释行/多行 data；`api.go:125` 畸形 JSON 静默跳过=掩盖故障；`api.go:105` `bufio.Scanner` 默认 64KB 行上限，超长行整个流死掉；`api.go:132` 不区分 `finish_reason=length`（max_tokens 截断不可见） | 按 WHATWG SSE 规范逐条实现并建立符合性矩阵；解析错误必须显式上报而非丢弃 |
| 3 | **无取消、无超时** | `api.go:26` `http.Client{}` 无超时；全代码无 `context.Context` | 超时分级（连接/首字节/空闲/总时长），context 全链路贯穿，Ctrl-C 可中断在途请求且状态收敛 |
| 4 | **轮次无原子性** | 锁只保护切片读写，不保护"一轮对话"；并发 `Chat` 会交错成 user1,user2,assistant1,assistant2 | 会话级轮次串行化，并发下配对不变量仍成立（`-race` + 并发 property test 证明） |
| 5 | **token 是猜的** | `agent.go:201-207` 用 rune 数/2 估算；`types.go:40` 定义了 `Usage` 却从未回填；历史无上限增长，溢出行为未定义 | 以服务端 `usage` 为真值对拍估算误差；定义上下文预算与溢出语义（超限必须显式报错或截断，禁止静默发超长请求） |
| 6 | **无测量、无观测** | 验证只有一次手工 3 轮 ✅；无 TTFT/tokens/s/p95；`config.go:39` API 密钥硬编码进仓库 | 结构化日志（turn_id、TTFT、finish_reason、tokens）、N 次重复测量出分布、密钥移出代码并脱敏 |
| 7 | **验证全走正路** | 测试全是纯逻辑正路测试；无 mock server、无故障注入、未跑 `-race`/`-fuzz`、e2e 不可重跑 | 故障注入 + fuzz + 变异测试 + 差分测试 + 抓包 fixture 重放，套件可被第三方在干净机器重跑 |
| 8 | **无重试语义** | `api.go:50-53` 非 200 一次性报错，无退避、不区分可重试/不可重试 | 错误分类（连接错误/429/5xx 可重试，4xx 不可重试），重试必须幂等（禁止重复追加 user 消息） |

---

## 2. 目标（七个深度轴）

- **D1 轮次状态机与失败原子性**：定义并证明历史不变量；失败、截断、取消、超时各自有明确的状态收敛语义。
- **D2 协议字节级符合性**：SSE 与 Chat Completions 行为对照规范原文逐条验证，建立"条款 → 测试 → 结果"符合性矩阵。
- **D3 取消、超时、资源回收**：context 贯穿全链路；超时分级；无 goroutine/连接泄漏（goleak 证明）。
- **D4 上下文预算与 token 真值**：usage 对拍估算误差；溢出语义显式化；每轮成本可测量。
- **D5 会话并发时序**：轮次原子性在并发下成立，`-race` 干净。
- **D6 测量与可观测**：TTFT、tokens/s、p50/p95 延迟、结构化日志、密钥脱敏。
- **D7 对抗性验证**：故障注入、fuzz、变异测试、差分测试、抓包重放——验证体系本身的有效性可被证明。

---

## 3. 非目标（明确排除，写死不可扩展）

以下都是"更大"而非"更深"，本题目**一律不做**：

- ❌ Tool Calling / 函数调用
- ❌ 多会话管理、对话持久化（文件/数据库）
- ❌ RAG、外部记忆、历史摘要压缩的"产品化"
- ❌ Web UI / TUI 框架 / 桌面客户端
- ❌ 多 Agent 协作、子任务规划
- ❌ 引入任何第三方运行时依赖（保持零依赖约束；工具链除外）
- ❌ 通用框架抽象（本项目是实验内核，不是 SDK）

> 判断标准：**新代码必须服务于"更深"的七个轴之一；只增加功能面的一律拒绝。**

---

## 4. 原始素材清单与获取方法

| 类别 | 原始素材 | 来源 | 用途 |
|---|---|---|---|
| API 规范 | Chat Completions 官方参考原文（错误对象、finish_reason 全集、stream_options） | platform.openai.com/docs | 请求/响应符合性判据 |
| 流式规范 | WHATWG SSE 规范原文（`data:` 语法、多行、CRLF、注释、event/id/retry） | html.spec.whatwg.org | SSE 解析器符合性判据 |
| 服务端真值 | OpenAI 兼容服务端源码（vLLM / llama.cpp / 各类推理服务，按实际部署选择） | 各项目源码 | 服务端实际行为的 ground truth |
| 线缆证据 | `curl -v --raw` / tcpdump 抓取的真实 SSE 原始字节 | 实际使用的服务端 | record & replay fixture，不经转述 |
| 语言语义 | Go 标准库源码与文档（`bufio.MaxScanTokenSize`、`net/http` 超时、`context`、`sync`、Go memory model） | go.dev | 并发/超时结论的依据 |
| token 真值 | API 响应 `usage` 字段 + 模型 tokenizer | API 本身 | 对拍本地估算误差 |
| 故障语料 | openai-python / openai-go 的 GitHub issues、流式中断事故报告 | GitHub | 已知故障模式清单 |
| 研究依据 | MT-Bench 多轮部分、*Lost in the Middle* 等多轮上下文论文原文 | 论文 | 上下文增长对质量影响的依据 |
| 自身记录 | 本实验完整原始对话 transcript（raw，非摘要表格） | 本地会话 | 失败细节回溯 |

**禁止**：以设计文档的转述、示例格式、个人印象代替规范原文与抓包字节。

**范围变更（2026-11-03）**：本项目**不再支持内网网关**——网关私有字段（`chatId`/`detail`）、内网默认地址与模型名一并移除，只保留 OpenAI 标准协议面。

---

## 5. 深度工作项与验收

### 5.1 轮次状态机（D1）

- 状态：`idle → pending → committed | rolled_back | aborted`
- 不变量 I1：任意时刻 `history = [system?] + (user, assistant)*` 成对
- 不变量 I2：失败轮次不产生孤儿消息（回滚）或显式标记（aborted），二者语义写入文档并测试
- 流中断：部分回复不得伪装为完整回复——回滚或标记 `truncated`
- 验收：state machine 文档 + property test（随机交错成功/失败/取消序列，断言 I1/I2 恒成立）

### 5.2 协议符合性（D2）

- SSE：`data:` 无空格、CRLF、多行 data、注释行、超长行、`[DONE]`、流内 error 对象
- 响应：`finish_reason=length` 必须对用户可见；畸形 JSON 必须显式上报
- 符合性矩阵：规范条款 → 实现位置 → 测试用例 → 通过结果
- 验收：矩阵无空白行（每条可核对条款都有对应测试）

### 5.3 取消与超时（D3）

- `context.Context` 贯穿 Chat/ChatStream/HTTP；超时分级（连接/首字节/空闲/总）
- 取消后：在途请求中断、goroutine 退出、历史状态收敛（回到 I1）
- 验收：goleak 零泄漏 + 取消时序测试

### 5.4 上下文预算（D4）

- `usage` 回填为真值；`GetStats` 估算值与其对拍，报告误差分布
- 溢出语义：超出预算必须显式报错或按声明策略截断，禁止静默超限
- 验收：token 对拍报告（估算 vs 真值，N≥20 轮）+ 溢出边界测试

### 5.5 并发轮次原子性（D5）

- 会话内轮次串行化（拒绝/排队策略显式声明）
- 验收：并发 property test + `-race` 干净 + 交错配对断言

### 5.6 可观测与安全（D6）

- 结构化日志：turn_id、TTFT、tokens、finish_reason、错误类别；密钥脱敏
- 密钥移出仓库（环境变量必填，无硬编码默认值）
- 验收：N≥20 次运行的 TTFT/tokens/s/p50/p95 报告；仓库无密钥（grep 证明）

### 5.7 对抗性验证（D7）

- 故障注入矩阵（httptest mock server）：连接拒绝 / 中途断流 / 429 / 500 / 慢响应 / 畸形 JSON / 超长行 / 空 choices
- SSE 解析器 fuzz：`go test -fuzz` 跑指定时长，seed corpus 入库
- 变异测试：注入已知 bug（回滚逻辑、前缀判断、超时），测试必须失败
- 差分测试：同请求对拍官方 `openai-go` SDK 行为
- record & replay：抓包字节固化为 fixture，CI 中离线重放
- 验收：故障矩阵全绿 + fuzz 无 crash + 变异得分报告 + 差分报告

---

## 6. 方法与工具

| 类别 | 工具 |
|---|---|
| 测试 | `net/http/httptest`、`go test -race / -fuzz / -bench / -cover`、golden file、property test |
| 静态 | `go vet`、`staticcheck`、goleak |
| 性能 | `pprof`、bench、TTFT/tokens/s/p95 统计 |
| 线缆 | `curl -v --raw`、tcpdump/Wireshark、`jq` |
| 差分 | 官方 `openai-go` SDK（仅作测试对拍对象，不进运行时依赖） |
| 复现 | 干净环境一条命令跑完整套件；fixture 版本锁定 |

方法原则：规范驱动 → 先测后改 → 测量说话 → N 次重复取分布（禁止单次 ✅ 作为结论）。

---

## 7. 验证体系（除作者之外的正确性证明）

1. **机器裁决**：编译器、`-race`、fuzzer 输出——不依赖作者判断
2. **线缆证据**：抓包原始字节入库，任何人可重新检视，不经转述
3. **规范原文**：SSE / OpenAI API 文档作为符合性判据，逐条对照矩阵
4. **上游真值**：vLLM 源码与服务端日志
5. **差分对拍**：官方 `openai-go` SDK 同请求同响应行为
6. **第三方重跑**：干净机器一条命令，结果一致才算数
7. **变异得分**：注入已知 bug，测试必须失败——证明测试有效
8. **独立评审**：另一人/模型按本 SPEC 逐条验收，不看作者解释
9. **统计证据**：N 次运行分布 + token 对拍误差，非单次轶事

---

## 8. 交付物与验收标准

| 交付物 | 验收标准 |
|---|---|
| 本 SPEC | 与实现一致，变更可追溯 |
| 状态机文档 + property test | 不变量 I1/I2 在随机失败序列下恒成立 |
| 符合性矩阵 | 每条可核对条款 → 测试 → 结果，无空白 |
| 故障注入套件 | 故障矩阵场景全绿，`-race` 干净 |
| fuzz corpus | 解析器 fuzz 指定时长零 crash，seed 入库 |
| 变异得分报告 | 注入 bug 被测试捕获的比率 |
| 抓包 fixture 库 | 原始字节 + 重放测试，CI 离线可跑 |
| 测量报告 | N≥20 次：TTFT/tokens/s/p50/p95 + token 估算误差 |
| 差分报告 | 与 openai-go 行为对照记录（含不一致项及解释） |
| 安全修正 | 仓库无密钥、超时生效、日志脱敏（grep + 测试证明） |

---

## 9. 待核清单（禁止凭记忆填写，核实后才可写入结论）

1. 目标 OpenAI 兼容服务端是否支持 `stream_options.include_usage`（流式 usage 的获取方式）
2. 真实 SSE 字节格式：`data:` 后是否有空格、行尾 CRLF/LF、`[DONE]` 前后空行、流内 error chunk 的确切形状（**必须对目标服务端抓包确认**）
3. 目标服务端 429/500/超时的真实返回体格式
4. 所用模型 tokenizer 能否离线获取（token 真值的本地来源；否则以 usage 为唯一真值）
5. `max_tokens` 是否实际触发 `finish_reason=length`（构造长回答验证）
6. 单行超长时服务端的实际行为（构造超长行验证）

> ~~7. 网关 `chatId`/`detail` 字段出处~~ → **已解决**：退出内网范围，私有字段已从请求体移除（`TestRequestOnlyStandardFields` 断言）。

> 以上任一项未核实前，相关结论标记 `pending_verification`，不进入符合性矩阵的"通过"列。

---

## 10. 与 v0 的关系

- v0 代码（`main.go`、`agent/`、`config/`）与设计文档 `../go-multi-turn-agent-design.md` **保留**，作为基线
- 本题目要求的增量全部在七个深度轴上：失败原子性、协议符合、取消超时、token 真值、并发原子性、可观测、对抗验证
- 迁移策略：先建状态机与故障注入套件（5.1、5.7），再逐项修复差距表中的 8 条

---

## 11. 变更记录

| 日期 | 版本 | 说明 |
|---|---|---|
| 2026-11-03 | v1.0 | 题目重写：深度优先非规模优先；8 条结构性差距（附代码位置）、七深度轴、非目标写死、原始素材清单、方法工具、九项外部验证、待核清单 7 项 |
| 2026-11-03 | v1.0 | 实现交付：38 项测试全绿（9s）、SSE fuzz 30s/118 万次 0 crash、变异 2/2 被杀、覆盖 93.2%、密钥清零；详见 VERIFICATION.md |
| 2026-11-03 | v1.1 | 范围变更：退出内网支持——移除 chatId/detail 私有字段、内网默认地址/模型、chatID 概念；配置改为 ENDPOINT 完整 URL + MODEL_NAME 必填 |
