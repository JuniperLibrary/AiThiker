// AICode Begin
package agent

import (
	"bufio"
	"errors"
	"io"
	"strings"
)

// maxSSELineBytes 是单行上限：超出返回显式协议错误，
// 而不是让 bufio 默认 64KB 限制以难懂的方式杀掉整个流。
const maxSSELineBytes = 4 << 20 // 4 MiB

// errSSELineTooLong 表示单行超出 maxSSELineBytes。
var errSSELineTooLong = errors.New("SSE 行超出最大长度")

// parseSSE 按 WHATWG EventSource 规范的子集解析 SSE 流，对每个事件调用 emit。
//
// 支持：
//   - 行终止 \n 与 \r\n（bufio.ReadLine 语义）
//   - ":" 开头的注释行，忽略
//   - "data:" 后可带一个可选空格（"data:x" 与 "data: x" 等价）
//   - 同一事件的多条 data 行以 \n 连接
//   - 无 data 行的事件不派发（只有注释/event/id/retry 的块）
//   - 末行无换行符时按规范丢弃残留（EOF 丢弃 pending data）
//
// 范围外（见 VERIFICATION 符合性矩阵）：单独 \r 作为行终止；
// event/id/retry 字段的语义（本用途只消费 data，字段按规范忽略）。
//
// emit 返回错误则中止解析并返回该错误。
// [DONE] 是 OpenAI 约定而非 SSE 规范内容，由调用方在 emit 内识别。
func parseSSE(r io.Reader, emit func(data string) error) error {
	br := bufio.NewReaderSize(r, 64*1024)
	var data []string

	flush := func() error {
		if len(data) == 0 {
			return nil // 无 data 行的事件不派发
		}
		ev := strings.Join(data, "\n")
		data = data[:0]
		return emit(ev)
	}

	for {
		line, err := readSSeLine(br)
		if err != nil {
			if err == io.EOF {
				// 规范：到达流末端时丢弃未派发的残留数据
				return nil
			}
			return err
		}
		if line == "" {
			if err := flush(); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue // 注释行
		}
		field, value := line, ""
		if i := strings.IndexByte(line, ':'); i >= 0 {
			field, value = line[:i], line[i+1:]
			value = strings.TrimPrefix(value, " ") // 规范：仅剥离一个前导空格
		}
		switch field {
		case "data":
			data = append(data, value)
		case "event", "id", "retry":
			// 本用途只消费 data 字段；其余按规范记录但忽略
		default:
			// 未知字段：按规范忽略（与浏览器行为一致）
		}
	}
}

// readSSeLine 读取一行（不含行终止符），支持 \n 与 \r\n。
// 超过 maxSSELineBytes 返回显式错误而非静默截断。
func readSSeLine(br *bufio.Reader) (string, error) {
	var sb strings.Builder
	for {
		chunk, isPrefix, err := br.ReadLine()
		if len(chunk) > 0 {
			if sb.Len()+len(chunk) > maxSSELineBytes {
				return "", errSSELineTooLong
			}
			sb.Write(chunk)
		}
		if err != nil {
			if err == io.EOF && sb.Len() > 0 {
				// 末行无换行符：按规范在 EOF 丢弃，返回 EOF
				return "", io.EOF
			}
			return "", err
		}
		if !isPrefix {
			return sb.String(), nil
		}
	}
}

// AICode End
