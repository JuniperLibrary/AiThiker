// AICode Begin
// Package agent 提供多轮对话 Agent 的核心类型定义。
package agent

import (
	"context"
	"errors"
	"fmt"
)

// Message 表示对话中的一条消息。
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionRequest 表示发送给 API 的聊天补全请求体（OpenAI 标准字段）。
type ChatCompletionRequest struct {
	Messages  []Message `json:"messages"`
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	Stream    bool      `json:"stream"`
}

// ChatCompletionResponse 表示 API 返回的非流式聊天补全响应。
type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   *Usage   `json:"usage,omitempty"`
}

// Choice 表示响应中的一个候选回复。
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// Usage 表示 token 使用统计（服务端返回的真值，取代本地估算）。
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// StreamChunk 表示 SSE 流式响应中的一个数据块。
// Usage/Error 为规范要求的可选字段：流内 error 对象、
// 服务端在流末尾返回的 token 统计（需网关支持，见 SPEC 待核 #1）。
type StreamChunk struct {
	ID                string         `json:"id"`
	Object            string         `json:"object"`
	Created           int64          `json:"created"`
	Model             string         `json:"model"`
	Choices           []StreamChoice `json:"choices"`
	SystemFingerprint string         `json:"system_fingerprint,omitempty"`
	Usage             *Usage         `json:"usage,omitempty"`
	Error             *StreamError   `json:"error,omitempty"`
}

// StreamError 表示流内错误对象（OpenAI 兼容服务可能在流中发送 error 字段）。
type StreamError struct {
	Message string `json:"message"`
	Type    string `json:"type,omitempty"`
	Code    any    `json:"code,omitempty"`
}

// StreamChoice 表示流式响应中的一个候选块。
type StreamChoice struct {
	Index        int     `json:"index"`
	Delta        Delta   `json:"delta"`
	FinishReason *string `json:"finish_reason"`
}

// Delta 表示流式响应中的增量内容。
type Delta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

// TurnState 是 Agent 的稳定状态（轮次状态机）。
// idle --(user 入历史)--> pending --(成功|失败|取消)--> idle
type TurnState string

const (
	StateIdle    TurnState = "idle"    // 无在途轮次
	StatePending TurnState = "pending" // user 消息已入历史，等待结果
)

// TurnOutcome 是一轮对话的结局。
type TurnOutcome string

const (
	OutcomeCommitted  TurnOutcome = "committed"   // user+assistant 成对入库
	OutcomeRolledBack TurnOutcome = "rolled_back" // 失败回滚，历史恢复原状
	OutcomeAborted    TurnOutcome = "aborted"     // 调用方取消/超时中止（同样回滚）
)

// TurnResult 是单轮对话的完整结果：可观测、可记录、可断言。
type TurnResult struct {
	TurnID       string
	Outcome      TurnOutcome
	Reply        string
	Truncated    bool   // finish_reason=length：回复被 max_tokens 截断（必须对用户可见）
	FinishReason string
	Usage        *Usage // 服务端 token 真值（服务端未返回时为 nil）
	TTFTMs       int64  // 首个内容块耗时（非流式 = 总耗时）
	DurationMs   int64
	Attempts     int    // 实际请求次数（含重试）
	ErrClass     string // 结局错误类别，成功为 ""
}

// 错误类别：结构化日志、重试判定与 CLI 展示共用。
const (
	ClassNetwork  = "network"   // 连接失败 / 流中途断开
	ClassTimeout  = "timeout"   // 连接/首字节/空闲超时
	ClassCanceled = "canceled"  // 调用方取消（ctx）
	ClassRateLimit = "rate_limit" // HTTP 429
	ClassServer   = "server"    // HTTP 5xx / 流内 error 对象
	ClassClient   = "client"    // 其他 4xx（不可重试）
	ClassProtocol = "protocol"  // JSON/SSE 解析失败、行超长
	ClassEmpty    = "empty"     // 响应无 choices
	ClassBudget   = "budget"    // 上下文预算不足（请求未发出）
)

// ClassifiedError 是携带类别的错误。
type ClassifiedError struct {
	Class string
	Err   error
}

func (e *ClassifiedError) Error() string { return fmt.Sprintf("[%s] %v", e.Class, e.Err) }
func (e *ClassifiedError) Unwrap() error { return e.Err }

func classErr(class string, err error) error {
	return &ClassifiedError{Class: class, Err: err}
}

// ClassOf 提取错误类别。ctx 错误优先判定；未分类返回 "unknown"；err 为 nil 返回 ""。
func ClassOf(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.Canceled) {
		return ClassCanceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ClassTimeout
	}
	var ce *ClassifiedError
	if errors.As(err, &ce) {
		return ce.Class
	}
	return "unknown"
}

// NewUserMessage 创建一条用户消息。
func NewUserMessage(content string) Message {
	return Message{Role: "user", Content: content}
}

// NewAssistantMessage 创建一条助手消息。
func NewAssistantMessage(content string) Message {
	return Message{Role: "assistant", Content: content}
}

// NewSystemMessage 创建一条系统消息。
func NewSystemMessage(content string) Message {
	return Message{Role: "system", Content: content}
}

// AICode End
