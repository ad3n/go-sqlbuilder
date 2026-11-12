//go:build darwin || linux

package sqlbuilder

import (
	"encoding/hex"
	"strings"
	"syscall"
	"testing"
)

func TestHexGuardPages(t *testing.T) {
	page := syscall.Getpagesize()
	alloc := func() []byte {
		data, err := syscall.Mmap(-1, 0, 3*page, syscall.PROT_NONE, syscall.MAP_ANON|syscall.MAP_PRIVATE)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := syscall.Munmap(data); err != nil {
				t.Error(err)
			}
		})

		middle := data[page : 2*page : 2*page]
		if err := syscall.Mprotect(middle, syscall.PROT_READ|syscall.PROT_WRITE); err != nil {
			t.Fatal(err)
		}

		return middle
	}
	source, target := alloc(), alloc()
	for i := range source {
		source[i] = byte(i * 37)
	}

	if err := syscall.Mprotect(source, syscall.PROT_READ); err != nil {
		t.Fatal(err)
	}

	for _, atEnd := range []bool{false, true} {
		for size := 0; size <= page/2; size++ {
			src, dst := source[:size:size], target[:2*size:2*size]
			if atEnd {
				src = source[page-size : page : page]
				dst = target[page-2*size : page : page]
			}

			encodeHex(dst, src)
			if string(dst) != strings.ToUpper(hex.EncodeToString(src)) {
				t.Fatalf("size=%d end=%t", size, atEnd)
			}
		}
	}
}
