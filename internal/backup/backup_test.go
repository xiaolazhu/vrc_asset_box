package backup_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/xiaolazhu/vrc_asset_box/internal/backup"
	"github.com/xiaolazhu/vrc_asset_box/internal/database"
)

func TestSnapshotRestoresLibraryAndRejectsDamagedArchive(t *testing.T) {
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "library")
	db, err := database.Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`INSERT INTO assets(id,name,name_key,created_at,updated_at) VALUES('kept','保留资料','保留资料','2026','2026')`)
	if err != nil {
		t.Fatal(err)
	}

	s := backup.New(dir, db)
	file := filepath.Join(t.TempDir(), "backup.zip")
	if _, err = s.Create(ctx, file, "manual"); err != nil {
		t.Fatal(err)
	}

	if _, err = s.Inspect(ctx, file); err != nil {
		t.Fatal(err)
	}

	if _, err = db.Exec(`DELETE FROM assets`); err != nil {
		t.Fatal(err)
	}

	restored, err := s.Restore(ctx, file)
	if err != nil {
		t.Fatal(err)
	}

	defer restored.Close()

	var name string
	if err = restored.QueryRow(`SELECT name FROM assets WHERE id='kept'`).Scan(&name); err != nil || name != "保留资料" {
		t.Fatalf("%q %v", name, err)
	}

	bad := filepath.Join(t.TempDir(), "bad.zip")
	os.WriteFile(bad, []byte("bad"), 0600)

	s = backup.New(dir, restored)
	if _, err = s.Restore(ctx, bad); err == nil {
		t.Fatal("bad backup accepted")
	}

	if err = restored.QueryRow(`SELECT name FROM assets WHERE id='kept'`).Scan(&name); err != nil {
		t.Fatal("bad restore damaged live db", err)
	}
}
