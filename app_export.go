package main

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/xiaolazhu/vrc_asset_box/internal/dto"
	"github.com/xiaolazhu/vrc_asset_box/internal/excel"
	"github.com/xiaolazhu/vrc_asset_box/internal/jobs"
)

func (a *App) StartExcelExport() (dto.JobRef, error) {
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title: a.dialogText("exportExcel"), DefaultFilename: "VRCAssetBox.xlsx",
		Filters: []runtime.FileFilter{{DisplayName: a.dialogText("workbooks"), Pattern: "*.xlsx"}},
	})
	if err != nil || path == "" {
		return dto.JobRef{}, err
	}
	if filepath.Ext(path) == "" {
		path += ".xlsx"
	}
	if !strings.EqualFold(filepath.Ext(path), ".xlsx") {
		return dto.JobRef{}, dto.Error("VALIDATION", "Please choose an .xlsx file")
	}
	if err = a.write(); err != nil {
		return dto.JobRef{}, err
	}

	return a.jobs.Start(a.ctx, "excelExport", func(ctx context.Context, progress jobs.Progress) (any, error) {
		defer a.gate.Unlock()
		return path, a.exportExcel(ctx, path, progress)
	}), nil
}

// 调用期间持有维护锁，保证资料、标签和封面来自同一资料库版本。
func (a *App) exportExcel(ctx context.Context, path string, progress jobs.Progress) error {
	progress("Export Excel", 0, 0, true)
	query := defaultPreferences().Query
	query.Limit = 1048576
	page, err := a.catalog.List(ctx, query)
	if err != nil {
		return err
	}
	if page.Total > 1048575 {
		return dto.Error("VALIDATION", "The asset count exceeds the Excel worksheet row limit")
	}
	categories, err := a.catalog.ListCategories(ctx)
	if err != nil {
		return err
	}
	names := map[string]string{"": "Uncategorized"}
	for _, category := range categories {
		names[category.ID] = category.Name
	}
	rows := make([]excel.ExportRow, 0, len(page.Items))
	for _, asset := range page.Items {
		row := excel.ExportRow{Asset: asset, Category: names[asset.CategoryID]}
		if asset.CoverID != "" {
			var thumbnail string
			if err := a.db.QueryRowContext(ctx, "SELECT thumbnail_path FROM media WHERE id=?", asset.CoverID).Scan(&thumbnail); err != nil {
				return err
			}
			row.CoverPath = filepath.Join(a.root, thumbnail)
		}
		rows = append(rows, row)
	}

	return excel.Export(ctx, path, rows, func(done, total int) {
		progress("Export Excel", done, total, true)
	})
}
