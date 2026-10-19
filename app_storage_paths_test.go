package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/xiaolazhu/vrc_asset_box/internal/dto"
)

func TestLibraryRestoreWithWebViewDirectoryInUse(t *testing.T) {
	a := testApp(t)
	base, err := webviewDataPath()
	if err != nil {
		t.Fatal(err)
	}
	if a.root != filepath.Join(base, "Library") {
		t.Fatalf("library still shares WebView root: %q", a.root)
	}

	// 模拟 WebView2 占用外层目录，恢复只需切换 Library 子目录。
	handle, err := os.Open(base)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()

	saved, err := a.SaveAsset(dto.AssetDraft{Name: "保留素材"})
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "backup.zip")
	if _, err = a.backup.Create(a.ctx, file, "manual"); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 2; i++ {
		live, err := a.backup.Restore(a.ctx, file)
		if err != nil {
			t.Fatalf("restore with locked WebView parent: %v", err)
		}
		a.bindDatabase(live)

		var name string
		if err = a.db.QueryRow("SELECT name FROM assets WHERE id = ?", saved.ID).Scan(&name); err != nil || name != "保留素材" {
			t.Fatalf("restored asset: %q %v", name, err)
		}
	}
}
