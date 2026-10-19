package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/xiaolazhu/vrc_asset_box/internal/dto"
)

type Progress func(stage string, completed, total int, cancellable bool)

type entry struct {
	snapshot dto.JobSnapshot
	cancel   context.CancelFunc
	finished time.Time
}

type Manager struct {
	mu      sync.Mutex
	entries map[string]*entry
}

func New() *Manager {
	return &Manager{entries: map[string]*entry{}}
}

func (m *Manager) Start(parent context.Context, kind string, work func(context.Context, Progress) (any, error)) dto.JobRef {
	ctx, cancel := context.WithCancel(parent)
	id := uuid.NewString()
	m.mu.Lock()
	for entryID, existingEntry := range m.entries {
		if !existingEntry.finished.IsZero() && time.Since(existingEntry.finished) > 30*time.Minute {
			delete(m.entries, entryID)
		}
	}

	jobEntry := &entry{snapshot: dto.JobSnapshot{ID: id, Kind: kind, Status: "running", Stage: "Preparing", Cancellable: true}, cancel: cancel}
	m.entries[id] = jobEntry
	m.mu.Unlock()
	go func() {
		defer cancel()

		result, err := work(ctx, func(stage string, completed, total int, cancellable bool) {
			m.mu.Lock()
			defer m.mu.Unlock()

			jobEntry.snapshot.Stage = stage
			jobEntry.snapshot.Completed = completed
			jobEntry.snapshot.Total = total
			jobEntry.snapshot.Cancellable = cancellable
		})
		m.mu.Lock()
		defer m.mu.Unlock()

		wasCancellable := jobEntry.snapshot.Cancellable
		jobEntry.snapshot.Cancellable = false
		jobEntry.finished = time.Now()
		if errors.Is(err, context.Canceled) || (ctx.Err() != nil && wasCancellable) {
			jobEntry.snapshot.Status = "cancelled"
			jobEntry.snapshot.Stage = "Canceled"

			return
		}

		if err != nil {
			jobEntry.snapshot.Status = "failed"
			jobEntry.snapshot.ErrorJSON = err.Error()
			jobEntry.snapshot.Stage = "Operation failed"

			return
		}

		resultJSON, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			jobEntry.snapshot.Status = "failed"
			jobEntry.snapshot.ErrorJSON = marshalErr.Error()

			return
		}

		jobEntry.snapshot.Status = "succeeded"
		jobEntry.snapshot.Stage = "Complete"
		// 任务成功后补齐最后一步；失败和取消保留最后上报的实际进度。
		jobEntry.snapshot.Completed = jobEntry.snapshot.Total
		jobEntry.snapshot.ResultJSON = string(resultJSON)
	}()

	return dto.JobRef{ID: id}
}

func (m *Manager) Get(id string) (dto.JobSnapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if e := m.entries[id]; e != nil {
		return e.snapshot, nil
	}

	return dto.JobSnapshot{}, dto.Error("NOT_FOUND", "Task not found or expired")
}

func (m *Manager) Cancel(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	e := m.entries[id]
	if e == nil {
		return dto.Error("NOT_FOUND", "Task not found")
	}

	if e.snapshot.Status == "running" {
		if !e.snapshot.Cancellable {
			return dto.Error("BUSY", "Saving. Please wait.")
		}

		e.cancel()
	}

	return nil
}
