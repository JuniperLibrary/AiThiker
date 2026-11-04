// AICode Begin
// 多轮对话 Agent — 命令行交互入口
//
// 使用方式:
//
//	set API_KEY=your_key
//	set MODEL_NAME=your_model
//	go run main.go
//
// 内置命令:
//
//	/exit, /quit    — 退出程序
//	/clear          — 清空对话历史
//	/stats          — 显示当前对话统计
//	/stream on|off  — 切换流式输出模式
//	/help           — 显示帮助信息
//
// Ctrl-C：中断在途轮次（历史回滚，不产生孤儿消息），再次 Ctrl-C 退出。
package main

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"

	"github.com/aithiker/go-agent/agent"
	"github.com/aithiker/go-agent/config"
)

func main() {
	// 结构化日志走 stderr，不污染对话输出；密钥绝不入日志
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	fmt.Println("================================================")
	fmt.Println("  AiThiker Go Agent — 多轮对话命令行客户端")
	fmt.Println("================================================")
	fmt.Println()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 配置错误: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("▪ 端点: %s\n", cfg.Endpoint)
	fmt.Printf("▪ 模型名称: %s\n", cfg.Model)
	fmt.Printf("▪ 流式模式: %v\n", cfg.Stream)
	if cfg.Budget > 0 {
		fmt.Printf("▪ 上下文预算: %d tokens\n", cfg.Budget)
	} else {
		fmt.Println("▪ 上下文预算: 未设置（不限制）")
	}
	fmt.Println()

	agt := agent.NewAgent(agent.Options{
		BaseURL:   cfg.Endpoint,
		APIKey:    cfg.APIKey,
		Model:     cfg.Model,
		MaxTokens: cfg.MaxTokens,
		Budget:    cfg.Budget,
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	streamMode := cfg.Stream
	scanner := bufio.NewScanner(os.Stdin)

	printHelp()
	fmt.Println()

	for {
		fmt.Print(">> 请输入 (输入 /help 查看命令): ")
		if !scanner.Scan() {
			fmt.Println()
			fmt.Println("👋 输入流结束，再见！")
			return
		}
		input := strings.TrimSpace(scanner.Text())

		if input == "" {
			continue
		}
		if ctx.Err() != nil {
			fmt.Println("👋 已收到中断信号，再见！")
			return
		}

		if strings.HasPrefix(input, "/") {
			handleCommand(input, agt, &streamMode)
			continue
		}

		if streamMode {
			chatStream(ctx, agt, input)
		} else {
			chatNonStream(ctx, agt, input)
		}
	}
}

// chatNonStream 使用非流式模式发送消息并打印完整回复。
func chatNonStream(ctx context.Context, agt *agent.Agent, input string) {
	fmt.Println()
	fmt.Println("🤖 Agent 思考中...")
	result, err := agt.Chat(ctx, input)
	printOutcome(result, err)
	if err == nil {
		fmt.Println()
		fmt.Println("━━━━━━━━━━━━━━━━ 回复 ━━━━━━━━━━━━━━━━")
		fmt.Println("🤖", result.Reply)
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println()
	}
}

// chatStream 使用流式模式发送消息并逐块打印回复。
func chatStream(ctx context.Context, agt *agent.Agent, input string) {
	fmt.Println()
	fmt.Print("🤖 ")
	result, err := agt.ChatStream(ctx, input, func(s string) error {
		fmt.Print(s)
		return nil
	})
	fmt.Println()
	fmt.Println()
	printOutcome(result, err)
}

// printOutcome 打印轮次结局（错误类别、截断警告、token 真值）。
func printOutcome(result agent.TurnResult, err error) {
	if err != nil {
		fmt.Printf("❌ 错误[%s]: %v\n", agent.ClassOf(err), err)
		if agent.ClassOf(err) == agent.ClassCanceled {
			fmt.Println("ℹ️  本轮已中止并回滚，历史保持一致")
		}
		return
	}
	if result.Truncated {
		fmt.Println("⚠️  回复被 max_tokens 截断（finish_reason=length），内容不完整")
	}
	if result.Usage != nil {
		fmt.Printf("🧮 tokens: prompt=%d completion=%d（服务端真值）\n",
			result.Usage.PromptTokens, result.Usage.CompletionTokens)
	}
}

// handleCommand 处理内置命令。
func handleCommand(input string, agt *agent.Agent, streamMode *bool) {
	parts := strings.Fields(input)
	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "/exit", "/quit":
		fmt.Println("👋 再见！")
		os.Exit(0)

	case "/clear":
		agt.ClearHistory()
		fmt.Println("✅ 对话历史已清空（系统提示词已保留）")

	case "/stats":
		stats := agt.GetStats()
		fmt.Println("📊 对话统计:")
		fmt.Printf("  消息总数: %d\n", stats.TotalMessages)
		fmt.Printf("  估算 Token: %d（估算非真值）\n", stats.Tokens)
		if stats.Usage != nil {
			fmt.Printf("  服务端真值: prompt=%d completion=%d total=%d\n",
				stats.Usage.PromptTokens, stats.Usage.CompletionTokens, stats.Usage.TotalTokens)
		} else {
			fmt.Println("  服务端真值: 暂无（服务端未返回 usage）")
		}
		if stats.Budget > 0 {
			fmt.Printf("  上下文预算: %d\n", stats.Budget)
		} else {
			fmt.Println("  上下文预算: 未设置（不限制）")
		}
		fmt.Printf("  轮次状态: %s\n", stats.State)
		fmt.Println()
		fmt.Println("📜 对话历史:")
		for _, msg := range agt.GetHistory() {
			roleName := map[string]string{
				"system":    "系统",
				"user":      "用户",
				"assistant": "助手",
			}
			preview := msg.Content
			if len([]rune(preview)) > 80 {
				preview = string([]rune(preview)[:80]) + "..."
			}
			fmt.Printf("  [%s] %s\n", roleName[msg.Role], preview)
		}

	case "/stream":
		if len(parts) < 2 {
			fmt.Printf("ℹ️  当前流式模式: %v\n", *streamMode)
			fmt.Println("用法: /stream on 或 /stream off")
			return
		}
		switch parts[1] {
		case "on", "1", "true", "yes":
			*streamMode = true
			fmt.Println("✅ 已切换为流式输出模式")
		case "off", "0", "false", "no":
			*streamMode = false
			fmt.Println("✅ 已切换为非流式输出模式")
		default:
			fmt.Printf("❌ 未知参数: %s (可用: on/off)\n", parts[1])
		}

	case "/help":
		printHelp()

	default:
		fmt.Printf("❌ 未知命令: %s (输入 /help 查看可用命令)\n", cmd)
	}
}

// printHelp 打印帮助信息。
func printHelp() {
	fmt.Println("━━━━━━━━━━━━━━━━ 命令帮助 ━━━━━━━━━━━━━━━━")
	fmt.Println("  /exit, /quit    — 退出程序")
	fmt.Println("  /clear          — 清空对话历史")
	fmt.Println("  /stats          — 显示对话统计和历史")
	fmt.Println("  /stream on|off  — 切换流式/非流式模式")
	fmt.Println("  /help           — 显示此帮助")
	fmt.Println("  Ctrl-C          — 中断在途轮次（历史回滚）")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

// AICode End
