package main

import (
	"path/filepath"
	"strings"

	"github.com/xiaolazhu/vrc_asset_box/internal/desktop"
	"github.com/xiaolazhu/vrc_asset_box/internal/dto"
	"github.com/xiaolazhu/vrc_asset_box/internal/media"
)

func (a *App) ListAssets(q dto.AssetQuery) (dto.AssetPage, error) {
	if err := a.read(); err != nil {
		return dto.AssetPage{}, err
	}

	defer a.startDailyBackup()
	defer a.gate.RUnlock()

	return a.catalog.List(a.ctx, q)
}

func (a *App) GetAsset(id string) (dto.Asset, error) {
	if err := a.read(); err != nil {
		return dto.Asset{}, err
	}

	defer a.gate.RUnlock()

	return a.catalog.Get(a.ctx, id)
}

func (a *App) SaveAsset(d dto.AssetDraft) (dto.Asset, error) {
	if err := a.write(); err != nil {
		return dto.Asset{}, err
	}

	defer a.gate.Unlock()

	d.FolderPath = strings.TrimSpace(d.FolderPath)
	if d.FolderPath != "" {
		if !filepath.IsAbs(d.FolderPath) {
			return dto.Asset{}, dto.Error("VALIDATION", "Linked directory must use a full absolute path")
		}

		d.FolderPath = filepath.Clean(d.FolderPath)
	}

	if d.FolderPath != "" && !d.AllowOfflineFolder {
		if err := desktop.CheckFolder(a.ctx, d.FolderPath); err != nil {
			return dto.Asset{}, err
		}
	}

	token := d.CoverToken

	var prepared *media.PreparedCover
	if token != "" {
		p, err := a.media.PrepareCover(a.ctx, token)
		if err != nil {
			return dto.Asset{}, err
		}

		prepared = &p
		d.CoverToken = ""
	}

	tx, err := a.db.BeginTx(a.ctx, nil)
	if err != nil {
		return dto.Asset{}, err
	}

	defer tx.Rollback()

	if prepared != nil {
		d.CoverID, err = prepared.InsertInTx(a.ctx, tx)
		if err != nil {
			return dto.Asset{}, err
		}
	}

	id, err := a.catalog.SaveInTx(a.ctx, tx, d)
	if err != nil {
		return dto.Asset{}, err
	}

	if err = tx.Commit(); err != nil {
		return dto.Asset{}, err
	}

	if token != "" {
		_ = a.media.Discard(a.ctx, token)
	}

	return a.catalog.Get(a.ctx, id)
}

func (a *App) BatchAssets(req dto.BatchRequest) error {
	if err := a.write(); err != nil {
		return err
	}

	defer a.gate.Unlock()

	if req.Operation == "delete" {
		if err := a.backup.Automatic(a.ctx, "critical"); err != nil {
			return &dto.AppError{
				Code:    "BACKUP_FAILED",
				Message: "Backup before deletion failed. Records were not deleted.",
				Details: err.Error(),
			}
		}
	}

	return a.catalog.Batch(a.ctx, req)
}

func (a *App) SaveCategory(d dto.CategoryDraft) (dto.Category, error) {
	if err := a.write(); err != nil {
		return dto.Category{}, err
	}

	defer a.gate.Unlock()

	return a.catalog.SaveCategory(a.ctx, d)
}

func (a *App) DeleteCategory(id, targetID string) error {
	if err := a.write(); err != nil {
		return err
	}

	defer a.gate.Unlock()

	return a.catalog.DeleteCategory(a.ctx, id, targetID)
}

func (a *App) ReorderCategories(ids []string) error {
	if err := a.write(); err != nil {
		return err
	}

	defer a.gate.Unlock()

	return a.catalog.ReorderCategories(a.ctx, ids)
}

func (a *App) ListTags(kind, keyword string) ([]dto.Tag, error) {
	if err := a.read(); err != nil {
		return nil, err
	}

	defer a.gate.RUnlock()

	return a.tags.List(a.ctx, kind, keyword)
}

func (a *App) SaveTag(d dto.TagDraft) (dto.Tag, error) {
	if err := a.write(); err != nil {
		return dto.Tag{}, err
	}

	defer a.gate.Unlock()

	return a.tags.Save(a.ctx, d)
}

func (a *App) DeleteTag(id string) error {
	if err := a.write(); err != nil {
		return err
	}

	defer a.gate.Unlock()

	return a.tags.Delete(a.ctx, id)
}

func (a *App) MergeTags(sourceID, targetID string) error {
	if err := a.write(); err != nil {
		return err
	}

	defer a.gate.Unlock()

	return a.tags.Merge(a.ctx, sourceID, targetID)
}
