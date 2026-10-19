package jobs_test

import (
	"context"
	"testing"
	"time"

	"github.com/xiaolazhu/vrc_asset_box/internal/jobs"
)

func TestCancellationProducesTerminalState(t *testing.T) {
	m := jobs.New()
	id := m.Start(context.Background(), "booth", func(ctx context.Context, p jobs.Progress) (any, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	if err := m.Cancel(id.ID); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 100; i++ {
		s, err := m.Get(id.ID)
		if err != nil {
			t.Fatal(err)
		}

		if s.Status == "cancelled" {
			return
		}

		time.Sleep(time.Millisecond)
	}

	t.Fatal("cancellation never completed")
}
