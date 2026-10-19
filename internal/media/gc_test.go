package media

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xiaolazhu/vrc_asset_box/internal/database"
)

func TestCollectOnlyExpiredUnreferencedManagedFiles(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	db, err := database.Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}

	defer db.Close()

	mediaDir := filepath.Join(root, "media")
	if err = os.MkdirAll(mediaDir, 0700); err != nil {
		t.Fatal(err)
	}

	stale := filepath.Join(mediaDir, strings.Repeat("a", 64)+".png")
	fresh := filepath.Join(mediaDir, strings.Repeat("b", 64)+".png")
	outside := filepath.Join(root, "external.png")
	for _, p := range []string{stale, fresh, outside} {
		if err = os.WriteFile(p, []byte("test"), 0600); err != nil {
			t.Fatal(err)
		}
	}

	old := time.Now().Add(-48 * time.Hour)
	if err = os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}

	if err = New(root, db).Collect(ctx, time.Now().Add(-24*time.Hour)); err != nil {
		t.Fatal(err)
	}

	if _, err = os.Stat(stale); !os.IsNotExist(err) {
		t.Fatal("old orphan retained", err)
	}

	for _, p := range []string{fresh, outside} {
		if _, err = os.Stat(p); err != nil {
			t.Fatal("removed active/outside file", p, err)
		}
	}
}
