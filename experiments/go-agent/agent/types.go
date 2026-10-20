// AICode Begin
// Package agent 提供多轮对话 Agent 的核心类型定义。
package agent

// Message 表示对话中的一条消息。
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionRequest 表示发送给 API 的聊天补全请求体。
// 兼容 OpenAI API 格式，并包含上游网关所需的 chatId/detail 字段。
type ChatCompletionRequest struct {
	ChatID    string    `json:"chatId"`
	Detail    bool      `json:"detail"`
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

// Usage 表示 token 使用统计。
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// StreamChunk 表示 SSE 流式响应中的一个数据块。
type StreamChunk struct {
	ID                string         `json:"id"`
	Object            string         `json:"object"`
	Created           int64          `json:"created"`
	Model             string         `json:"model"`
	Choices           []StreamChoice `json:"choices"`
	SystemFingerprint string         `json:"system_fingerprint,omitempty"`
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
