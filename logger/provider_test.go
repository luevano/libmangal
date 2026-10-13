package logger

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/luevano/libmangal/progress"
)

func TestProviderLoggerBacklog(t *testing.T) {
	var out bytes.Buffer
	pl := NewProviderLogger("mango-x", "X")
	pl.Log("early %d", 1)

	l := NewLogger()
	l.SetOutput(&out)
	pl.SetLogger(l)
	pl.Log("late")

	got := out.String()
	if !strings.Contains(got, "[backlog] mango-x: early 1\n") || !strings.HasSuffix(got, "mango-x: late\n") {
		t.Errorf("output %q", got)
	}
}

func TestProviderLoggerStatus(t *testing.T) {
	for _, tt := range []struct{ name, want string }{
		{"X", "X: wait 2s"},
		{"", "mango-x: wait 2s"},
	} {
		var got []progress.Status
		ctx := progress.WithReporter(context.Background(), func(s progress.Status) { got = append(got, s) })
		pl := NewProviderLogger("mango-x", tt.name)
		pl.Status(ctx, "wait %ds", 2)
		pl.Progress(ctx, 1, 3, "page %d", 1)

		if len(got) != 2 {
			t.Fatalf("got %d statuses, want 2", len(got))
		}
		if got[0].Source != "mango-x" || got[0].Message != tt.want || got[0].Minor {
			t.Errorf("status %+v, want message %q", got[0], tt.want)
		}
		if got[1].Done != 1 || got[1].Total != 3 || !got[1].Minor || !strings.HasSuffix(got[1].Message, ": page 1") {
			t.Errorf("progress %+v", got[1])
		}
		if len(pl.backlog) != 1 {
			t.Errorf("backlog %q, want only the status", pl.backlog)
		}
	}
}

func TestProviderLoggerNil(t *testing.T) {
	var pl *ProviderLogger
	pl.SetLogger(NewLogger())
	pl.Log("x")
	pl.Status(context.Background(), "x")
	pl.Progress(context.Background(), 0, 0, "x")
	NewProviderLogger("a", "").SetLogger(nil)
}
