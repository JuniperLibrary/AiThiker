// AICode Begin
package agent

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"testing"
	"time"
)

// newTimeoutAgent 构造带短超时的 Agent（timeout 测试专用）。
func newTimeoutAgent(baseURL string, header, idle time.Duration) *Agent {
	a := newTestAgent(baseURL)
	a.client = newAPIClient(baseURL, "test-key", 50*time.Millisecond, header, idle)
	return a
}

// TestHeaderTimeout 首字节超时 → timeout + 回滚。
func TestHeaderTimeout(t *testing.T) {
	srv := newMock(t, func(_ int, w http.ResponseWriter, r *http.Request) {
		// 挂住不写响应头，直到客户端断开
		select {
		case <-r.Context().Done():
		case <-time.After(10 * time.Second):
		}
	})
	a := newTimeoutAgent(srv.URL, 100*time.Millisecond, time.Second)

	start := time.Now()
	result, err := a.Chat(context.Background(), "慢响应")
	if ClassOf(err) != ClassTimeout {
		t.Fatalf("错误类别应为 timeout，实际: %s (err=%v)", ClassOf(err), err)
	}
	if result.Outcome != OutcomeRolledBack {
		t.Fatalf("结局应为 rolled_back，实际: %s", result.Outcome)
	}
	// 头超时 100ms，重试 3 次（可重试类别），总时长应显著小于 10s
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("未及时中止: %v", elapsed)
	}
	assertInvariant(t, a)
}

// TestIdleTimeout 流式空闲超时：收到一块后服务器静默，看门狗中断 → timeout。
func TestIdleTimeout(t *testing.T) {
	srv := newMock(t, func(_ int, w http.ResponseWriter, r *http.Request) {
		writeSSE(w, sseContent("这一块收到了"))
		// 静默（客户端断开即返回）
		select {
		case <-r.Context().Done():
		case <-time.After(10 * time.Second):
		}
	})
	a := newTimeoutAgent(srv.URL, time.Second, 150*time.Millisecond)

	var received string
	start := time.Now()
	result, err := a.ChatStream(context.Background(), "静默流", func(s string) error {
		received += s
		return nil
	})
	if ClassOf(err) != ClassTimeout {
		t.Fatalf("错误类别应为 timeout，实际: %s (err=%v)", ClassOf(err), err)
	}
	if received != "这一块收到了" {
		t.Fatalf("应已收到第一块，实际: %q", received)
	}
	if result.Outcome != OutcomeRolledBack {
		t.Fatalf("结局应为 rolled_back（半截内容不入库），实际: %s", result.Outcome)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("看门狗未及时中断: %v", elapsed)
	}
	assertInvariant(t, a)
}

// TestCancelNonStreamAborted 非流式请求中取消 → aborted + 回滚。
func TestCancelNonStreamAborted(t *testing.T) {
	release := make(chan struct{})
	srv := newMock(t, func(_ int, w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		case <-time.After(10 * time.Second):
		}
	})
	defer close(release)
	a := newTestAgent(srv.URL)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	result, err := a.Chat(ctx, "会被取消的非流式")
	if ClassOf(err) != ClassCanceled {
		t.Fatalf("错误类别应为 canceled，实际: %s (err=%v)", ClassOf(err), err)
	}
	if result.Outcome != OutcomeAborted {
		t.Fatalf("结局应为 aborted，实际: %s", result.Outcome)
	}
	assertInvariant(t, a)
}

// TestNoGoroutineLeak 取消/超时路径不产生 goroutine 泄漏（D3）。
func TestNoGoroutineLeak(t *testing.T) {
	release := make(chan struct{})
	srv := newMock(t, func(_ int, w http.ResponseWriter, r *http.Request) {
		writeSSE(w, sseContent("块"))
		select {
		case <-r.Context().Done():
		case <-release:
		case <-time.After(10 * time.Second):
		}
	})
	a := newTestAgent(srv.URL)

	// 预热一轮：让 transport keep-alive 等持久 goroutine 建立后再取基线
	// （emit 内取消，避免流挂住）
	ctx0, cancel0 := context.WithCancel(context.Background())
	warmed := false
	_, _ = a.ChatStream(ctx0, "预热", func(string) error {
		if !warmed {
			warmed = true
			cancel0()
		}
		return nil
	})
	cancel0()
	baseline := goroutineCount(t)

	for i := 0; i < 5; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		first := true
		_, _ = a.ChatStream(ctx, fmt.Sprintf("轮次 %d", i), func(string) error {
			if first {
				first = false
				cancel()
			}
			return nil
		})
		cancel()
	}
	close(release)

	if got := goroutineCount(t); got != baseline {
		t.Fatalf("疑似 goroutine 泄漏: baseline=%d, after=%d", baseline, got)
	}
}

// goroutineCount 等待 goroutine 数收敛后返回（给清理 goroutine 一些时间）。
func goroutineCount(t *testing.T) int {
	t.Helper()
	for i := 0; i < 50; i++ {
		n := runtime.NumGoroutine()
		if i == 0 {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		prev := n
		time.Sleep(20 * time.Millisecond)
		if runtime.NumGoroutine() == prev {
			return prev
		}
	}
	return runtime.NumGoroutine()
}

// AICode End
