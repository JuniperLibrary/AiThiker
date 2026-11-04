// AICode Begin
package agent

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// FuzzParseSSE SSE 解析器 fuzz（SPEC D7）。
// 不变量：任意输入不 panic、不无限循环；唯一可能的错误是
// 行超长（errSSELineTooLong）或 emit 自身返回的错误。
func FuzzParseSSE(f *testing.F) {
	seeds := []string{
		"data: hello\n\n",
		"data:hello\n\n",
		"data: a\r\ndata: b\r\n\r\n",
		": comment\ndata: x\n\n",
		"data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\ndata: [DONE]\n\n",
		"data: \n\n",
		"event: e\nid: 1\nretry: 100\ndata: x\n\n",
		"\n\n\n",
		"data: x\n",
		"weird: 1\ndata: y\n\n",
		strings.Repeat("data: y", 100),
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}

	f.Fuzz(func(t *testing.T, in []byte) {
		count := 0
		err := parseSSE(bytes.NewReader(in), func(string) error {
			count++
			if count > 1<<20 {
				return errors.New("事件数爆炸")
			}
			return nil
		})
		if err != nil && !errors.Is(err, errSSELineTooLong) && !strings.Contains(err.Error(), "事件数爆炸") {
			t.Fatalf("parseSSE 返回了非预期错误: %v", err)
		}
	})
}

// AICode End
