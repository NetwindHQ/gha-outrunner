package tart

import (
	"os"
	"testing"
)

func TestTailBuffer_UnderMax(t *testing.T) {
	tb := &tailBuffer{max: 16}
	_, _ = tb.Write([]byte("hello"))
	if got := tb.String(); got != "hello" {
		t.Fatalf("got %q, want %q", got, "hello")
	}
}

func TestTailBuffer_ExactMax(t *testing.T) {
	tb := &tailBuffer{max: 5}
	_, _ = tb.Write([]byte("abcde"))
	if got := tb.String(); got != "abcde" {
		t.Fatalf("got %q, want %q", got, "abcde")
	}
}

func TestTailBuffer_OverMax(t *testing.T) {
	tb := &tailBuffer{max: 4}
	_, _ = tb.Write([]byte("abcdef"))
	if got := tb.String(); got != "cdef" {
		t.Fatalf("got %q, want %q", got, "cdef")
	}
}

func TestTailBuffer_MultipleWrites(t *testing.T) {
	tb := &tailBuffer{max: 6}
	_, _ = tb.Write([]byte("abcd"))
	_, _ = tb.Write([]byte("efgh"))
	if got := tb.String(); got != "cdefgh" {
		t.Fatalf("got %q, want %q", got, "cdefgh")
	}
}

func TestTailBuffer_ReportsFullWriteLen(t *testing.T) {
	tb := &tailBuffer{max: 2}
	n, err := tb.Write([]byte("abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 6 {
		t.Fatalf("got n=%d, want 6", n)
	}
}

func TestStderrLog_WritesFileAndTail(t *testing.T) {
	sl, err := newStderrLog("test-vm", "vm")
	if err != nil {
		t.Fatal(err)
	}
	defer sl.CleanupFile()

	_, _ = sl.Write([]byte("line one\n"))
	_, _ = sl.Write([]byte("line two\n"))

	// Tail should contain the data
	tail := sl.Tail()
	if tail != "line one\nline two" {
		t.Fatalf("tail: got %q", tail)
	}

	// File should contain the data
	sl.Close()
	data, err := os.ReadFile(sl.Path())
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "line one\nline two\n" {
		t.Fatalf("file: got %q", string(data))
	}
}

func TestStderrLog_TailTruncates(t *testing.T) {
	sl, err := newStderrLog("test-vm", "runner")
	if err != nil {
		t.Fatal(err)
	}
	defer sl.CleanupFile()

	// Write more than maxStderrLog bytes
	big := make([]byte, maxStderrLog+500)
	for i := range big {
		big[i] = 'x'
	}
	big[len(big)-1] = '!'
	_, _ = sl.Write(big)

	tail := sl.tail.String()
	if len(tail) != maxStderrLog {
		t.Fatalf("tail len: got %d, want %d", len(tail), maxStderrLog)
	}
	if tail[len(tail)-1] != '!' {
		t.Fatal("tail should end with the last byte written")
	}

	// File should have everything
	sl.Close()
	data, err := os.ReadFile(sl.Path())
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != maxStderrLog+500 {
		t.Fatalf("file len: got %d, want %d", len(data), maxStderrLog+500)
	}
}

func TestStderrLog_CleanupRemovesFile(t *testing.T) {
	sl, err := newStderrLog("test-vm", "cleanup")
	if err != nil {
		t.Fatal(err)
	}
	path := sl.Path()
	sl.CleanupFile()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("file should be removed, got err: %v", err)
	}
}
