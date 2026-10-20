// AICode Begin
// Package config 提供 Agent 的配置管理，支持从环境变量和默认值加载配置。
package config

import (
	"fmt"
	"os"
)

// Config 存储 Agent 运行所需的全部配置项。
type Config struct {
	// APIBaseURL 是上游 API 网关地址，格式为 host:port。
	APIBaseURL string
	// APIKey 是调用上游 API 的身份凭证。
	APIKey string
	// Model 是使用的模型名称。
	Model string
	// MaxTokens 是每次请求的最大 token 数。
	MaxTokens int
	// Stream 是否启用流式输出。
	Stream bool
	// ChatID 对话会话标识。
	ChatID string
}

// Load 从环境变量加载配置，不存在则使用默认值。
//
// 环境变量:
//
//	API_BASE_URL - API 网关地址（默认: 20.200.25.216:8080）
//	API_KEY      - API 密钥（默认: test-curl.sh 中的 GLM_KEY）
//	MODEL_NAME   - 模型名称（默认: service-GLM-5-w8a8-vllm-Bigep）
//	MAX_TOKENS   - 每次最大 token 数（默认: 1024）
//	STREAM       - 是否流式输出（默认: true）
//	CHAT_ID      - 对话 ID（默认: 自动生成）
func Load() *Config {
	return &Config{
		APIBaseURL: getEnv("API_BASE_URL", "20.200.25.216:8080"),
		APIKey:     getEnv("API_KEY", "NnVkBHYB7SkS9fXWnvOytael_Rf46j_ijen2h7iH4Zt_by3857pDx26s3igrMYS19CzCI26UHUYTLCQ6U5GvBw"),
		Model:      getEnv("MODEL_NAME", "service-GLM-5-w8a8-vllm-Bigep"),
		MaxTokens:  getEnvInt("MAX_TOKENS", 1024),
		Stream:     getEnvBool("STREAM", true),
		ChatID:     getEnv("CHAT_ID", fmt.Sprintf("chat_%d", os.Getpid())),
	}
}

// APIURL 返回完整的 API 端点 URL。
func (c *Config) APIURL() string {
	return fmt.Sprintf("http://%s/v1/chat/completions", c.APIBaseURL)
}

// getEnv 读取环境变量，不存在则返回默认值。
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// getEnvInt 读取环境变量为整数，不存在或格式错误则返回默认值。
func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		var n int
		if _, err := fmt.Sscanf(val, "%d", &n); err == nil {
			return n
		}
	}
	return defaultVal
}

// getEnvBool 读取环境变量为布尔值，不存在则返回默认值。
// 接受 "true"/"1"/"yes" 为 true，其余为 false。
func getEnvBool(key string, defaultVal bool) bool {
	if val := os.Getenv(key); val != "" {
		return val == "true" || val == "1" || val == "yes"
	}
	return defaultVal
}

// AICode End
