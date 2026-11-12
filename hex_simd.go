//go:build (amd64 || arm64) && !purego && !race

package sqlbuilder

func encodeHex(dst, src []byte) {
	for len(src) >= 16 {
		n := min(len(src)&^15, 64<<10)
		encodeHexBlocks(dst[:2*n], src[:n])
		dst = dst[2*n:]
		src = src[n:]
	}

	encodeHexGo(dst, src)
}

//go:noescape
func encodeHexBlocks(dst, src []byte)
