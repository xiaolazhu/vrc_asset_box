package desktop

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/xiaolazhu/vrc_asset_box/internal/dto"
)

var probes = make(chan struct{}, 4)

func CheckFolder(ctx context.Context, path string) error {
	if strings.TrimSpace(path) == "" || !filepath.IsAbs(path) {
		return dto.Error("VALIDATION", "Choose or enter a full folder path")
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	select {
	case probes <- struct{}{}:
	case <-ctx.Done():
		return dto.Error("PATH_UNAVAILABLE", "Directory check timed out. Check your disk or network connection.")
	}

	done := make(chan error, 1)
	go func() {
		defer func() {
			<-probes
		}()

		info, err := os.Stat(path)
		if err == nil && !info.IsDir() {
			err = dto.Error("VALIDATION", "Linked path must be a folder")
		}

		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			return &dto.AppError{Code: "PATH_UNAVAILABLE", Message: "Folder is temporarily unavailable", Details: err.Error()}
		}
		return nil
	case <-ctx.Done():
		return dto.Error("PATH_UNAVAILABLE", "Directory check timed out. Check your disk or network connection.")
	}
}

func OpenFolder(ctx context.Context, path string) error {
	if err := CheckFolder(ctx, path); err != nil {
		return err
	}

	cmd := exec.Command("explorer.exe", path)
	if err := cmd.Start(); err != nil {
		return err
	}

	return cmd.Process.Release()
}
