// AICode Begin
package agent

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// APIClient 封装对上游 API 的 HTTP 调用。
type APIClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewAPIClient 创建 API 客户端实例。
func NewAPIClient(baseURL, apiKey string) *APIClient {
	return &APIClient{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}
}

// ChatCompletion 发送非流式聊天补全请求，返回完整响应。
func (c *APIClient) ChatCompletion(req *ChatCompletionRequest) (*ChatCompletionResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建 HTTP 请求失败: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API 返回错误状态 %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return &chatResp, nil
}

// StreamChatCompletion 发送流式聊天补全请求，通过 channel 逐块返回内容。
// 当 channel 关闭时表示流结束。如果发生错误，会通过 errorCh 传递。
func (c *APIClient) StreamChatCompletion(req *ChatCompletionRequest) (contentCh chan string, errorCh chan error) {
	contentCh = make(chan string, 100)
	errorCh = make(chan error, 1)

	req.Stream = true
	body, err := json.Marshal(req)
	if err != nil {
		errorCh <- fmt.Errorf("序列化请求失败: %w", err)
		close(contentCh)
		close(errorCh)
		return
	}

	httpReq, err := http.NewRequest(http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		errorCh <- fmt.Errorf("创建 HTTP 请求失败: %w", err)
		close(contentCh)
		close(errorCh)
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	go func() {
		defer close(contentCh)
		defer close(errorCh)

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			errorCh <- fmt.Errorf("流式 HTTP 请求失败: %w", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			errorCh <- fmt.Errorf("流式 API 返回错误状态 %d: %s", resp.StatusCode, string(respBody))
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		// SSE 流中每行以 "data: " 开头
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")

			// SSE 结束标记
			if data == "[DONE]" {
				return
			}

			var chunk StreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				// 跳过解析失败的块（非致命）
				continue
			}

			for _, choice := range chunk.Choices {
				if choice.Delta.Content != "" {
					contentCh <- choice.Delta.Content
				}
				if choice.FinishReason != nil {
					return
				}
			}
		}

		if err := scanner.Err(); err != nil {
			errorCh <- fmt.Errorf("读取流式响应失败: %w", err)
		}
	}()

	return
}

// AICode End
