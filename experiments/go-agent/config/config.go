// AICode Begin
// Package config 提供 Agent 的配置管理，支持从环境变量和默认值加载配置。
package config

import (
	"errors"
	"fmt"
	"os"
)

// Config 存储 Agent 运行所需的全部配置项。
type Config struct {
	// Endpoint 是完整端点 URL（OpenAI 兼容），含协议与路径。
	Endpoint string
	// APIKey 是调用上游 API 的身份凭证（必填，不提供内置默认值）。
	APIKey string
	// Model 是使用的模型名称（必填）。
	Model string
	// MaxTokens 是每次请求的最大 token 数。
	MaxTokens int
	// Stream 是否启用流式输出。
	Stream bool
	// Budget 是上下文预算（prompt+completion 上限）；0 = 不限制（显式声明，见 SPEC D4）。
	Budget int
}

// Load 从环境变量加载配置。
//
// 环境变量:
//
//	API_KEY             - API 密钥（必填：密钥不再内置在代码中，见 SPEC 第 6 节）
//	MODEL_NAME          - 模型名称（必填）
//	ENDPOINT            - 完整端点 URL（默认: https://api.openai.com/v1/chat/completions）
//	MAX_TOKENS          - 每次最大 token 数（默认: 1024）
//	STREAM              - 是否流式输出（默认: true）
//	CONTEXT_BUDGET_TOKENS - 上下文预算（默认: 0 = 不限制）
func Load() (*Config, error) {
	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		return nil, errors.New("API_KEY 环境变量未设置（密钥不再内置在代码中，见 SPEC 第 6 节）")
	}
	model := os.Getenv("MODEL_NAME")
	if model == "" {
		return nil, errors.New("MODEL_NAME 环境变量未设置")
	}
	return &Config{
		Endpoint:  getEnv("ENDPOINT", "https://api.openai.com/v1/chat/completions"),
		APIKey:    apiKey,
		Model:     model,
		MaxTokens: getEnvInt("MAX_TOKENS", 1024),
		Stream:    getEnvBool("STREAM", true),
		Budget:    getEnvInt("CONTEXT_BUDGET_TOKENS", 0),
	}, nil
}

// getEnv 读取环境变量，不存在则返回默认值。
func getEnv(key string, defaultVal string) string {
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
