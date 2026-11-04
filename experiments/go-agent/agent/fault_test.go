// AICode Begin
package agent

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestFaultConnectionRefused 连接被拒 → network + 回滚。
func TestFaultConnectionRefused(t *testing.T) {
	srv := newMock(t, hOKNonStream("x"))
	url := srv.URL
	srv.Close() // 立即释放端口 → 连接被拒

	a := newTestAgent(url)
	result, err := a.Chat(context.Background(), "无法连接")
	if err == nil {
		t.Fatal("连接被拒应返回错误")
	}
	if ClassOf(err) != ClassNetwork {
		t.Fatalf("错误类别应为 network，实际: %s (err=%v)", ClassOf(err), err)
	}
	if result.Outcome != OutcomeRolledBack {
		t.Fatalf("结局应为 rolled_back，实际: %s", result.Outcome)
	}
	assertInvariant(t, a)
}

// TestFaultRateLimitThenSuccess 429 可重试：退避后成功，且重试幂等
// （每次请求的 messages 快照完全一致——user 只入历史一次）。
func TestFaultRateLimitThenSuccess(t *testing.T) {
	var calls int
	srv := newMock(t, func(_ int, w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls <= 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"error":"rate limited"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, nonStreamBody("重试成功", "stop"))
	})
	a := newTestAgent(srv.URL)

	result, err := a.Chat(context.Background(), "限流轮次")
	if err != nil {
		t.Fatalf("429 退避后应成功: %v", err)
	}
	if result.Outcome != OutcomeCommitted {
		t.Fatalf("结局应为 committed，实际: %s", result.Outcome)
	}
	if result.Attempts != 3 {
		t.Fatalf("期望 3 次尝试（2 次 429 + 1 成功），实际 %d", result.Attempts)
	}
	// 幂等断言：3 次请求的 messages 快照逐字节一致
	snaps := srv.captured()
	if len(snaps) != 3 {
		t.Fatalf("期望 3 次请求，实际 %d", len(snaps))
	}
	for i := 1; i < len(snaps); i++ {
		if fmt.Sprint(snaps[i]) != fmt.Sprint(snaps[0]) {
			t.Fatalf("重试请求载荷不一致（user 被重复追加？）:\n%v\n%v", snaps[0], snaps[i])
		}
	}
	assertInvariant(t, a)
}

// TestFaultClientErrorNoRetry 4xx 不可重试：attempts=1。
func TestFaultClientErrorNoRetry(t *testing.T) {
	srv := newMock(t, hStatus(http.StatusBadRequest, `{"error":"bad request"}`))
	a := newTestAgent(srv.URL)

	result, err := a.Chat(context.Background(), "参数错误")
	if ClassOf(err) != ClassClient {
		t.Fatalf("错误类别应为 client，实际: %s (err=%v)", ClassOf(err), err)
	}
	if result.Attempts != 1 {
		t.Fatalf("4xx 不应重试，attempts=%d", result.Attempts)
	}
	if srv.count() != 1 {
		t.Fatalf("只应发出 1 次请求，实际 %d", srv.count())
	}
	assertInvariant(t, a)
}

// TestFaultStreamErrorChunk 流内 error 对象 → server + 回滚。
func TestFaultStreamErrorChunk(t *testing.T) {
	srv := newMock(t, func(_ int, w http.ResponseWriter, _ *http.Request) {
		writeSSE(w,
			"data: {\"error\":{\"message\":\"upstream exploded\",\"type\":\"server_error\"}}\n\n")
	})
	a := newTestAgent(srv.URL)

	result, err := a.ChatStream(context.Background(), "流内错误", func(string) error { return nil })
	if ClassOf(err) != ClassServer {
		t.Fatalf("错误类别应为 server，实际: %s (err=%v)", ClassOf(err), err)
	}
	if result.Outcome != OutcomeRolledBack {
		t.Fatalf("结局应为 rolled_back，实际: %s", result.Outcome)
	}
	assertInvariant(t, a)
}

// TestFaultTruncatedStream 未见 finish/[DONE] 即 EOF → network（流被截断）。
func TestFaultTruncatedStream(t *testing.T) {
	srv := newMock(t, func(_ int, w http.ResponseWriter, _ *http.Request) {
		writeSSE(w, sseContent("断在这里")) // 无 finish、无 [DONE]
	})
	a := newTestAgent(srv.URL)

	result, err := a.ChatStream(context.Background(), "截断流", func(string) error { return nil })
	if ClassOf(err) != ClassNetwork {
		t.Fatalf("错误类别应为 network，实际: %s (err=%v)", ClassOf(err), err)
	}
	if result.Outcome != OutcomeRolledBack {
		t.Fatalf("结局应为 rolled_back，实际: %s", result.Outcome)
	}
	// 半截内容不得入库
	for _, m := range a.GetHistory() {
		if strings.Contains(m.Content, "断在这里") {
			t.Fatal("截断内容被当完整回复入库")
		}
	}
	assertInvariant(t, a)
}

// TestFaultStreamNotRetriedAfterEmit 流式一旦已向用户输出内容即不重试
// （否则用户会看到重复内容）。
func TestFaultStreamNotRetriedAfterEmit(t *testing.T) {
	var calls int
	srv := newMock(t, func(_ int, w http.ResponseWriter, _ *http.Request) {
		calls++
		writeSSE(w, sseContent("已输出的一块"))
		panic(http.ErrAbortHandler) // 输出后断流
	})
	a := newTestAgent(srv.URL)

	result, err := a.ChatStream(context.Background(), "输出后断流", func(string) error { return nil })
	if err == nil {
		t.Fatal("断流应返回错误")
	}
	if calls != 1 {
		t.Fatalf("已输出内容后不应重试，实际请求 %d 次", calls)
	}
	if result.Outcome != OutcomeRolledBack {
		t.Fatalf("结局应为 rolled_back，实际: %s", result.Outcome)
	}
	assertInvariant(t, a)
}

// TestFaultLineTooLong 单行超上限 → 显式协议错误（而非 64KB 静默死流）。
func TestFaultLineTooLong(t *testing.T) {
	// 直接压测解析器：单行超过 maxSSELineBytes
	long := strings.Repeat("x", maxSSELineBytes+1024)
	err := parseSSE(strings.NewReader("data: "+long+"\n\n"), func(string) error { return nil })
	if err == nil {
		t.Fatal("超长行应返回错误")
	}
	if !errors.Is(err, errSSELineTooLong) {
		t.Fatalf("期望 errSSELineTooLong，实际: %v", err)
	}
}

// TestFaultRetriesStopAfterContextDeadline ctx 截止后不再重试。
func TestFaultRetriesStopAfterContextDeadline(t *testing.T) {
	srv := newMock(t, hStatus(http.StatusInternalServerError, `{"error":"always 500"}`))
	a := newTestAgent(srv.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := a.Chat(ctx, "快速截止")
	if err == nil {
		t.Fatal("应返回错误")
	}
	// 退避节奏 200ms、400ms 远超 50ms 截止：应在截止后立即停止
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("ctx 截止后未及时停止重试: %v", elapsed)
	}
	assertInvariant(t, a)
}

// AICode End
