package sqlbuilder

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

const maxCompileBufferCapacity = 64 << 10

type compileBuffer struct {
	large strings.Builder
	data  []byte
}

func (buf *compileBuffer) WriteString(s string) {
	if buf.large.Len() > 0 {
		buf.large.WriteString(s)
		return
	}

	if len(s)+len(buf.data) > maxCompileBufferCapacity {
		buf.large.Grow(len(buf.data) + len(s))
		buf.large.Write(buf.data)
		buf.large.WriteString(s)
		buf.data = buf.data[:0]
		return
	}

	buf.reserve(len(s))
	buf.data = append(buf.data, s...)
}

func (buf *compileBuffer) reserve(size int) {
	if cap(buf.data)-len(buf.data) >= size {
		return
	}

	capacity := min(maxCompileBufferCapacity, max(len(buf.data)+size, 2*cap(buf.data)))
	data := make([]byte, len(buf.data), capacity)
	copy(data, buf.data)
	buf.data = data
}

func (buf *compileBuffer) WriteRune(r rune) {
	if r >= 0 && r < utf8.RuneSelf && buf.large.Len() == 0 && len(buf.data) < maxCompileBufferCapacity {
		buf.reserve(1)
		buf.data = append(buf.data, byte(r))
		return
	}

	var scratch [utf8.UTFMax]byte
	buf.WriteString(string(utf8.AppendRune(scratch[:0], r)))
}

func (buf *compileBuffer) WriteInt(value int) {
	if buf.large.Len() == 0 && len(buf.data)+20 <= maxCompileBufferCapacity {
		buf.reserve(20)
		buf.data = strconv.AppendInt(buf.data, int64(value), 10)
		return
	}

	var scratch [20]byte
	buf.WriteString(string(strconv.AppendInt(scratch[:0], int64(value), 10)))
}

func (buf *compileBuffer) String() string {
	if buf.large.Len() > 0 {
		return buf.large.String()
	}

	return string(buf.data)
}

func (buf *compileBuffer) Cap() int {
	return cap(buf.data)
}

func (buf *compileBuffer) Reset() {
	buf.large.Reset()
	if cap(buf.data) > maxCompileBufferCapacity {
		buf.data = nil
		return
	}

	buf.data = buf.data[:0]
}
