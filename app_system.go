package main

import (
	"encoding/base64"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/xiaolazhu/vrc_asset_box/internal/booth"
	"github.com/xiaolazhu/vrc_asset_box/internal/desktop"
	"github.com/xiaolazhu/vrc_asset_box/internal/dto"
	"github.com/xiaolazhu/vrc_asset_box/internal/media"
)

func (a *App) SelectFolder() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: a.dialogText("selectFolder")})
}

func (a *App) OpenFolder(path string) error {
	return desktop.OpenFolder(a.ctx, path)
}

func (a *App) OpenBooth(input string) error {
	id, err := booth.ParseInput(input)
	if err != nil {
		return err
	}

	runtime.BrowserOpenURL(a.ctx, "https://booth.pm/zh-cn/items/"+id)

	return nil
}

func (a *App) CopyText(text string) error {
	return runtime.ClipboardSetText(a.ctx, text)
}

func (a *App) SelectCover() (dto.MediaDraft, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: a.dialogText("selectCover"), Filters: []runtime.FileFilter{{DisplayName: a.dialogText("images"), Pattern: "*.png;*.jpg;*.jpeg;*.webp"}}})
	if err != nil || path == "" {
		return dto.MediaDraft{}, err
	}

	return a.StageCoverPath(path)
}

func (a *App) StageCoverPath(path string) (dto.MediaDraft, error) {
	if err := a.read(); err != nil {
		return dto.MediaDraft{}, err
	}

	defer a.gate.RUnlock()

	return a.media.StagePath(a.ctx, path)
}

func (a *App) StageCoverBytes(dataURL string) (dto.MediaDraft, error) {
	if len(dataURL) > media.MaxBytes*4/3+1024 {
		return dto.MediaDraft{}, dto.Error("VALIDATION", "Images must not exceed 20 MiB")
	}

	header, raw, ok := strings.Cut(dataURL, ",")
	if !ok || !strings.HasPrefix(header, "data:image/") || !strings.HasSuffix(header, ";base64") {
		return dto.MediaDraft{}, dto.Error("VALIDATION", "No valid image on the clipboard")
	}

	b, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return dto.MediaDraft{}, dto.Error("VALIDATION", "Invalid image encoding")
	}

	if err = a.read(); err != nil {
		return dto.MediaDraft{}, err
	}

	defer a.gate.RUnlock()

	return a.media.StageBytes(a.ctx, b)
}

func (a *App) DiscardCover(token string) error {
	if err := a.read(); err != nil {
		return err
	}

	defer a.gate.RUnlock()

	return a.media.Discard(a.ctx, token)
}

func (a *App) SelectWorkbook() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: a.dialogText("selectWorkbook"), Filters: []runtime.FileFilter{{DisplayName: a.dialogText("workbooks"), Pattern: "*.xlsx"}}})
}

func (a *App) OpenDataFolder() error {
	if err := a.read(); err != nil {
		return err
	}

	path := a.root
	a.gate.RUnlock()

	return desktop.OpenFolder(a.ctx, path)
}
