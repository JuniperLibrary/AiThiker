// AICode Begin
package agent

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

// assertInvariant 断言历史不变量（SPEC D1）：
//   - I1 idle：    messages = [system] + (user, assistant)* 严格成对交替
//   - I2 pending： I1 前缀 + 恰好一条尾部 user
func assertInvariant(t *testing.T, a *Agent) {
	t.Helper()
	a.mu.RLock()
	defer a.mu.RUnlock()

	if len(a.messages) == 0 {
		t.Fatal("不变量失败: 历史为空")
	}
	if a.messages[0].Role != "system" {
		t.Fatalf("不变量失败: 首条消息不是 system，而是 %s", a.messages[0].Role)
	}

	body := a.messages[1:] // system 之后的部分
	switch a.state {
	case StateIdle:
		if len(body)%2 != 0 {
			t.Fatalf("不变量 I1 失败: idle 下 system 后应成对，实际 %d 条", len(body))
		}
		for i := 0; i < len(body); i += 2 {
			if body[i].Role != "user" || body[i+1].Role != "assistant" {
				t.Fatalf("不变量 I1 失败: 位置 %d 应为 user/assistant，实际 %s/%s",
					i, body[i].Role, body[i+1].Role)
			}
		}
	case StatePending:
		if len(body) == 0 {
			t.Fatal("不变量 I2 失败: pending 下至少有一条 user")
		}
		if len(body)%2 != 1 {
			t.Fatalf("不变量 I2 失败: pending 下应为奇数条（多一条 user），实际 %d 条", len(body))
		}
		if body[len(body)-1].Role != "user" {
			t.Fatalf("不变量 I2 失败: pending 尾部应为 user，实际 %s", body[len(body)-1].Role)
		}
		for i := 0; i < len(body)-1; i += 2 {
			if body[i].Role != "user" || body[i+1].Role != "assistant" {
				t.Fatalf("不变量 I2 失败: 前缀未成对 at %d: %s/%s",
					i, body[i].Role, body[i+1].Role)
			}
		}
	default:
		t.Fatalf("未知状态: %s", a.state)
	}
}

// TestRollbackOnServerError 失败轮次必须回滚：历史恢复原状，无孤儿 user。
func TestRollbackOnServerError(t *testing.T) {
	srv := newMock(t, hStatus(http.StatusInternalServerError, `{"error":"boom"}`))
	a := newTestAgent(srv.URL)

	before := len(a.GetHistory())
	result, err := a.Chat(context.Background(), "会失败的一轮")
	if err == nil {
		t.Fatal("500 应返回错误")
	}
	if ClassOf(err) != ClassServer {
		t.Fatalf("错误类别应为 server，实际: %s", ClassOf(err))
	}
	if result.Outcome != OutcomeRolledBack {
		t.Fatalf("结局应为 rolled_back，实际: %s", result.Outcome)
	}
	if result.Attempts != 1+maxRetries {
		t.Fatalf("5xx 可重试，期望 attempts=%d，实际 %d", 1+maxRetries, result.Attempts)
	}
	after := len(a.GetHistory())
	if after != before {
		t.Fatalf("失败后历史长度应恢复为 %d，实际 %d（孤儿消息！）", before, after)
	}
	assertInvariant(t, a)
}

// TestRollbackOnMalformedJSON 协议错误回滚。
func TestRollbackOnMalformedJSON(t *testing.T) {
	srv := newMock(t, func(_ int, w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{not-json`)
	})
	a := newTestAgent(srv.URL)

	before := len(a.GetHistory())
	_, err := a.Chat(context.Background(), "畸形响应")
	if ClassOf(err) != ClassProtocol {
		t.Fatalf("错误类别应为 protocol，实际: %s (err=%v)", ClassOf(err), err)
	}
	if len(a.GetHistory()) != before {
		t.Fatal("协议错误未回滚")
	}
	assertInvariant(t, a)
}

// TestEmptyChoices 空 choices 归类为 empty 并回滚。
func TestEmptyChoices(t *testing.T) {
	srv := newMock(t, func(_ int, w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"x","choices":[]}`)
	})
	a := newTestAgent(srv.URL)

	_, err := a.Chat(context.Background(), "空响应")
	if ClassOf(err) != ClassEmpty {
		t.Fatalf("错误类别应为 empty，实际: %s (err=%v)", ClassOf(err), err)
	}
	assertInvariant(t, a)
}

// TestTruncatedFinishLengthVisible finish_reason=length 必须对调用方可见。
func TestTruncatedFinishLengthVisible(t *testing.T) {
	srv := newMock(t, func(_ int, w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, nonStreamBody("被截断的回复", "length"))
	})
	a := newTestAgent(srv.URL)

	result, err := a.Chat(context.Background(), "长问题")
	if err != nil {
		t.Fatalf("length 截断不是错误: %v", err)
	}
	if !result.Truncated {
		t.Fatal("Truncated 应为 true（finish_reason=length）")
	}
	if result.Outcome != OutcomeCommitted {
		t.Fatalf("截断回复仍是成对提交，实际: %s", result.Outcome)
	}
	assertInvariant(t, a)
}

// TestCancelMidStreamAborted 流中取消 → aborted + 回滚。
func TestCancelMidStreamAborted(t *testing.T) {
	release := make(chan struct{})
	srv := newMock(t, func(_ int, w http.ResponseWriter, r *http.Request) {
		writeSSE(w, sseContent("第一块"))
		select { // 等客户端断开或超时
		case <-r.Context().Done():
		case <-release:
		case <-time.After(5 * time.Second):
		}
	})
	defer close(release)
	a := newTestAgent(srv.URL)

	ctx, cancel := context.WithCancel(context.Background())
	before := len(a.GetHistory())
	first := true
	result, err := a.ChatStream(ctx, "会被取消", func(string) error {
		if first {
			first = false
			cancel() // 收到第一块后取消
		}
		return nil
	})
	if err == nil {
		t.Fatal("取消应返回错误")
	}
	if ClassOf(err) != ClassCanceled {
		t.Fatalf("错误类别应为 canceled，实际: %s (err=%v)", ClassOf(err), err)
	}
	if result.Outcome != OutcomeAborted {
		t.Fatalf("结局应为 aborted，实际: %s", result.Outcome)
	}
	if len(a.GetHistory()) != before {
		t.Fatal("取消后历史未回滚")
	}
	if a.State() != StateIdle {
		t.Fatalf("取消后状态应恢复 idle，实际: %s", a.State())
	}
	assertInvariant(t, a)
}

// TestMidStreamDisconnectRollback 流中途断开 → 回滚，半截内容不得入库。
func TestMidStreamDisconnectRollback(t *testing.T) {
	srv := newMock(t, func(_ int, w http.ResponseWriter, _ *http.Request) {
		writeSSE(w, sseContent("半截内容"))
		panic(http.ErrAbortHandler) // 硬断连接
	})
	a := newTestAgent(srv.URL)

	before := len(a.GetHistory())
	result, err := a.ChatStream(context.Background(), "断流轮次", func(string) error { return nil })
	if err == nil {
		t.Fatal("断流应返回错误")
	}
	if result.Outcome != OutcomeRolledBack {
		t.Fatalf("结局应为 rolled_back，实际: %s", result.Outcome)
	}
	if got := len(a.GetHistory()); got != before {
		t.Fatalf("断流后历史应恢复为 %d 条，实际 %d（半截内容入库！）", before, got)
	}
	for _, m := range a.GetHistory() {
		if m.Content == "半截内容" {
			t.Fatal("半截内容被当完整回复入库")
		}
	}
	assertInvariant(t, a)
}

// TestBudgetRejected 不超预算不发请求；超预算显式拒绝且历史不动。
func TestBudgetRejected(t *testing.T) {
	srv := newMock(t, hOKNonStream("不该被调用"))
	// Budget 0 = 不限制 → 放行
	a := newTestAgent(srv.URL)
	if err := a.checkBudget("任意输入"); err != nil {
		t.Fatalf("budget=0 应放行: %v", err)
	}

	// 极小预算 → 拒绝
	a2 := newTestAgent(srv.URL)
	a2.budget = 10
	a2.maxTokens = 100 // 最坏情况估算必然超 10
	before := srv.count()
	_, err := a2.Chat(context.Background(), "输入")
	if ClassOf(err) != ClassBudget {
		t.Fatalf("错误类别应为 budget，实际: %s (err=%v)", ClassOf(err), err)
	}
	if srv.count() != before {
		t.Fatal("超预算不应发出请求")
	}
	if len(a2.GetHistory()) != 1 {
		t.Fatal("超预算不应改动历史")
	}
	assertInvariant(t, a2)
}

// TestPropertyRandomInterleavings 性质测试：随机 成功/失败/取消 序列下，
// 每一轮之后不变量恒成立、历史长度变化符合结局（D1 的机械证明）。
func TestPropertyRandomInterleavings(t *testing.T) {
	// 缩短重试退避以加速（不影响语义）
	origDelay := retryBaseDelay
	retryBaseDelay = 5 * time.Millisecond
	t.Cleanup(func() { retryBaseDelay = origDelay })

	const (
		modeOK = iota
		modeFail500
		modeCancelMidStream
	)
	var mode atomic.Int32
	mode.Store(modeOK)

	srv := newMock(t, func(_ int, w http.ResponseWriter, r *http.Request) {
		switch mode.Load() {
		case modeFail500:
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"error":"boom"}`)
		case modeCancelMidStream:
			writeSSE(w, sseContent("块"))
			select {
			case <-r.Context().Done():
			case <-time.After(5 * time.Second):
			}
		default:
			writeSSE(w, sseContent("回复"), sseFinish("stop"), sseDone())
		}
	})
	a := newTestAgent(srv.URL)

	rng := rand.New(rand.NewSource(42)) // 固定种子：可复现
	for i := 0; i < 100; i++ {
		wantMode := rng.Intn(3)
		mode.Store(int32(wantMode))
		before := len(a.GetHistory())

		ctx, cancel := context.WithCancel(context.Background())
		first := true
		_, err := a.ChatStream(ctx, fmt.Sprintf("第 %d 轮", i), func(string) error {
			if wantMode == modeCancelMidStream && first {
				first = false
				cancel()
			}
			return nil
		})
		cancel()

		wantDelta := 0
		if wantMode == modeOK {
			if err != nil {
				t.Fatalf("轮次 %d: 成功模式应无错误: %v", i, err)
			}
			wantDelta = 2
		} else {
			if err == nil {
				t.Fatalf("轮次 %d: 模式 %d 应返回错误", i, wantMode)
			}
		}
		if got := len(a.GetHistory()); got != before+wantDelta {
			t.Fatalf("轮次 %d (mode=%d): 历史长度变化应为 +%d，实际 %d→%d（outcome 错乱）",
				i, wantMode, wantDelta, before, got)
		}
		assertInvariant(t, a)
	}
}

// AICode End
