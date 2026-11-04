// AICode Begin
package agent

import (
	"context"
	"strings"
	"testing"
)

// TestNewAgent 验证 Agent 初始化和系统提示词。
func TestNewAgent(t *testing.T) {
	a := newTestAgent("http://test:8080/v1/chat/completions")
	if a == nil {
		t.Fatal("NewAgent() 返回 nil")
	}
	if a.State() != StateIdle {
		t.Fatalf("初始状态应为 idle，实际: %s", a.State())
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
	a := newTestAgent("http://test:8080/v1/chat/completions")
	a.SetSystemMessage("新的系统提示词")

	history := a.GetHistory()
	if history[0].Content != "新的系统提示词" {
		t.Fatalf("系统提示词未更新，期望: '新的系统提示词', 实际: '%s'", history[0].Content)
	}
}

// TestClearHistory 验证清空历史会保留系统提示词。
func TestClearHistory(t *testing.T) {
	a := newTestAgent("http://test:8080/v1/chat/completions")

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

// TestGetStats 验证对话统计功能（估算与真值并列）。
func TestGetStats(t *testing.T) {
	a := newTestAgent("http://test:8080/v1/chat/completions")

	stats := a.GetStats()
	if stats.TotalMessages != 1 {
		t.Fatalf("初始化后应有 1 条消息 (system)，实际: %d", stats.TotalMessages)
	}
	if stats.State != StateIdle {
		t.Fatalf("初始状态应为 idle，实际: %s", stats.State)
	}
	if stats.Usage != nil {
		t.Fatal("未发生请求时不应有服务端真值")
	}

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

// TestRequestOnlyStandardFields 请求体只携带 OpenAI 标准字段
// （退出内网后不再有 chatId/detail 等网关私有字段）。
func TestRequestOnlyStandardFields(t *testing.T) {
	srv := newMock(t, hOKNonStream("ok"))
	a := newTestAgent(srv.URL)

	if _, err := a.Chat(context.Background(), "字段检查"); err != nil {
		t.Fatalf("Chat 失败: %v", err)
	}
	if srv.count() != 1 {
		t.Fatalf("期望 1 次请求，实际 %d", srv.count())
	}
	body := string(srv.rawBody(0))
	for _, banned := range []string{"chatId", "detail"} {
		if strings.Contains(body, `"`+banned+`"`) {
			t.Fatalf("请求体含网关私有字段 %q: %s", banned, body)
		}
	}
	for _, want := range []string{"messages", "model", "max_tokens", "stream"} {
		if !strings.Contains(body, `"`+want+`"`) {
			t.Fatalf("请求体缺少标准字段 %q: %s", want, body)
		}
	}
}

// TestConcurrentAccess 验证并发读写安全（配合 -race 运行）。
func TestConcurrentAccess(t *testing.T) {
	a := newTestAgent("http://test:8080/v1/chat/completions")

	done := make(chan bool, 2)

	go func() {
		for i := 0; i < 100; i++ {
			a.mu.Lock()
			a.messages = append(a.messages, NewUserMessage("测试消息"))
			a.mu.Unlock()
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			_ = a.GetHistory()
			_ = a.GetStats()
			_ = a.State()
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
	a := newTestAgent("http://test:8080/v1/chat/completions")
	history := a.GetHistory()
	if len(history) > 0 {
		history[0].Content = "篡改内容"
	}
	original := a.GetHistory()
	if original[0].Content == "篡改内容" {
		t.Fatal("GetHistory 未返回副本，修改影响了原始数据")
	}
}

// TestLargeMessage 测试大消息场景。
func TestLargeMessage(t *testing.T) {
	a := newTestAgent("http://test:8080/v1/chat/completions")

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

// TestChatCommitted 端到端（mock）：成功轮次成对入库 + usage 真值回填。
func TestChatCommitted(t *testing.T) {
	srv := newMock(t, hOKNonStream("测试回复"))
	a := newTestAgent(srv.URL)

	result, err := a.Chat(context.Background(), "第一轮")
	if err != nil {
		t.Fatalf("Chat 失败: %v", err)
	}
	if result.Outcome != OutcomeCommitted {
		t.Fatalf("结局应为 committed，实际: %s", result.Outcome)
	}
	if result.Reply != "测试回复" {
		t.Fatalf("回复不正确: %q", result.Reply)
	}
	if result.Usage == nil || result.Usage.PromptTokens != 10 {
		t.Fatalf("usage 真值未回填: %+v", result.Usage)
	}
	if result.Attempts != 1 {
		t.Fatalf("成功不应重试，attempts=%d", result.Attempts)
	}
	if a.State() != StateIdle {
		t.Fatalf("轮次结束后应为 idle，实际: %s", a.State())
	}
	assertInvariant(t, a)
}

// AICode End
