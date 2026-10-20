// AICode Begin
package agent

import (
	"fmt"
	"strings"
	"sync"
)

// Agent 是多轮对话 Agent 的核心结构体。
// 它管理对话上下文（消息历史），并通过 APIClient 与上游模型交互。
type Agent struct {
	client    *APIClient
	model     string
	maxTokens int
	stream    bool
	chatID    string

	mu       sync.RWMutex
	messages []Message // 对话消息历史
}

// NewAgent 创建一个新的多轮对话 Agent 实例。
//
// 参数:
//   - baseURL: API 网关地址
//   - apiKey: API 密钥
//   - model: 模型名称
//   - maxTokens: 每次请求最大 token 数
//   - stream: 是否启用流式输出
//   - chatID: 对话会话 ID
func NewAgent(baseURL, apiKey, model string, maxTokens int, stream bool, chatID string) *Agent {
	a := &Agent{
		client:    NewAPIClient(baseURL, apiKey),
		model:     model,
		maxTokens: maxTokens,
		stream:    stream,
		chatID:    chatID,
		messages:  make([]Message, 0),
	}

	// 添加系统提示词，设定 Agent 角色
	systemMsg := NewSystemMessage("你是一个智能 AI 助手，请用中文回答用户的问题。回答简洁准确。")
	a.messages = append(a.messages, systemMsg)

	return a
}

// SetSystemMessage 设置或更新系统提示词。
// 系统提示词始终位于消息列表的第一条。
func (a *Agent) SetSystemMessage(content string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	msg := NewSystemMessage(content)
	if len(a.messages) > 0 && a.messages[0].Role == "system" {
		a.messages[0] = msg
	} else {
		a.messages = append([]Message{msg}, a.messages...)
	}
}

// Chat 发送一条用户消息并获取助手的完整回复（非流式）。
// 自动管理对话上下文：将用户消息和助手回复追加到消息历史。
func (a *Agent) Chat(userMessage string) (string, error) {
	a.mu.Lock()
	// 追加用户消息
	a.messages = append(a.messages, NewUserMessage(userMessage))

	req := &ChatCompletionRequest{
		ChatID:    a.chatID,
		Detail:    false,
		Messages:  a.messages,
		Model:     a.model,
		MaxTokens: a.maxTokens,
		Stream:    false,
	}
	a.mu.Unlock()

	resp, err := a.client.ChatCompletion(req)
	if err != nil {
		return "", fmt.Errorf("Chat 请求失败: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("API 返回空响应")
	}

	reply := resp.Choices[0].Message.Content

	a.mu.Lock()
	// 追加助手回复到消息历史
	a.messages = append(a.messages, NewAssistantMessage(reply))
	a.mu.Unlock()

	return reply, nil
}

// ChatStream 发送一条用户消息并通过 channel 逐块接收流式回复。
// 自动管理对话上下文：在流结束后将完整回复追加到消息历史。
func (a *Agent) ChatStream(userMessage string) (<-chan string, error) {
	a.mu.Lock()
	a.messages = append(a.messages, NewUserMessage(userMessage))

	req := &ChatCompletionRequest{
		ChatID:    a.chatID,
		Detail:    false,
		Messages:  a.messages,
		Model:     a.model,
		MaxTokens: a.maxTokens,
		Stream:    true,
	}
	a.mu.Unlock()

	contentCh, errorCh := a.client.StreamChatCompletion(req)

	// 包装 channel：收集流式内容用于追加到消息历史
	outCh := make(chan string, 100)
	go func() {
		defer close(outCh)
		var fullReply strings.Builder

		for {
			select {
			case content, ok := <-contentCh:
				if !ok {
					// contentCh 关闭，检查 errorCh
					err, hasErr := <-errorCh
					if hasErr {
						outCh <- fmt.Sprintf("\n[错误: %v]", err)
					}
					// 追加完整回复到消息历史
					if fullReply.Len() > 0 {
						a.mu.Lock()
						a.messages = append(a.messages, NewAssistantMessage(fullReply.String()))
						a.mu.Unlock()
					}
					return
				}
				fullReply.WriteString(content)
				outCh <- content
			case err, ok := <-errorCh:
				if ok {
					outCh <- fmt.Sprintf("\n[错误: %v]", err)
				}
				return
			}
		}
	}()

	return outCh, nil
}

// GetHistory 返回当前对话历史的只读副本。
func (a *Agent) GetHistory() []Message {
	a.mu.RLock()
	defer a.mu.RUnlock()

	history := make([]Message, len(a.messages))
	copy(history, a.messages)
	return history
}

// ClearHistory 清空对话历史（保留系统提示词）。
func (a *Agent) ClearHistory() {
	a.mu.Lock()
	defer a.mu.Unlock()

	// 保留系统提示词
	var systemMsg Message
	if len(a.messages) > 0 && a.messages[0].Role == "system" {
		systemMsg = a.messages[0]
	}
	a.messages = []Message{systemMsg}
}

// ResetChatID 重置对话 ID，用于开始新一轮对话。
func (a *Agent) ResetFChatID(chatID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.chatID = chatID
}

// HistoryStats 返回对话历史统计信息。
type HistoryStats struct {
	TotalMessages int
	Tokens        int // 估算的 token 数（按中文 1.5 字符/token，英文 4 字符/token）
}

// GetStats 返回对话历史的统计信息。
func (a *Agent) GetStats() HistoryStats {
	a.mu.RLock()
	defer a.mu.RUnlock()

	stats := HistoryStats{
		TotalMessages: len(a.messages),
	}

	for _, msg := range a.messages {
		// 粗略估算 token 数
		runeCount := len([]rune(msg.Content))
		// 中英文混合估算: 假设约 2 字符/token
		stats.Tokens += runeCount / 2
		if runeCount%2 != 0 {
			stats.Tokens++
		}
	}

	return stats
}

// AICode End
