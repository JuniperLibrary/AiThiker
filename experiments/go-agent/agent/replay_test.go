// AICode Begin
package agent

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
)

// TestReplayFixtures 原始字节重放（record & replay，SPEC D7）。
// testdata/sse/*.txt 是固化的 SSE 字节序列——任何人可直接检视，
// 不经作者转述；CI 中离线重放，行为不随时间漂移。
//
// CRLF 变体由 LF fixture 转换生成（避免二进制字节进入仓库）。
func TestReplayFixtures(t *testing.T) {
	cases := []struct {
		name        string
		file        string
		crlf        bool // 把 fixture 的行尾转换为 \r\n 再重放
		wantReply   string
		wantFinish  string
		wantClass   string // 期望错误类别；"" = 期望成功
	}{
		{name: "标准流", file: "basic.txt", wantReply: "你好，世界", wantFinish: "stop"},
		{name: "data 后无空格", file: "no_space.txt", wantReply: "无空格", wantFinish: "stop"},
		{name: "CRLF 行终止", file: "basic.txt", crlf: true, wantReply: "你好，世界", wantFinish: "stop"},
		{name: "多行 data 连接", file: "multiline.txt", wantReply: "多行 data 连接", wantFinish: "stop"},
		{name: "注释与元字段", file: "comments.txt", wantReply: "注释与元字段", wantFinish: "stop"},
		{name: "流内 error 对象", file: "error_chunk.txt", wantClass: ClassServer},
		{name: "无 finish/[DONE] 截断", file: "truncated_no_done.txt", wantClass: ClassNetwork},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := os.ReadFile("testdata/sse/" + tc.file)
			if err != nil {
				t.Fatalf("读取 fixture: %v", err)
			}
			body := string(raw)
			if tc.crlf {
				body = strings.ReplaceAll(body, "\n", "\r\n")
			}

			srv := newMock(t, func(_ int, w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, body)
			})
			a := newTestAgent(srv.URL)

			var chunks []string
			result, err := a.ChatStream(context.Background(), "重放轮", func(s string) error {
				chunks = append(chunks, s)
				return nil
			})

			if tc.wantClass == "" {
				if err != nil {
					t.Fatalf("应成功: %v", err)
				}
				if result.Reply != tc.wantReply {
					t.Fatalf("回复不符: 期望 %q，实际 %q", tc.wantReply, result.Reply)
				}
				if result.FinishReason != tc.wantFinish {
					t.Fatalf("finish_reason 不符: 期望 %q，实际 %q", tc.wantFinish, result.FinishReason)
				}
				if strings.Join(chunks, "") != tc.wantReply {
					t.Fatalf("emit 拼接不符: %q", strings.Join(chunks, ""))
				}
				assertInvariant(t, a)
				return
			}

			if ClassOf(err) != tc.wantClass {
				t.Fatalf("错误类别不符: 期望 %s，实际 %s (err=%v)", tc.wantClass, ClassOf(err), err)
			}
			if result.Outcome != OutcomeRolledBack {
				t.Fatalf("结局应为 rolled_back，实际: %s", result.Outcome)
			}
			assertInvariant(t, a)
		})
	}
}

// TestMeasurementMock 离线测量报告（SPEC D6 的离线部分）：
// N=20 轮对 mock 的 TTFT/总时长分布；真实网关测量见 VERIFICATION 待核。
func TestMeasurementMock(t *testing.T) {
	if testing.Short() {
		t.Skip("short 模式跳过测量")
	}
	srv := newMock(t, hOKStream("这是一段用于测量的标准回复文本。"))
	a := newTestAgent(srv.URL)

	var ttfts, durations []int64
	for i := 0; i < 20; i++ {
		result, err := a.ChatStream(context.Background(), fmt.Sprintf("测量轮 %d", i), func(string) error { return nil })
		if err != nil {
			t.Fatalf("轮次 %d 失败: %v", i, err)
		}
		ttfts = append(ttfts, result.TTFTMs)
		durations = append(durations, result.DurationMs)
	}

	t.Logf("mock 测量（N=20，仅流程开销，非真实模型延迟）: "+
		"ttft 范围 %v, 总时长范围 %v, 历史长度 %d",
		[]int64{min64(ttfts), max64(ttfts)},
		[]int64{min64(durations), max64(durations)},
		len(a.GetHistory()))
}

func min64(xs []int64) int64 {
	m := xs[0]
	for _, x := range xs {
		if x < m {
			m = x
		}
	}
	return m
}

func max64(xs []int64) int64 {
	m := xs[0]
	for _, x := range xs {
		if x > m {
			m = x
		}
	}
	return m
}

// AICode End
