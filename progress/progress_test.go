package progress

import (
	"context"
	"testing"
)

func TestReport(t *testing.T) {
	// no reporter: nothing happens
	Report(context.Background(), Status{Message: "x"})

	var got []string
	ctx := WithReporter(context.Background(), func(s Status) { got = append(got, s.String()) })
	Report(ctx, Status{Message: "AniList rate limited, retrying in 58s"})
	Report(ctx, Status{Source: "anilist", Message: "looking up AniList metadata: request", Done: 3, Total: 7})
	want := []string{"AniList rate limited, retrying in 58s", "looking up AniList metadata: request 3/7"}
	if len(got) != len(want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got %q, want %q", got[i], want[i])
		}
	}

	if (Status{Message: "x"}).IsMinor() || !(Status{Message: "x", Done: 1, Total: 2}).IsMinor() || !(Status{Minor: true}).IsMinor() {
		t.Error("IsMinor")
	}
}
