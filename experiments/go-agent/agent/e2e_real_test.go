// AICode Begin
package agent

import (
	"context"
	"os"
	"testing"
)

// TestE2ERealEndpoint 真实端点端到端（门禁：E2E_REAL=1 + API_KEY + MODEL_NAME）。
//
// 用途：
//   - 验证真实多轮上下文保持（第 2 轮回忆第 1 轮的信息）
//   - 采集 usage 真值与 TTFT/总时长（真实端点 N 轮分布）
//   - 观察真实 SSE 字节行为是否与规范子集兼容
//
// 运行：$env:E2E_REAL="1"; $env:API_KEY="..."; $env:MODEL_NAME="..."; go test -run TestE2ERealEndpoint -v ./agent/
func TestE2ERealEndpoint(t *testing.T) {
	if os.Getenv("E2E_REAL") != "1" {
		t.Skip("门禁未开启：设置 E2E_REAL=1（并配置 API_KEY / MODEL_NAME / ENDPOINT）后运行")
	}
	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		t.Fatal("E2E_REAL=1 但未设置 API_KEY")
	}
	model := os.Getenv("MODEL_NAME")
	if model == "" {
		t.Fatal("E2E_REAL=1 但未设置 MODEL_NAME")
	}

	a := NewAgent(Options{
		BaseURL:   envOr("ENDPOINT", "https://api.openai.com/v1/chat/completions"),
		APIKey:    apiKey,
		Model:     model,
		MaxTokens: 1024,
	})

	ctx := context.Background()

	// 轮次 1：注入信息
	r1, err := a.Chat(ctx, "我的代号是蓝猫七号。请记住。")
	if err != nil {
		t.Fatalf("轮次 1 失败[%s]: %v", ClassOf(err), err)
	}
	t.Logf("轮次 1: outcome=%s ttft=%dms dur=%dms usage=%+v",
		r1.Outcome, r1.TTFTMs, r1.DurationMs, r1.Usage)

	// 轮次 2：回忆测试（多轮上下文保持的直接证据）
	r2, err := a.Chat(ctx, "我的代号是什么？只回答代号。")
	if err != nil {
		t.Fatalf("轮次 2 失败[%s]: %v", ClassOf(err), err)
	}
	t.Logf("轮次 2: outcome=%s ttft=%dms dur=%dms usage=%+v reply=%q",
		r2.Outcome, r2.TTFTMs, r2.DurationMs, r2.Usage, r2.Reply)
	if r2.Reply == "" {
		t.Fatal("轮次 2 回复为空")
	}

	// 轮次 3：流式 + 观察真实流行为
	r3, err := a.ChatStream(ctx, "用一句话评价蓝猫这个代号。", func(string) error { return nil })
	if err != nil {
		t.Fatalf("轮次 3（流式）失败[%s]: %v", ClassOf(err), err)
	}
	t.Logf("轮次 3（流式）: outcome=%s ttft=%dms dur=%dms usage=%+v truncated=%v",
		r3.Outcome, r3.TTFTMs, r3.DurationMs, r3.Usage, r3.Truncated)

	assertInvariant(t, a)
	if got := len(a.GetHistory()); got != 7 {
		t.Fatalf("3 轮后应有 7 条消息（1+6），实际 %d", got)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// AICode End
