package sqlbuilder

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"testing"
)

var hexResult []byte

func TestHexEncoding(t *testing.T) {
	for size := 0; size <= 1025; size++ {
		for offset := 0; offset < 16; offset++ {
			storage := make([]byte, size+offset)
			src := storage[offset:]
			for i := range src {
				src[i] = byte(i*37 + size)
			}

			want := strings.ToUpper(hex.EncodeToString(src))
			for _, spare := range []bool{false, true} {
				prefix := []byte("prefix:")
				if spare {
					prefix = make([]byte, 7, 7+2*size+16)
					copy(prefix, "prefix:")
				}

				got := appendHex(prefix, src)
				if string(got) != "prefix:"+want {
					t.Fatalf("size=%d offset=%d spare=%t: %x", size, offset, spare, got)
				}
			}

			dst := bytes.Repeat([]byte{0xcd}, 2*size+32)
			encodeHex(dst[offset:offset+2*size], src)
			if string(dst[offset:offset+2*size]) != want || !bytes.Equal(dst[:offset], bytes.Repeat([]byte{0xcd}, offset)) || !bytes.Equal(dst[offset+2*size:], bytes.Repeat([]byte{0xcd}, 32-offset)) {
				t.Fatalf("incorrect output or overwrite: size=%d offset=%d", size, offset)
			}
		}
	}
}

func FuzzHexEncoding(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte("0123456789abcdef0123456789abcdef!"))
	f.Add([]byte{0, 127, 128, 255})
	f.Fuzz(func(t *testing.T, src []byte) {
		want := "prefix:" + strings.ToUpper(hex.EncodeToString(src))
		if got := string(appendHex([]byte("prefix:"), src)); got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})
}

func BenchmarkHexEncoding(b *testing.B) {
	for _, size := range []int{0, 8, 15, 16, 32, 64, 4096, 65536} {
		src := bytes.Repeat([]byte{0xab}, size)
		for _, mode := range []string{"Original", "Scalar", "SIMD"} {
			b.Run(fmt.Sprintf("%d/%s", size, mode), func(b *testing.B) {
				dst := make([]byte, 2*size)
				b.ReportAllocs()
				b.SetBytes(int64(size))
				for b.Loop() {
					switch mode {
					case "Original":
						dst = dst[:0]
						for _, v := range src {
							dst = append(dst, hexDigits[v>>4], hexDigits[v&15])
						}
					case "Scalar":
						encodeHexGo(dst, src)
					case "SIMD":
						encodeHex(dst, src)
					}

					hexResult = dst
				}
			})
		}
	}
}

func BenchmarkHexInterpolate(b *testing.B) {
	for _, size := range []int{0, 1, 8, 15, 16, 31, 32, 63, 64, 65, 4096, 65536} {
		src := bytes.Repeat([]byte{0xab}, size)
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(size))
			for b.Loop() {
				var err error
				scanSQL, err = PostgreSQL.Interpolate("SELECT $1", []any{src})
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestHexConcurrentOwnership(t *testing.T) {
	src := bytes.Repeat([]byte{0, 127, 128, 255}, 32769)
	original := bytes.Clone(src)
	want := "prefix:" + strings.ToUpper(hex.EncodeToString(src))
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			for range 32 {
				got := appendHex([]byte("prefix:"), src)
				if string(got) != want {
					t.Error("incorrect concurrent encoding")
					return
				}
				got[7] = '!'
			}
		})
	}

	wg.Wait()
	if !bytes.Equal(src, original) {
		t.Fatal("input mutated")
	}
}

func TestHexChunkBoundaries(t *testing.T) {
	for _, size := range []int{65535, 65536, 65537, 131071, 131072, 131073} {
		src := bytes.Repeat([]byte{0xda}, size)
		got := appendHex([]byte("prefix:"), src)
		if string(got) != "prefix:"+strings.ToUpper(hex.EncodeToString(src)) {
			t.Fatalf("size=%d", size)
		}
		saved := bytes.Clone(got)
		appendHex(nil, bytes.Repeat([]byte{0xff}, size))
		if !bytes.Equal(got, saved) {
			t.Fatal("retained output changed")
		}
	}
}
