// AICode Begin
// 多轮对话 Agent — 命令行交互入口
//
// 使用方式:
//
//	直接运行: go run main.go
//	设置环境变量: set API_BASE_URL=your_host:port && go run main.go
//
// 内置命令:
//
//	/exit, /quit    — 退出程序
//	/clear          — 清空对话历史
//	/stats          — 显示当前对话统计
//	/stream on|off  — 切换流式输出模式
//	/help           — 显示帮助信息
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/aithiker/go-agent/agent"
	"github.com/aithiker/go-agent/config"
)

func main() {
	fmt.Println("================================================")
	fmt.Println("  AiThiker Go Agent — 多轮对话命令行客户端")
	fmt.Println("================================================")
	fmt.Println()

	// 加载配置
	cfg := config.Load()
	fmt.Printf("▪ API 地址: %s\n", cfg.APIBaseURL)
	fmt.Printf("▪ 模型名称: %s\n", cfg.Model)
	fmt.Printf("▪ 流式模式: %v\n", cfg.Stream)
	fmt.Printf("▪ 对话 ID:  %s\n", cfg.ChatID)
	fmt.Println()

	// 创建 Agent 实例
	agt := agent.NewAgent(
		cfg.APIURL(),
		cfg.APIKey,
		cfg.Model,
		cfg.MaxTokens,
		cfg.Stream,
		cfg.ChatID,
	)

	streamMode := cfg.Stream
	scanner := bufio.NewScanner(os.Stdin)

	printHelp()
	fmt.Println()

	for {
		fmt.Print(">> 请输入 (输入 /help 查看命令): ")
		scanner.Scan()
		input := strings.TrimSpace(scanner.Text())

		// 处理空输入
		if input == "" {
			continue
		}

		// 处理内置命令
		if strings.HasPrefix(input, "/") {
			handleCommand(input, agt, &streamMode)
			continue
		}

		// 调用 Agent 进行对话
		if streamMode {
			chatStream(agt, input)
		} else {
			chatNonStream(agt, input)
		}
	}
}

// chatNonStream 使用非流式模式发送消息并打印完整回复。
func chatNonStream(agt *agent.Agent, input string) {
	fmt.Println()
	fmt.Println("🤖 Agent 思考中...")
	reply, err := agt.Chat(input)
	if err != nil {
		fmt.Printf("❌ 错误: %v\n", err)
		return
	}
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━ 回复 ━━━━━━━━━━━━━━━━")
	fmt.Println("🤖", reply)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
}

// chatStream 使用流式模式发送消息并逐块打印回复。
func chatStream(agt *agent.Agent, input string) {
	contentCh, err := agt.ChatStream(input)
	if err != nil {
		fmt.Printf("❌ 错误: %v\n", err)
		return
	}

	fmt.Println()
	fmt.Print("🤖 ")
	for content := range contentCh {
		fmt.Print(content)
	}
	fmt.Println()
	fmt.Println()
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
		fmt.Printf("  估算 Token: %d\n", stats.Tokens)
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
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

// AICode End
