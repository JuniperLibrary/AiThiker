// AICode Begin
package agent

import (
	"strings"
	"testing"
)

// TestNewAgent 验证 Agent 初始化和系统提示词。
func TestNewAgent(t *testing.T) {
	a := NewAgent("http://test:8080/v1/chat/completions", "test-key", "test-model", 100, false, "test-chat")
	if a == nil {
		t.Fatal("NewAgent() 返回 nil")
	}

	history := a.GetHistory()
	if len(history) == 0 {
		t.Fatal("Agent 初始化后消息历史不应为空")
	}
	if history[0].Role != "system" {
		t.Fatalf("首条消息应为 system 角色，实际为: %s", history[0].Role)
	}
	if history[0].Content == "" {
		t.Fatal("系统提示词不应为空")
	}
}

// TestSetSystemMessage 验证设置系统提示词。
func TestSetSystemMessage(t *testing.T) {
	a := NewAgent("http://test:8080/v1/chat/completions", "test-key", "test-model", 100, false, "test-chat")
	a.SetSystemMessage("新的系统提示词")

	history := a.GetHistory()
	if history[0].Content != "新的系统提示词" {
		t.Fatalf("系统提示词未更新，期望: '新的系统提示词', 实际: '%s'", history[0].Content)
	}
}

// TestClearHistory 验证清空历史会保留系统提示词。
func TestClearHistory(t *testing.T) {
	a := NewAgent("http://test:8080/v1/chat/completions", "test-key", "test-model", 100, false, "test-chat")

	// 手动追加一些消息模拟对话
	a.mu.Lock()
	a.messages = append(a.messages, NewUserMessage("你好"))
	a.messages = append(a.messages, NewAssistantMessage("你好！有什么我可以帮助你的吗？"))
	a.mu.Unlock()

	if len(a.GetHistory()) != 3 {
		t.Fatalf("期望 3 条消息 (system+user+assistant)，实际: %d", len(a.GetHistory()))
	}

	a.ClearHistory()
	history := a.GetHistory()
	if len(history) != 1 {
		t.Fatalf("清空后应只有 1 条系统消息，实际: %d", len(history))
	}
	if history[0].Role != "system" {
		t.Fatal("清空后首条消息应为 system 角色")
	}
}

// TestMessageCreation 验证消息创建辅助函数。
func TestMessageCreation(t *testing.T) {
	userMsg := NewUserMessage("用户消息")
	if userMsg.Role != "user" || userMsg.Content != "用户消息" {
		t.Fatalf("NewUserMessage 结果不正确: %+v", userMsg)
	}

	assistantMsg := NewAssistantMessage("助手回复")
	if assistantMsg.Role != "assistant" || assistantMsg.Content != "助手回复" {
		t.Fatalf("NewAssistantMessage 结果不正确: %+v", assistantMsg)
	}

	systemMsg := NewSystemMessage("系统提示")
	if systemMsg.Role != "system" || systemMsg.Content != "系统提示" {
		t.Fatalf("NewSystemMessage 结果不正确: %+v", systemMsg)
	}
}

// TestGetStats 验证对话统计功能。
func TestGetStats(t *testing.T) {
	a := NewAgent("http://test:8080/v1/chat/completions", "test-key", "test-model", 100, false, "test-chat")

	// 初始状态
	stats := a.GetStats()
	if stats.TotalMessages != 1 {
		t.Fatalf("初始化后应有 1 条消息 (system)，实际: %d", stats.TotalMessages)
	}

	// 追加测试消息
	a.mu.Lock()
	a.messages = append(a.messages, NewUserMessage("你好"))
	a.messages = append(a.messages, NewAssistantMessage("你好！我是 AI 助手。"))
	a.mu.Unlock()

	stats = a.GetStats()
	if stats.TotalMessages != 3 {
		t.Fatalf("期望 3 条消息，实际: %d", stats.TotalMessages)
	}
	if stats.Tokens <= 0 {
		t.Fatal("Token 估算应为正数")
	}
}

// TestResetChatID 验证重置对话 ID。
func TestResetChatID(t *testing.T) {
	a := NewAgent("http://test:8080/v1/chat/completions", "test-key", "test-model", 100, false, "test-chat")
	a.ResetChatID("new-chat-id")

	// 无法直接从外部访问 chatID，但我们可以验证 ClearHistory 后功能正常
	a.ClearHistory()
	history := a.GetHistory()
	if len(history) != 1 {
		t.Fatalf("期望 1 条消息，实际: %d", len(history))
	}
}

// TestConcurrentAccess 验证并发读安全（竞态检测）。
func TestConcurrentAccess(t *testing.T) {
	a := NewAgent("http://test:8080/v1/chat/completions", "test-key", "test-model", 100, false, "test-chat")

	done := make(chan bool, 2)

	// 并发写入
	go func() {
		for i := 0; i < 100; i++ {
			a.mu.Lock()
			a.messages = append(a.messages, NewUserMessage("测试消息"))
			a.mu.Unlock()
		}
		done <- true
	}()

	// 并发读取
	go func() {
		for i := 0; i < 100; i++ {
			_ = a.GetHistory()
			_ = a.GetStats()
		}
		done <- true
	}()

	<-done
	<-done

	stats := a.GetStats()
	if stats.TotalMessages < 1 {
		t.Fatal("并发操作后消息数异常")
	}
}

// TestHistoryCopy 验证 GetHistory 返回的是副本而非引用。
func TestHistoryCopy(t *testing.T) {
	a := NewAgent("http://test:8080/v1/chat/completions", "test-key", "test-model", 100, false, "test-chat")
	history := a.GetHistory()
	// 修改副本
	if len(history) > 0 {
		history[0].Content = "篡改内容"
	}
	// 验证原始数据未受影响
	original := a.GetHistory()
	if original[0].Content == "篡改内容" {
		t.Fatal("GetHistory 未返回副本，修改影响了原始数据")
	}
}

// TestLargeMessage 测试大消息场景。
func TestLargeMessage(t *testing.T) {
	a := NewAgent("http://test:8080/v1/chat/completions", "test-key", "test-model", 100, false, "test-chat")

	largeContent := strings.Repeat("这是一个很长的测试消息。", 1000)
	a.mu.Lock()
	a.messages = append(a.messages, NewUserMessage(largeContent))
	a.mu.Unlock()

	history := a.GetHistory()
	found := false
	for _, msg := range history {
		if msg.Content == largeContent {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("大消息未正确保存到历史中")
	}

	stats := a.GetStats()
	if stats.Tokens <= 0 {
		t.Fatal("大消息的 Token 估算应为正数")
	}
}

// AICode End
