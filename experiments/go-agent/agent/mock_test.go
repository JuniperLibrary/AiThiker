// AICode Begin
package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// mockServer 是可编程的故障注入 mock server（SPEC D7 故障矩阵的载体）。
// 每次请求捕获其 messages 快照与原始 body，供断言"发出去的上下文"。
type mockServer struct {
	*httptest.Server
	mu       sync.Mutex
	requests [][]Message
	rawBodies [][]byte
	handler  func(idx int, w http.ResponseWriter, r *http.Request)
}

func newMock(t *testing.T, handler func(idx int, w http.ResponseWriter, r *http.Request)) *mockServer {
	t.Helper()
	m := &mockServer{handler: handler}
	m.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var req ChatCompletionRequest
		_ = json.Unmarshal(raw, &req)
		m.mu.Lock()
		idx := len(m.requests)
		m.requests = append(m.requests, req.Messages)
		m.rawBodies = append(m.rawBodies, raw)
		m.mu.Unlock()
		m.handler(idx, w, r)
	}))
	t.Cleanup(m.Close)
	return m
}

// captured 返回每次请求的 messages 快照副本。
func (m *mockServer) captured() [][]Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([][]Message, len(m.requests))
	copy(out, m.requests)
	return out
}

// rawBody 返回第 idx 次请求的原始 body。
func (m *mockServer) rawBody(idx int) []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.rawBodies[idx]
}

func (m *mockServer) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.requests)
}

// newTestAgent 构造指向 mock 的 Agent。
func newTestAgent(baseURL string) *Agent {
	return NewAgent(Options{
		BaseURL:   baseURL,
		APIKey:    "test-key",
		Model:     "test-model",
		MaxTokens: 100,
	})
}

// ---- 响应体构造 ----

func nonStreamBody(reply, finish string) string {
	return fmt.Sprintf(
		`{"id":"cmpl-1","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":%q},"finish_reason":%q}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`,
		reply, finish)
}

func sseContent(content string) string {
	return fmt.Sprintf("data: {\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":%q}}]}\n\n", content)
}

func sseFinish(reason string) string {
	return fmt.Sprintf("data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":%q}]}\n\n", reason)
}

func sseDone() string { return "data: [DONE]\n\n" }

// writeSSE 写入 SSE 响应（逐块 flush）。
func writeSSE(w http.ResponseWriter, parts ...string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	fl, _ := w.(http.Flusher)
	for _, p := range parts {
		_, _ = w.Write([]byte(p))
		if fl != nil {
			fl.Flush()
		}
	}
}

// hOKNonStream 恒定成功的非流式处理器。
func hOKNonStream(reply string) func(int, http.ResponseWriter, *http.Request) {
	return func(_ int, w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, nonStreamBody(reply, "stop"))
	}
}

// hOKStream 恒定成功的流式处理器：把 reply 拆两块输出。
func hOKStream(reply string) func(int, http.ResponseWriter, *http.Request) {
	return func(_ int, w http.ResponseWriter, _ *http.Request) {
		half := len([]rune(reply)) / 2
		r := []rune(reply)
		writeSSE(w,
			sseContent(string(r[:half])),
			sseContent(string(r[half:])),
			sseFinish("stop"),
			sseDone())
	}
}

// hStatus 固定状态码错误处理器。
func hStatus(code int, body string) func(int, http.ResponseWriter, *http.Request) {
	return func(_ int, w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		fmt.Fprint(w, body)
	}
}

// AICode End
