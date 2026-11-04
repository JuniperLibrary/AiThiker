// AICode Begin
package agent

import (
	"bytes"
	"strings"
	"testing"
)

// TestSSEConformance SSE 解析器的规范符合性用例（SPEC D2 符合性矩阵的实现）。
func TestSSEConformance(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "基本事件",
			input: "data: hello\n\n",
			want:  []string{"hello"},
		},
		{
			name:  "data 后无空格",
			input: "data:hello\n\n",
			want:  []string{"hello"},
		},
		{
			name:  "data 后只剥离一个空格",
			input: "data:  hello\n\n",
			want:  []string{" hello"},
		},
		{
			name:  "CRLF 行终止",
			input: "data: a\r\ndata: b\r\n\r\n",
			want:  []string{"a\nb"},
		},
		{
			name:  "同一事件多条 data 行以换行连接",
			input: "data: first\ndata: second\n\n",
			want:  []string{"first\nsecond"},
		},
		{
			name:  "注释行忽略",
			input: ": keep-alive\n\ndata: x\n\n",
			want:  []string{"x"},
		},
		{
			name:  "无 data 行的事件不派发",
			input: "event: ping\n\ndata: y\n\n",
			want:  []string{"y"},
		},
		{
			name:  "event/id/retry 字段忽略但事件仍派发",
			input: "event: message\nid: 42\nretry: 1000\ndata: payload\n\n",
			want:  []string{"payload"},
		},
		{
			name:  "空 data 字段派发空字符串",
			input: "data: \n\n",
			want:  []string{""},
		},
		{
			name:  "多个事件",
			input: "data: one\n\ndata: two\n\ndata: three\n\n",
			want:  []string{"one", "two", "three"},
		},
		{
			name:  "EOF 丢弃未派发的残留（规范行为）",
			input: "data: done\n\ndata: pending\n",
			want:  []string{"done"},
		},
		{
			name:  "空输入",
			input: "",
			want:  nil,
		},
		{
			name:  "[DONE] 按约定原样透传（由 API 层识别）",
			input: "data: [DONE]\n\n",
			want:  []string{"[DONE]"},
		},
		{
			name:  "未知字段忽略",
			input: "weirdfield: 123\ndata: ok\n\n",
			want:  []string{"ok"},
		},
		{
			name:  "连续空行不产生事件",
			input: "\n\n\ndata: x\n\n\n\n",
			want:  []string{"x"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			err := parseSSE(strings.NewReader(tc.input), func(d string) error {
				got = append(got, d)
				return nil
			})
			if err != nil {
				t.Fatalf("解析失败: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("事件数不符: 期望 %v，实际 %v", tc.want, got)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("事件 %d 不符: 期望 %q，实际 %q", i, tc.want[i], got[i])
				}
			}
		})
	}
}

// TestSSEEmitErrorStops emit 返回错误立即中止解析并透传。
func TestSSEEmitErrorStops(t *testing.T) {
	stop := "停止"
	err := parseSSE(strings.NewReader("data: a\n\ndata: b\n\n"), func(d string) error {
		return newSentinelError(stop)
	})
	if err == nil || !strings.Contains(err.Error(), stop) {
		t.Fatalf("emit 错误应透传，实际: %v", err)
	}
}

type sentinelError struct{ msg string }

func (e *sentinelError) Error() string { return e.msg }
func newSentinelError(msg string) error { return &sentinelError{msg: msg} }

// TestSSEReadLineBufferBoundaries 行恰好等于/超过缓冲边界的行为。
func TestSSEReadLineBoundaries(t *testing.T) {
	// 超过 64KB bufio 内部缓冲但低于 maxSSELineBytes：必须正常解析
	long := strings.Repeat("y", 100*1024)
	var got []string
	err := parseSSE(strings.NewReader("data: "+long+"\n\n"), func(d string) error {
		got = append(got, d)
		return nil
	})
	if err != nil {
		t.Fatalf("100KB 行应可解析: %v", err)
	}
	if len(got) != 1 || got[0] != long {
		t.Fatalf("长行内容不符: len=%d", len(got))
	}
}

var _ = bytes.NewReader // 保留 bytes 引用（fuzz 文件使用）

// AICode End
