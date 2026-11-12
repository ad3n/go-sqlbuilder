//go:build (!amd64 && !arm64) || purego || race

package sqlbuilder

func encodeHex(dst, src []byte) {
	encodeHexGo(dst, src)
}
