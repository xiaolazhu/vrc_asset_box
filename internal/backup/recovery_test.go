package backup

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/xiaolazhu/vrc_asset_box/internal/database"
)

func library(t *testing.T) (*Service, string) {
	t.Helper()

	root := filepath.Join(t.TempDir(), "library")
	db, err := database.Open(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}

	s := New(root, db)
	t.Cleanup(func() {
		if s.DB != nil {
			s.DB.Close()
		}
	})
	if _, err = db.Exec(`INSERT INTO assets(id,name,name_key,created_at,updated_at) VALUES('kept','kept','kept','2026','2026')`); err != nil {
		t.Fatal(err)
	}

	file := filepath.Join(t.TempDir(), "backup.zip")
	if _, err = s.Create(context.Background(), file, "manual"); err != nil {
		t.Fatal(err)
	}

	return s, file
}

func TestRepeatedRestoreRebindsServiceAndPreservesExternalFolder(t *testing.T) {
	s, file := library(t)
	external := filepath.Join(t.TempDir(), "asset.txt")
	if err := os.WriteFile(external, []byte("external asset"), 0600); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 2; i++ {
		db, err := s.Restore(context.Background(), file)
		if err != nil {
			t.Fatalf("restore %d: %v", i, err)
		}

		if s.DB != db {
			db.Close()
			t.Fatal("service still owns closed generation")
		}

		var n int
		if err = db.QueryRow("SELECT count(*) FROM assets").Scan(&n); err != nil || n != 1 {
			t.Fatalf("count=%d err=%v", n, err)
		}
	}

	if b, err := os.ReadFile(external); err != nil || string(b) != "external asset" {
		t.Fatalf("external asset changed: %v", err)
	}
}

func writeDatabaseArchive(t *testing.T, dbPath, destination string) {
	t.Helper()

	b, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	sum := sha256.Sum256(b)
	m := manifest{
		Version:       1,
		SchemaVersion: 1,
		AssetCount:    1,
		Hashes:        map[string]string{"library.db": hex.EncodeToString(sum[:])},
	}
	f, err := os.Create(destination)
	if err != nil {
		t.Fatal(err)
	}

	z := zip.NewWriter(f)
	w, err := z.Create("library.db")
	if err != nil {
		t.Fatal(err)
	}

	if _, err = w.Write(b); err != nil {
		t.Fatal(err)
	}

	w, err = z.Create("manifest.json")
	if err != nil {
		t.Fatal(err)
	}

	if err = json.NewEncoder(w).Encode(m); err != nil {
		t.Fatal(err)
	}

	if err = z.Close(); err != nil {
		t.Fatal(err)
	}

	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestInspectionRejectsValidHashWithIncompleteSchemaOrForeignKeys(t *testing.T) {
	for _, kind := range []string{"incomplete", "foreign key"} {
		t.Run(kind, func(t *testing.T) {
			s, _ := library(t)
			badRoot := t.TempDir()

			var db *sql.DB
			var err error
			if kind == "incomplete" {
				db, err = sql.Open("sqlite", filepath.Join(badRoot, "library.db"))
				if err != nil {
					t.Fatal(err)
				}

				_, err = db.Exec("CREATE TABLE assets(id TEXT); PRAGMA user_version=1")
			} else {
				db, err = database.Open(context.Background(), badRoot)
				if err != nil {
					t.Fatal(err)
				}

				_, err = db.Exec("PRAGMA foreign_keys=OFF; INSERT INTO assets(id,name,name_key,category_id,created_at,updated_at) VALUES('bad','bad','bad','missing','2026','2026')")
			}

			if err != nil {
				t.Fatal(err)
			}

			if err = db.Close(); err != nil {
				t.Fatal(err)
			}

			file := filepath.Join(t.TempDir(), "invalid.zip")
			writeDatabaseArchive(t, filepath.Join(badRoot, "library.db"), file)
			if _, err = s.Inspect(context.Background(), file); err == nil {
				t.Fatal("invalid database passed inspection")
			}

			live, err := s.Restore(context.Background(), file)
			if err == nil {
				t.Fatal("invalid database restored")
			}

			if live != s.DB || live == nil {
				t.Fatal("failure did not return live database")
			}

			if err = live.Ping(); err != nil {
				t.Fatal("failure closed original database", err)
			}
		})
	}
}

func TestBackupIncludesOnlyReferencedMedia(t *testing.T) {
	s, _ := library(t)
	if _, err := s.DB.Exec(`INSERT INTO media VALUES('orphan','hash','media/missing.png','media/missing-thumb.png','image/png',1,1,'2026')`); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Create(context.Background(), filepath.Join(t.TempDir(), "only-referenced.zip"), "manual"); err != nil {
		t.Fatal("unreferenced missing media prevented backup", err)
	}
}

func TestStartupRecoveryAlwaysSelectsCompleteOldGeneration(t *testing.T) {
	for _, phase := range []string{"prepared", "old moved", "new installed"} {
		t.Run(phase, func(t *testing.T) {
			s, file := library(t)
			root := s.Root
			previous := root + ".previous-test"
			stage := root + ".restore-test"
			if err := os.MkdirAll(stage, 0700); err != nil {
				t.Fatal(err)
			}

			if _, _, err := extract(context.Background(), file, stage); err != nil {
				t.Fatal(err)
			}

			if _, err := s.DB.Exec("UPDATE assets SET name='current generation'"); err != nil {
				t.Fatal(err)
			}

			if err := s.DB.Close(); err != nil {
				t.Fatal(err)
			}

			s.DB = nil
			journal := `{"version":1,"stage":` + quoteJSON(stage) + `,"previous":` + quoteJSON(previous) + `}`
			if err := os.WriteFile(root+".recovery.json", []byte(journal), 0600); err != nil {
				t.Fatal(err)
			}

			if phase != "prepared" {
				if err := os.Rename(root, previous); err != nil {
					t.Fatal(err)
				}
			}

			if phase == "new installed" {
				if err := os.Rename(stage, root); err != nil {
					t.Fatal(err)
				}
			}

			if err := Recover(root); err != nil {
				t.Fatal(err)
			}

			var err error

			s.DB, err = database.Open(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}

			var name string
			if err = s.DB.QueryRow("SELECT name FROM assets WHERE id='kept'").Scan(&name); err != nil || name != "current generation" {
				t.Fatalf("mixed or wrong generation: %q %v", name, err)
			}

			if err = Recover(root); err != nil {
				t.Fatal("recovery was not idempotent", err)
			}
		})
	}
}

func quoteJSON(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func TestRecoveryRefusesUnsafeJournalWithoutCreatingLibrary(t *testing.T) {
	root := filepath.Join(t.TempDir(), "library")
	if err := os.WriteFile(root+".recovery.json", []byte(`{"version":1,"stage":"C:/external","previous":"C:/external2"}`), 0600); err != nil {
		t.Fatal(err)
	}

	if err := Recover(root); err == nil {
		t.Fatal("unsafe journal accepted")
	}

	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatal("recovery created an empty library")
	}
}

func TestRestoreReopensOriginalAfterDirectoryRenameFails(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows directory handle denies generation rename")
	}

	s, file := library(t)
	handle, err := os.Open(s.Root)
	if err != nil {
		t.Fatal(err)
	}

	defer handle.Close()

	oldDB := s.DB
	live, err := s.Restore(context.Background(), file)
	if err == nil {
		t.Fatal("restore succeeded despite Windows directory lock")
	}

	if live == nil || live != s.DB {
		t.Fatal("rollback did not return live database", err)
	}

	if live == oldDB {
		t.Fatal("rollback returned the closed original connection")
	}

	var count int
	if e := live.QueryRow("SELECT count(*) FROM assets").Scan(&count); e != nil || count != 1 {
		t.Fatalf("rollback lost original data: count=%d err=%v", count, e)
	}
}

func TestRecoveryKeepsCurrentWhenOldGenerationIsInvalid(t *testing.T) {
	s, _ := library(t)
	root := s.Root
	if err := s.DB.Close(); err != nil {
		t.Fatal(err)
	}

	s.DB = nil
	previous := root + ".previous-bad"
	if err := os.Mkdir(previous, 0700); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(previous, "library.db"), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}

	journal := `{"version":1,"stage":` + quoteJSON(root+".restore-interrupted") + `,"previous":` + quoteJSON(previous) + `}`
	if err := os.WriteFile(root+".recovery.json", []byte(journal), 0600); err != nil {
		t.Fatal(err)
	}

	if err := Recover(root); err == nil {
		t.Fatal("invalid old generation accepted")
	}

	var err error

	s.DB, err = database.Open(context.Background(), root)
	if err != nil {
		t.Fatal("current generation was moved despite invalid old copy", err)
	}

	var count int
	if err = s.DB.QueryRow("SELECT count(*) FROM assets").Scan(&count); err != nil || count != 1 {
		t.Fatalf("current generation damaged: count=%d err=%v", count, err)
	}

	if _, err = os.Stat(root + ".recovery.json"); err != nil {
		t.Fatal("failed recovery discarded journal", err)
	}
}
