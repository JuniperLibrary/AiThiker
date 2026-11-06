// AICode Begin
package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

// 超时分级（D3）：连接 / 首字节 / 流空闲 / 总时长（由调用方 ctx 控制）。
const (
	defaultDialTimeout   = 5 * time.Second
	defaultHeaderTimeout = 30 * time.Second // 首字节（TTFT 预算）
	defaultIdleTimeout   = 60 * time.Second // 流式空闲
	defaultMaxBodyBytes  = 8 << 20          // 非流式响应体上限
	errorBodyLimit       = 512              // 错误响应体截断长度
)

// APIClient 封装对上游 API 的 HTTP 调用。
// 无全局 Client.Timeout：流式长连接需要它不受总时长限制，
// 总时长由请求 ctx 控制；分级超时由 Transport 与空闲看门狗实现。
type APIClient struct {
	baseURL     string
	apiKey      string
	httpClient  *http.Client
	idleTimeout time.Duration
}

// NewAPIClient 创建带默认分级超时的 API 客户端。
func NewAPIClient(baseURL, apiKey string) *APIClient {
	return newAPIClient(baseURL, apiKey, defaultDialTimeout, defaultHeaderTimeout, defaultIdleTimeout)
}

// newAPIClient 供测试注入更短的超时。
func newAPIClient(baseURL, apiKey string, dial, header, idle time.Duration) *APIClient {
	tr := &http.Transport{
		DialContext:           (&net.Dialer{Timeout: dial, KeepAlive: 30 * time.Second}).DialContext,
		TLSHandshakeTimeout:   dial,
		ResponseHeaderTimeout: header, // 首字节超时
		IdleConnTimeout:       90 * time.Second,
	}
	return &APIClient{
		baseURL:     baseURL,
		apiKey:      apiKey,
		httpClient:  &http.Client{Transport: tr},
		idleTimeout: idle,
	}
}

// ChatCompletion 发送非流式聊天补全请求（带 ctx）。
// 所有失败都返回 ClassifiedError，类别见 ClassOf。
func (c *APIClient) ChatCompletion(ctx context.Context, req *ChatCompletionRequest) (*ChatCompletionResponse, error) {
	req.Stream = false
	body, err := json.Marshal(req)
	if err != nil {
		return nil, classErr(ClassProtocol, fmt.Errorf("序列化请求失败: %w", err))
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, classErr(ClassProtocol, fmt.Errorf("创建 HTTP 请求失败: %w", err))
	}
	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, classifyTransportErr(ctx, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, errorBodyLimit))
		return nil, statusError(resp.StatusCode, respBody)
	}

	var chatResp ChatCompletionResponse
	dec := json.NewDecoder(io.LimitReader(resp.Body, defaultMaxBodyBytes))
	if err := dec.Decode(&chatResp); err != nil {
		return nil, classErr(ClassProtocol, fmt.Errorf("响应 JSON 解析失败: %w", err))
	}
	if len(chatResp.Choices) == 0 {
		return nil, classErr(ClassEmpty, errors.New("API 返回空 choices"))
	}
	return &chatResp, nil
}

// streamResult 是流式调用的完整结果。
type streamResult struct {
	Content      string
	FinishReason string
	Usage        *Usage
	DoneMarker   bool // 收到 [DONE] 结束标记
}

// errStreamDone 是 [DONE] 正常结束哨兵（非错误）。
var errStreamDone = errors.New("SSE [DONE]")

// StreamChatCompletion 发送流式请求，每个内容块回调 emit。
// 返回值约定：
//   - 正常结束（收到 [DONE]，或 EOF 前已见 finish_reason）→ (result, nil)
//   - 未见 finish/[DONE] 即 EOF → ClassNetwork（流被截断）
//   - 空闲超时 → ClassTimeout；ctx 取消 → context 错误（ClassOf → canceled）
//   - emit 返回错误 → 原样透传（调用方中止语义）
func (c *APIClient) StreamChatCompletion(ctx context.Context, req *ChatCompletionRequest, emit func(string) error) (*streamResult, error) {
	req.Stream = true
	body, err := json.Marshal(req)
	if err != nil {
		return nil, classErr(ClassProtocol, fmt.Errorf("序列化请求失败: %w", err))
	}

	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(streamCtx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, classErr(ClassProtocol, fmt.Errorf("创建 HTTP 请求失败: %w", err))
	}
	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, classifyTransportErr(ctx, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, errorBodyLimit))
		return nil, statusError(resp.StatusCode, respBody)
	}

	// 空闲看门狗：超过 idleTimeout 无读活动 → cancel → 阻塞中的 Read 立即失败
	reader := io.Reader(resp.Body)
	if c.idleTimeout > 0 {
		var last atomic.Int64
		last.Store(time.Now().UnixNano())
		reader = &activityReader{r: resp.Body, last: &last}
		stop := make(chan struct{})
		defer close(stop)
		go watchdog(stop, &last, c.idleTimeout, cancel)
	}

	res := &streamResult{}
	var sb strings.Builder
	err = parseSSE(reader, func(data string) error {
		if data == "[DONE]" {
			return errStreamDone
		}
		var chunk StreamChunk
		if jsonErr := json.Unmarshal([]byte(data), &chunk); jsonErr != nil {
			return classErr(ClassProtocol, fmt.Errorf("SSE data JSON 解析失败: %w", jsonErr))
		}
		if chunk.Error != nil {
			return classErr(ClassServer, fmt.Errorf("流内错误对象: %s", chunk.Error.Message))
		}
		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				sb.WriteString(choice.Delta.Content)
				if emitErr := emit(choice.Delta.Content); emitErr != nil {
					return emitErr
				}
			}
			if choice.FinishReason != nil && *choice.FinishReason != "" {
				res.FinishReason = *choice.FinishReason
			}
		}
		if chunk.Usage != nil {
			res.Usage = chunk.Usage
		}
		return nil
	})

	switch {
	case err == nil:
		// EOF 正常返回：必须已见 finish_reason，否则视为流被截断
		if res.FinishReason == "" {
			return nil, classErr(ClassNetwork, errors.New("流在收到 finish_reason/[DONE] 前被截断"))
		}
		res.Content = sb.String()
		return res, nil
	case errors.Is(err, errStreamDone):
		res.Content = sb.String()
		res.DoneMarker = true
		return res, nil
	}

	// 失败归类：父 ctx 优先，其次是空闲看门狗，最后透传已分类错误
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if streamCtx.Err() != nil {
		return nil, classErr(ClassTimeout, fmt.Errorf("流空闲超过 %s", c.idleTimeout))
	}
	return nil, err
}

func (c *APIClient) setHeaders(r *http.Request) {
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer "+c.apiKey)
}

// classifyTransportErr 把 http.Client.Do 的错误归类。
func classifyTransportErr(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err() // 取消/截止：由 ClassOf 归类为 canceled/timeout
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return classErr(ClassTimeout, err)
	}
	return classErr(ClassNetwork, err)
}

// statusError 按状态码分类 HTTP 错误。
func statusError(status int, body []byte) error {
	msg := strings.TrimSpace(string(body))
	switch {
	case status == http.StatusTooManyRequests:
		return classErr(ClassRateLimit, fmt.Errorf("HTTP 429: %s", msg))
	case status >= 500:
		return classErr(ClassServer, fmt.Errorf("HTTP %d: %s", status, msg))
	default:
		return classErr(ClassClient, fmt.Errorf("HTTP %d: %s", status, msg))
	}
}

// watchdog 每 idle/4 检查一次最后活动时间，超时则 cancel（中断阻塞读）。
func watchdog(stop <-chan struct{}, last *atomic.Int64, idle time.Duration, cancel context.CancelFunc) {
	interval := idle / 4
	if interval < 10*time.Millisecond {
		interval = 10 * time.Millisecond
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			if time.Since(time.Unix(0, last.Load())) > idle {
				cancel()
				return
			}
		}
	}
}

// activityReader 每次读到数据更新最后活动时间（供空闲看门狗判断）。
type activityReader struct {
	r    io.Reader
	last *atomic.Int64
}

func (a *activityReader) Read(p []byte) (int, error) {
	n, err := a.r.Read(p)
	if n > 0 {
		a.last.Store(time.Now().UnixNano())
	}
	return n, err
}

// AICode End
