package jobs

import (
	"context"
	"testing"
	"time"
)

func TestCancelledLateResultCannotBecomeSuccess(t *testing.T) {
	m := New()
	release := make(chan struct{})
	ref := m.Start(context.Background(), "booth", func(ctx context.Context, p Progress) (any, error) {
		<-release
		return "late result", nil
	})
	if err := m.Cancel(ref.ID); err != nil {
		t.Fatal(err)
	}

	close(release)

	deadline := time.After(time.Second)
	for {
		s, _ := m.Get(ref.ID)
		if s.Status != "running" {
			if s.Status != "cancelled" || s.ResultJSON != "" {
				t.Fatalf("cancelled request published result: %+v", s)
			}

			return
		}

		select {
		case <-deadline:
			t.Fatal("job did not finish")
		case <-time.After(time.Millisecond):
		}
	}
}
