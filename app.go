package main

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/xiaolazhu/vrc_asset_box/internal/backup"
	"github.com/xiaolazhu/vrc_asset_box/internal/catalog"
	"github.com/xiaolazhu/vrc_asset_box/internal/database"
	"github.com/xiaolazhu/vrc_asset_box/internal/desktop"
	"github.com/xiaolazhu/vrc_asset_box/internal/dto"
	"github.com/xiaolazhu/vrc_asset_box/internal/jobs"
	"github.com/xiaolazhu/vrc_asset_box/internal/localization"
	"github.com/xiaolazhu/vrc_asset_box/internal/media"
	"github.com/xiaolazhu/vrc_asset_box/internal/migration"
	"github.com/xiaolazhu/vrc_asset_box/internal/tags"
)

// App 持有当前资料库版本。恢复操作关闭并替换数据库及其依赖服务前，
// 所有读取操作必须先释放维护锁。
type App struct {
	ctx          context.Context
	cancel       context.CancelFunc
	gate         sync.RWMutex
	root         string
	db           *sql.DB
	catalog      *catalog.Service
	tags         *tags.Service
	media        *media.Service
	backup       *backup.Service
	migration    *migration.Service
	jobs         *jobs.Manager
	startupErr   error
	inspections  map[string]backupInspection
	dailyOnce    sync.Once
	language     atomic.Value
	systemLocale atomic.Value
}

type backupInspection struct {
	path, hash string
}

func NewApp() *App {
	a := &App{jobs: jobs.New(), inspections: map[string]backupInspection{}}
	a.language.Store("system")
	a.systemLocale.Store(desktop.SystemLocale())
	return a
}

func applicationDataPath() (string, error) {
	if path := os.Getenv("VRC_ASSET_BOX_DATA_DIR"); path != "" {
		return filepath.Abs(path)
	}

	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(base, "VRCAssetBox"), nil
}

func webviewDataPath() (string, error) {
	return applicationDataPath()
}

func dataPath() (string, error) {
	base, err := applicationDataPath()
	if err != nil {
		return "", err
	}

	return filepath.Join(base, "Library"), nil
}

func (a *App) startup(ctx context.Context) {
	a.ctx, a.cancel = context.WithCancel(ctx)
	a.gate.Lock()
	defer a.gate.Unlock()

	a.root, a.startupErr = dataPath()
	if a.startupErr != nil {
		return
	}

	if a.startupErr = backup.Recover(a.root); a.startupErr != nil {
		return
	}

	language, _ := a.language.Load().(string)
	systemLocale, _ := a.systemLocale.Load().(string)
	db, err := database.OpenWithLanguage(a.ctx, a.root, localization.Resolve(language, systemLocale))
	if err != nil {
		a.startupErr = err
		return
	}

	a.bindDatabase(db)
}

func (a *App) startDailyBackup() {
	a.dailyOnce.Do(func() {
		go func() {
			a.gate.Lock()
			defer a.gate.Unlock()

			if a.ctx.Err() != nil || a.backup == nil {
				return
			}

			if err := a.backup.Automatic(a.ctx, "daily"); err != nil {
				runtime.EventsEmit(a.ctx, "library:notice", map[string]string{"message": "Automatic backup failed: " + err.Error() + ". Check disk space and permissions, then export a backup manually."})
				return
			}

			if err := a.media.Collect(a.ctx, time.Now().Add(-24*time.Hour)); err != nil {
				runtime.EventsEmit(a.ctx, "library:notice", map[string]string{"message": "Old cover cleanup is incomplete: " + err.Error()})
			}
		}()
	})
}

func (a *App) bindDatabase(db *sql.DB) {
	a.db = db
	a.catalog = catalog.New(db)
	a.tags = tags.New(db)
	a.media = media.New(a.root, db)
	a.backup = backup.New(a.root, db)
	a.migration = migration.New(db, a.media, a.backup)
	if p, err := a.preferences(); err == nil {
		a.language.Store(p.Language)
	}
}

func (a *App) shutdown(ctx context.Context) {
	if a.cancel != nil {
		a.cancel()
	}

	a.gate.Lock()
	defer a.gate.Unlock()

	if a.db != nil {
		_ = a.db.Close()
	}
}

func (a *App) read() error {
	if !a.gate.TryRLock() {
		return dto.Error("BUSY", "Library maintenance is in progress. Try again later.")
	}

	if a.startupErr != nil {
		a.gate.RUnlock()
		return a.startupErr
	}

	if a.db == nil {
		a.gate.RUnlock()
		return dto.Error("STORAGE", "Library not ready")
	}

	return nil
}

func (a *App) write() error {
	if !a.gate.TryLock() {
		return dto.Error("BUSY", "Another operation is in progress. Try again later.")
	}

	if a.startupErr != nil {
		a.gate.Unlock()
		return a.startupErr
	}

	if a.db == nil {
		a.gate.Unlock()
		return dto.Error("STORAGE", "Library not ready")
	}

	return nil
}

func (a *App) serveMedia(w http.ResponseWriter, r *http.Request) {
	a.gate.RLock()
	defer a.gate.RUnlock()

	if a.media == nil || a.startupErr != nil {
		http.Error(w, "Library temporarily unavailable", http.StatusServiceUnavailable)
		return
	}

	a.media.Handler().ServeHTTP(w, r)
}
