package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/xiaolazhu/vrc_asset_box/internal/booth"
	"github.com/xiaolazhu/vrc_asset_box/internal/dto"
	"github.com/xiaolazhu/vrc_asset_box/internal/jobs"
	"github.com/xiaolazhu/vrc_asset_box/internal/migration"
)

func (a *App) GetJob(id string) (dto.JobSnapshot, error) {
	return a.jobs.Get(id)
}

func (a *App) CancelJob(id string) error {
	return a.jobs.Cancel(id)
}

func (a *App) StartBooth(input, excludingAssetID string) (dto.JobRef, error) {
	id, err := booth.ParseInput(input)
	if err != nil {
		return dto.JobRef{}, err
	}

	if err = a.read(); err != nil {
		return dto.JobRef{}, err
	}

	var owner string

	err = a.db.QueryRowContext(a.ctx, "SELECT id FROM assets WHERE booth_id=? AND id<>?", id, excludingAssetID).Scan(&owner)
	if err != nil && err != sql.ErrNoRows {
		a.gate.RUnlock()
		return dto.JobRef{}, err
	}

	if owner != "" {
		a.gate.RUnlock()
		return dto.JobRef{}, &dto.AppError{
			Code:            "DUPLICATE_BOOTH",
			Message:         "This Booth product already exists",
			ExistingAssetID: owner,
		}
	}

	generation := a.db
	a.gate.RUnlock()

	return a.jobs.Start(a.ctx, "booth", func(ctx context.Context, p jobs.Progress) (any, error) {
		p("Fetching product details…", 0, 2, true)

		product, err := (booth.Fetcher{}).Fetch(ctx, id)
		if err != nil {
			return nil, err
		}

		// Booth 自动识别只回填商品名称和封面；素体标签由用户手动选择。
		draft := dto.BoothDraft{BoothID: id, Name: product.Name, Candidates: []dto.AvatarCandidate{}}
		if product.CoverURL != "" {
			p("Downloading cover…", 1, 2, true)

			b, err := booth.FetchCover(ctx, product.CoverURL)
			if err != nil {
				return nil, err
			}

			if err = a.read(); err != nil {
				return nil, err
			}

			defer a.gate.RUnlock()

			if a.db != generation {
				return nil, dto.Error("STALE_DRAFT", "Library restored. Fetch the product again.")
			}

			cover, err := a.media.StageBytes(ctx, b)
			if err != nil {
				return nil, err
			}

			draft.Cover = &cover
		}

		if err = ctx.Err(); err != nil {
			return nil, err
		}

		return draft, nil
	}), nil
}

func (a *App) StartImportPreview(req dto.ImportPreviewRequest) (dto.JobRef, error) {
	if err := a.write(); err != nil {
		return dto.JobRef{}, err
	}

	return a.jobs.Start(a.ctx, "importPreview", func(ctx context.Context, p jobs.Progress) (any, error) {
		defer a.gate.Unlock()

		return a.migration.PreviewWithProgress(ctx, req, migration.Progress(p))
	}), nil
}

func (a *App) StartImportCommit(req dto.ImportCommitRequest) (dto.JobRef, error) {
	if err := a.write(); err != nil {
		return dto.JobRef{}, err
	}

	return a.jobs.Start(a.ctx, "importCommit", func(ctx context.Context, p jobs.Progress) (any, error) {
		defer a.gate.Unlock()

		return a.migration.CommitWithProgress(ctx, req, migration.Progress(p))
	}), nil
}

func (a *App) SaveImportReport(previewID string) (string, error) {
	if err := a.read(); err != nil {
		return "", err
	}

	b, err := a.migration.Report(previewID)
	source, sourceErr := a.migration.SourcePath(previewID)
	a.gate.RUnlock()
	if err != nil {
		return "", err
	}

	if sourceErr != nil {
		return "", sourceErr
	}

	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           a.dialogText("saveReport"),
		DefaultFilename: a.dialogText("reportFilename"),
		Filters:         []runtime.FileFilter{{DisplayName: a.dialogText("csvReports"), Pattern: "*.csv"}},
	})
	if err != nil || path == "" {
		return "", err
	}

	if !strings.EqualFold(filepath.Ext(path), ".csv") {
		return "", dto.Error("VALIDATION", "Save the report as a separate .csv file. Do not overwrite the source workbook.")
	}

	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	sourceAbsolute, err := filepath.Abs(source)
	if err != nil {
		return "", err
	}

	if strings.EqualFold(absolute, sourceAbsolute) {
		return "", dto.Error("VALIDATION", "Cannot overwrite the source workbook")
	}

	if destInfo, e := os.Stat(path); e == nil {
		if sourceInfo, e := os.Stat(source); e == nil && os.SameFile(destInfo, sourceInfo) {
			return "", dto.Error("VALIDATION", "Cannot overwrite the source workbook")
		}
	}

	if err = os.WriteFile(path, b, 0600); err != nil {
		return "", err
	}

	return path, nil
}

func (a *App) DiscardImportPreview(previewID string) error {
	if err := a.write(); err != nil {
		return err
	}

	defer a.gate.Unlock()

	return a.migration.Discard(a.ctx, previewID)
}

func (a *App) StartBackup() (dto.JobRef, error) {
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           a.dialogText("exportBackup"),
		DefaultFilename: "VRCAssetBox-backup.zip",
		Filters:         []runtime.FileFilter{{DisplayName: a.dialogText("backups"), Pattern: "*.zip"}},
	})
	if err != nil || path == "" {
		return dto.JobRef{}, err
	}

	if err = a.write(); err != nil {
		return dto.JobRef{}, err
	}

	return a.jobs.Start(a.ctx, "backup", func(ctx context.Context, p jobs.Progress) (any, error) {
		defer a.gate.Unlock()

		p("Backing up library…", 0, 1, true)

		return a.backup.Create(ctx, path, "manual")
	}), nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}

	defer f.Close()

	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func (a *App) InspectBackup() (dto.BackupInfo, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: a.dialogText("selectBackup"), Filters: []runtime.FileFilter{{DisplayName: a.dialogText("backups"), Pattern: "*.zip"}}})
	if err != nil || path == "" {
		return dto.BackupInfo{}, err
	}

	if err = a.write(); err != nil {
		return dto.BackupInfo{}, err
	}

	defer a.gate.Unlock()

	before, err := hashFile(path)
	if err != nil {
		return dto.BackupInfo{}, err
	}

	info, err := a.backup.Inspect(a.ctx, path)
	if err != nil {
		return info, err
	}

	after, err := hashFile(path)
	if err != nil {
		return info, err
	}

	if before != after {
		return info, dto.Error("STALE_DRAFT", "Backup file has changed. Select it again.")
	}

	id := uuid.NewString()
	a.inspections = map[string]backupInspection{id: {path: path, hash: after}}
	info.InspectionID = id

	return info, nil
}

func (a *App) StartRestore(inspectionID string) (dto.JobRef, error) {
	if err := a.write(); err != nil {
		return dto.JobRef{}, err
	}

	inspection, ok := a.inspections[inspectionID]
	if !ok {
		a.gate.Unlock()
		return dto.JobRef{}, dto.Error("NOT_FOUND", "Backup preview expired. Select it again.")
	}

	return a.jobs.Start(a.ctx, "restore", func(ctx context.Context, p jobs.Progress) (any, error) {
		defer a.gate.Unlock()

		p("Validating backup…", 0, 2, true)

		hash, err := hashFile(inspection.path)
		if err != nil {
			return nil, err
		}

		if hash != inspection.hash {
			return nil, dto.Error("STALE_DRAFT", "Backup file has changed. Generate a new preview.")
		}

		if err = ctx.Err(); err != nil {
			return nil, err
		}

		p("Restoring library. Do not close the app.", 1, 2, false)

		db, err := a.backup.Restore(ctx, inspection.path)
		if db != nil && db != a.db {
			a.bindDatabase(db)
		} else if db == nil {
			a.startupErr = dto.Error("STORAGE", "Library restoration failed. Restart the app to check recovery.")
		}

		a.inspections = map[string]backupInspection{}
		if err != nil {
			return nil, err
		}

		return dto.WorkspaceInfo{DataPath: filepath.Clean(a.root), SchemaVersion: 1}, nil
	}), nil
}
