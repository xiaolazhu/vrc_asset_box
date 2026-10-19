package jobs

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestTerminalProgress(t *testing.T) {
	for _, tc := range []struct {
		name      string
		workError error
		status    string
		completed int
	}{
		{"success", nil, "succeeded", 2},
		{"failure", errors.New("backup failed"), "failed", 1},
		{"cancellation", context.Canceled, "cancelled", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := New()
			ref := m.Start(context.Background(), "backup", func(ctx context.Context, p Progress) (any, error) {
				p("Backing up library…", 1, 2, true)
				return "result", tc.workError
			})

			deadline := time.Now().Add(time.Second)
			for time.Now().Before(deadline) {
				snapshot, err := m.Get(ref.ID)
				if err != nil {
					t.Fatal(err)
				}
				if snapshot.Status != "running" {
					if snapshot.Status != tc.status || snapshot.Completed != tc.completed || snapshot.Total != 2 || snapshot.Cancellable {
						t.Fatalf("unexpected terminal progress: %+v", snapshot)
					}
					return
				}
				time.Sleep(time.Millisecond)
			}
			t.Fatal("job did not finish")
		})
	}
}
