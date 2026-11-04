// AICode Begin
package agent

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// 轮次重试策略：仅对可重试类别（network/timeout/server/rate_limit）重试，
// 且流式一旦已有内容发出即不重试（避免向用户重复输出）。
// 使用 var 而非 const：测试可注入更小的退避以加速（见 TestPropertyRandomInterleavings）。
var (
	maxRetries     = 2
	retryBaseDelay = 200 * time.Millisecond
)

const defaultSystemPrompt = "你是一个智能 AI 助手，请用中文回答用户的问题。回答简洁准确。"

// Options 是创建 Agent 的选项。
type Options struct {
	BaseURL   string        // 完整端点 URL（含路径）
	APIKey    string        // 必填：不再内置默认密钥
	Model     string
	MaxTokens int
	Budget    int           // 上下文预算（prompt+completion 上限）；0 = 不限制（显式声明）
	Logger    *slog.Logger  // nil → slog.Default()
}

// Agent 是多轮对话 Agent 的核心：轮次状态机 + 上下文管理 + 重试 + 可观测。
//
// 不变量（由 runTurn 维护，state_test.go 断言）：
//   - I1 idle：messages = [system] + (user, assistant)* 严格成对交替
//   - I2 pending：I1 的前缀 + 恰好一条尾部 user（本轮在途消息）
//   - 任何失败/取消路径：回滚到轮次开始前的长度（base），不留孤儿消息
//
// 并发模型（D5）：turnSem 容量 1，同一时刻至多一轮在途（排队策略）。
// GetHistory/GetStats/State 只读，可在轮次中并发调用。
// ClearHistory/SetSystemMessage 也串行于轮次之后——但禁止在 emit 回调内调用（会死锁）。
type Agent struct {
	client    *APIClient
	model     string
	maxTokens int
	budget    int
	logger    *slog.Logger

	mu       sync.RWMutex
	messages []Message
	state    TurnState
	seq      int
	usage    *Usage // 最近一次服务端 token 真值

	turnSem chan struct{} // 容量 1：轮次串行化
}

// NewAgent 创建 Agent 实例。
func NewAgent(opt Options) *Agent {
	lg := opt.Logger
	if lg == nil {
		lg = slog.Default()
	}
	return &Agent{
		client:    NewAPIClient(opt.BaseURL, opt.APIKey),
		model:     opt.Model,
		maxTokens: opt.MaxTokens,
		budget:    opt.Budget,
		logger:    lg,
		messages:  []Message{NewSystemMessage(defaultSystemPrompt)},
		state:     StateIdle,
		turnSem:   make(chan struct{}, 1),
	}
}

// Chat 发送一条用户消息并获取完整回复（非流式）。
func (a *Agent) Chat(ctx context.Context, input string) (TurnResult, error) {
	return a.runTurn(ctx, input, nil)
}

// ChatStream 发送一条用户消息，每个内容块回调 emit。
// emit 返回错误则中止本轮（历史回滚）。
func (a *Agent) ChatStream(ctx context.Context, input string, emit func(string) error) (TurnResult, error) {
	if emit == nil {
		emit = func(string) error { return nil }
	}
	return a.runTurn(ctx, input, emit)
}

// runTurn 执行一轮对话：pending → committed | rolled_back | aborted。
func (a *Agent) runTurn(ctx context.Context, input string, emit func(string) error) (TurnResult, error) {
	turnStart := time.Now()

	// 轮次令牌：排队等待；等待期间 ctx 取消则未开始即中止
	select {
	case a.turnSem <- struct{}{}:
		defer func() { <-a.turnSem }()
	case <-ctx.Done():
		return TurnResult{
			Outcome:    OutcomeAborted,
			ErrClass:   ClassOf(ctx.Err()),
			DurationMs: msSince(turnStart),
		}, ctx.Err()
	}

	// 上下文预算预检（D4）：未通过则不动历史
	if err := a.checkBudget(input); err != nil {
		return TurnResult{
			Outcome:    OutcomeRolledBack,
			ErrClass:   ClassBudget,
			DurationMs: msSince(turnStart),
		}, err
	}

	// pending：user 入历史，快照请求载荷，记录回滚点 base
	a.mu.Lock()
	a.seq++
	turnID := fmt.Sprintf("t%03d", a.seq)
	base := len(a.messages) // 回滚点（user 入历史前的长度）
	a.messages = append(a.messages, NewUserMessage(input))
	reqMsgs := make([]Message, len(a.messages))
	copy(reqMsgs, a.messages)
	a.state = StatePending
	a.mu.Unlock()

	newReq := func(stream bool) *ChatCompletionRequest {
		return &ChatCompletionRequest{
			Messages:  reqMsgs,
			Model:     a.model,
			MaxTokens: a.maxTokens,
			Stream:    stream,
		}
	}

	// 重试循环：可重试类别 + 尚未向用户输出过内容 + ctx 未取消
	var (
		reply      string
		usage      *Usage
		finish     string
		ttftMs     int64
		attempts   int
		emittedAny bool
		lastErr    error
	)
	for attempt := 0; ; attempt++ {
		attempts = attempt + 1
		callStart := time.Now()

		if emit == nil {
			resp, err := a.client.ChatCompletion(ctx, newReq(false))
			if err == nil {
				reply = resp.Choices[0].Message.Content
				finish = resp.Choices[0].FinishReason
				usage = resp.Usage
				ttftMs = msSince(callStart)
				lastErr = nil
				break
			}
			lastErr = err
		} else {
			started := false
			res, err := a.client.StreamChatCompletion(ctx, newReq(true), func(s string) error {
				if !started {
					ttftMs = msSince(callStart)
					started = true
					emittedAny = true
				}
				return emit(s)
			})
			if err == nil {
				reply, finish, usage = res.Content, res.FinishReason, res.Usage
				if !started {
					ttftMs = msSince(callStart)
				}
				lastErr = nil
				break
			}
			lastErr = err
		}

		if attempt >= maxRetries || emittedAny || ctx.Err() != nil || !retryable(lastErr) {
			break
		}
		select {
		case <-time.After(retryBaseDelay << uint(attempt)):
		case <-ctx.Done():
			lastErr = ctx.Err()
		}
		if ctx.Err() != nil {
			break
		}
	}

	// 结局收敛（D1）：成功提交成对消息，失败回滚到 base
	a.mu.Lock()
	outcome := OutcomeCommitted
	if lastErr == nil {
		a.messages = append(a.messages, NewAssistantMessage(reply))
		if usage != nil {
			a.usage = usage
		}
	} else if ctx.Err() != nil {
		outcome = OutcomeAborted
		a.messages = a.messages[:base]
	} else {
		outcome = OutcomeRolledBack
		a.messages = a.messages[:base]
	}
	a.state = StateIdle
	historyLen := len(a.messages)
	a.mu.Unlock()

	result := TurnResult{
		TurnID:       turnID,
		Outcome:      outcome,
		Reply:        reply,
		Truncated:    finish == "length",
		FinishReason: finish,
		Usage:        usage,
		TTFTMs:       ttftMs,
		DurationMs:   msSince(turnStart),
		Attempts:     attempts,
	}
	if lastErr != nil {
		result.ErrClass = ClassOf(lastErr)
	}

	// 结构化日志（D6）：密钥绝不入日志
	var pt, ct int
	if usage != nil {
		pt, ct = usage.PromptTokens, usage.CompletionTokens
	}
	a.logger.Info("turn",
		"turn_id", turnID,
		"outcome", outcome,
		"err_class", result.ErrClass,
		"ttft_ms", ttftMs,
		"duration_ms", result.DurationMs,
		"attempts", attempts,
		"finish_reason", finish,
		"truncated", result.Truncated,
		"prompt_tokens", pt,
		"completion_tokens", ct,
		"history_len", historyLen,
	)

	return result, lastErr
}

// retryable 判定错误是否可重试（幂等前提：user 消息只入历史一次）。
func retryable(err error) bool {
	switch ClassOf(err) {
	case ClassNetwork, ClassTimeout, ClassServer, ClassRateLimit:
		return true
	}
	return false
}

// checkBudget 上下文预算预检（D4）。
// 估算 = 历史 + 本轮输入 + max_tokens（最坏情况：本轮 prompt 与最大回复都要放进上下文）。
// 估算超预算 → 显式 ClassBudget 错误，请求不发出、历史不动。
func (a *Agent) checkBudget(input string) error {
	if a.budget <= 0 {
		return nil
	}
	a.mu.RLock()
	est := 0
	for _, m := range a.messages {
		est += EstimateTokens(m.Content) + 4 // 每条消息的 role/格式开销
	}
	a.mu.RUnlock()
	est += EstimateTokens(input) + a.maxTokens
	if est > a.budget {
		return classErr(ClassBudget, fmt.Errorf(
			"上下文预算不足: 估算 %d tokens > 预算 %d（历史+本轮输入+max_tokens）", est, a.budget))
	}
	return nil
}

// EstimateTokens 粗略 token 估算（rune 数向上取整除 2）。
// 这是估算不是真值：真值来自服务端 usage，见 GetStats 的 Usage 字段。
func EstimateTokens(s string) int {
	n := len([]rune(s))
	if n == 0 {
		return 0
	}
	return (n + 1) / 2
}

// HistoryStats 是对话历史统计（估算与真值并列，避免把估算当真值）。
type HistoryStats struct {
	TotalMessages int
	Tokens        int      // 估算值（含每条消息 4 token 格式开销）
	Budget        int      // 上下文预算；0 = 未设置
	Usage         *Usage   // 最近一次服务端真值；nil = 服务端未返回
	State         TurnState
}

// GetStats 返回对话历史统计。
func (a *Agent) GetStats() HistoryStats {
	a.mu.RLock()
	defer a.mu.RUnlock()

	stats := HistoryStats{
		TotalMessages: len(a.messages),
		Budget:        a.budget,
		State:         a.state,
	}
	for _, m := range a.messages {
		stats.Tokens += EstimateTokens(m.Content) + 4
	}
	if a.usage != nil {
		u := *a.usage
		stats.Usage = &u
	}
	return stats
}

// GetHistory 返回当前对话历史的只读副本。
func (a *Agent) GetHistory() []Message {
	a.mu.RLock()
	defer a.mu.RUnlock()

	history := make([]Message, len(a.messages))
	copy(history, a.messages)
	return history
}

// State 返回轮次状态机当前状态。
func (a *Agent) State() TurnState {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state
}

// SetSystemMessage 设置或更新系统提示词（始终位于第 0 条）。
// 与轮次串行：等待在途轮次完成后执行；禁止在 emit 回调内调用。
func (a *Agent) SetSystemMessage(content string) {
	a.turnSem <- struct{}{}
	defer func() { <-a.turnSem }()

	a.mu.Lock()
	defer a.mu.Unlock()

	msg := NewSystemMessage(content)
	if len(a.messages) > 0 && a.messages[0].Role == "system" {
		a.messages[0] = msg
	} else {
		a.messages = append([]Message{msg}, a.messages...)
	}
}

// ClearHistory 清空对话历史（保留系统提示词）。
// 与轮次串行：等待在途轮次完成后执行；禁止在 emit 回调内调用。
func (a *Agent) ClearHistory() {
	a.turnSem <- struct{}{}
	defer func() { <-a.turnSem }()

	a.mu.Lock()
	defer a.mu.Unlock()

	var systemMsg Message
	if len(a.messages) > 0 && a.messages[0].Role == "system" {
		systemMsg = a.messages[0]
	}
	a.messages = []Message{systemMsg}
}

func msSince(t time.Time) int64 {
	return time.Since(t).Milliseconds()
}

// AICode End
