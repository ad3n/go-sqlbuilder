package sqlbuilder

import "slices"

var hexDigits = [16]byte{'0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 'A', 'B', 'C', 'D', 'E', 'F'}

func appendHex(buf, v []byte) []byte {
	if len(v) < 64 {
		for _, b := range v {
			buf = append(buf, hexDigits[b>>4], hexDigits[b&15])
		}

		return buf
	}

	if len(v) > (int(^uint(0)>>1)-len(buf))/2 {
		panic("sqlbuilder: hex output too large")
	}

	start := len(buf)
	buf = slices.Grow(buf, 2*len(v))
	buf = buf[:start+2*len(v)]
	encodeHex(buf[start:], v)
	return buf
}

func encodeHexGo(dst, src []byte) {
	for i, b := range src {
		dst[2*i] = hexDigits[b>>4]
		dst[2*i+1] = hexDigits[b&15]
	}
}
