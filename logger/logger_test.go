package logger

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestLoggerPrefix(t *testing.T) {
	var out bytes.Buffer
	var hooked string
	l := NewLogger()
	l.SetOutput(&out)
	l.SetPrefix("mango")
	l.SetOnLog(func(format string, a ...any) { hooked = fmt.Sprintf(format, a...) })

	l.Log("search %q", "x")

	if want := `mango: search "x"`; hooked != want {
		t.Errorf("hook got %q, want %q", hooked, want)
	}
	if got := out.String(); strings.Count(got, "mango") != 1 || !strings.HasSuffix(got, "mango: search \"x\"\n") {
		t.Errorf("output %q, want the prefix once", got)
	}
}

func TestLoggerConcurrent(t *testing.T) {
	l := NewLogger()
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			l.SetOnLog(func(string, ...any) {})
			l.SetPrefix("x")
			l.Log("page %d", 1)
		})
	}
	wg.Wait()
}
