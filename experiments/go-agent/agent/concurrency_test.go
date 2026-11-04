// AICode Begin
package agent

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"
)

// TestConcurrentTurnsSerialized 并发轮次被串行化，且配对错乱不可能发生（D5）。
// 断言：服务端收到的第 k 个请求的 messages 数 = 2k+2（sys + 2k 历史 + 当前 user），
// 即每次请求携带的历史都是完整成对前缀。
func TestConcurrentTurnsSerialized(t *testing.T) {
	const n = 6
	srv := newMock(t, func(_ int, w http.ResponseWriter, _ *http.Request) {
		time.Sleep(10 * time.Millisecond) // 放大竞态窗口
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, nonStreamBody("ok", "stop"))
	})
	a := newTestAgent(srv.URL)

	var wg sync.WaitGroup
	results := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := a.Chat(context.Background(), fmt.Sprintf("并发轮 %d", i))
			results[i] = err
		}(i)
	}
	wg.Wait()

	for i, err := range results {
		if err != nil {
			t.Fatalf("并发轮 %d 失败: %v", i, err)
		}
	}

	// 串行化断言：第 k 次请求（0-based）应携带 2k+2 条消息
	snaps := srv.captured()
	if len(snaps) != n {
		t.Fatalf("期望 %d 次请求，实际 %d", n, len(snaps))
	}
	for k, msgs := range snaps {
		want := 2*k + 2
		if len(msgs) != want {
			t.Fatalf("请求 %d 携带 %d 条消息，期望 %d（并发未串行化！）", k, len(msgs), want)
		}
		// 快照内部成对性
		if msgs[0].Role != "system" {
			t.Fatalf("请求 %d 首条不是 system", k)
		}
		for i := 1; i < len(msgs)-1; i += 2 {
			if msgs[i].Role != "user" || msgs[i+1].Role != "assistant" {
				t.Fatalf("请求 %d 快照不成对 at %d", k, i)
			}
		}
		if msgs[len(msgs)-1].Role != "user" {
			t.Fatalf("请求 %d 尾部应为当前 user", k)
		}
	}
	assertInvariant(t, a)
}

// TestConcurrentTurnsAndReads 轮次与只读查询并发（配合 -race 运行）。
func TestConcurrentTurnsAndReads(t *testing.T) {
	srv := newMock(t, hOKNonStream("ok"))
	a := newTestAgent(srv.URL)

	var turns, readers sync.WaitGroup
	stop := make(chan struct{})

	// 只读查询协程
	for i := 0; i < 3; i++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_ = a.GetHistory()
					_ = a.GetStats()
					_ = a.State()
				}
			}
		}()
	}

	// 并发轮次
	for i := 0; i < 4; i++ {
		turns.Add(1)
		go func(i int) {
			defer turns.Done()
			for j := 0; j < 3; j++ {
				_, _ = a.Chat(context.Background(), fmt.Sprintf("读压 %d-%d", i, j))
			}
		}(i)
	}

	turns.Wait()  // 先等轮次全部结束
	close(stop)   // 再通知只读协程退出
	readers.Wait()
	assertInvariant(t, a)
}

// TestConcurrentStreamAndChat 流式与非流式并发交错（配合 -race 运行）。
func TestConcurrentStreamAndChat(t *testing.T) {
	srv := newMock(t, func(_ int, w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Millisecond)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		writeSSE(w, sseContent("块"), sseFinish("stop"), sseDone())
	})
	a := newTestAgent(srv.URL)

	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			_, _ = a.Chat(context.Background(), fmt.Sprintf("非流式 %d", i))
		}(i)
		go func(i int) {
			defer wg.Done()
			_, _ = a.ChatStream(context.Background(), fmt.Sprintf("流式 %d", i), func(string) error { return nil })
		}(i)
	}
	wg.Wait()
	assertInvariant(t, a)
}

// AICode End
