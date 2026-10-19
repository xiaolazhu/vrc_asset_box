package migration

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/xiaolazhu/vrc_asset_box/internal/backup"
	"github.com/xiaolazhu/vrc_asset_box/internal/database"
	"github.com/xiaolazhu/vrc_asset_box/internal/dto"
	"github.com/xiaolazhu/vrc_asset_box/internal/excel"
	"github.com/xiaolazhu/vrc_asset_box/internal/media"
)

func setup(t *testing.T) (*Service, *sql.DB) {
	t.Helper()

	root := t.TempDir()
	db, e := database.Open(context.Background(), root)
	if e != nil {
		t.Fatal(e)
	}

	t.Cleanup(func() {
		db.Close()
	})

	return New(db, media.New(root, db), backup.New(root, db)), db
}

func decisions(p dto.ImportPreview) []dto.ImportDecision {
	out := []dto.ImportDecision{}
	for _, r := range p.Rows {
		action := "create"
		if r.ExistingAssetID != "" {
			action = "skip"
		}

		out = append(out, dto.ImportDecision{RowKey: r.Key, Action: action, ResolvedDraft: r.Draft})
	}

	return out
}

func count(t *testing.T, db *sql.DB, table string) int {
	t.Helper()

	var n int
	if e := db.QueryRow("SELECT count(*) FROM " + table).Scan(&n); e != nil {
		t.Fatal(e)
	}

	return n
}

func TestWorkbookImportAndRepeat(t *testing.T) {
	s, db := setup(t)
	ctx := context.Background()
	path := "../../docs/VRC Assets.xlsx"
	p, e := s.Preview(ctx, dto.ImportPreviewRequest{Path: path})
	if e != nil {
		t.Fatal(e)
	}

	if len(p.Rows) != 71 {
		t.Fatal(len(p.Rows))
	}

	if count(t, db, "tags") != 0 || count(t, db, "assets") != 0 || count(t, db, "media") != 0 {
		t.Fatal("preview wrote library")
	}

	universal := 0
	for _, r := range p.Rows {
		if r.Draft.Universal {
			universal++
		}

		if len(r.ImageOptions) != 1 {
			t.Fatalf("%s %d images", r.Key, len(r.ImageOptions))
		}
	}

	if universal != 5 {
		t.Fatal(universal)
	}

	result, e := s.Commit(ctx, dto.ImportCommitRequest{PreviewID: p.PreviewID, Decisions: decisions(p)})
	if e != nil {
		t.Fatal(e)
	}

	if result.Created != 71 || count(t, db, "assets") != 71 || count(t, db, "import_sources") != 71 {
		t.Fatal(result)
	}

	w, e := excel.Read(ctx, path)
	if e != nil {
		t.Fatal(e)
	}

	defer w.Close()

	for _, sheet := range w.Sheets {
		for _, row := range sheet.Rows {
			b, e := w.Image(row.Images[0])
			if e != nil {
				t.Fatal(e)
			}

			sum := sha256.Sum256(b)

			var hash string

			e = db.QueryRow(
				`SELECT m.sha256 FROM import_sources s JOIN assets a ON a.id=s.asset_id JOIN media m ON m.id=a.cover_id WHERE s.source_sha256=? AND s.sheet=? AND s.row_number=?`,
				w.SHA256,
				sheet.Name,
				row.Number,
			).Scan(&hash)
			if e != nil || hash != hex.EncodeToString(sum[:]) {
				t.Fatalf("%s/%d cover mismatch %v", sheet.Name, row.Number, e)
			}
		}
	}

	p, e = s.Preview(ctx, dto.ImportPreviewRequest{Path: path})
	if e != nil {
		t.Fatal(e)
	}

	for _, r := range p.Rows {
		if r.ExistingAssetID == "" {
			t.Fatalf("repeat missing %s", r.Key)
		}
	}

	result, e = s.Commit(ctx, dto.ImportCommitRequest{PreviewID: p.PreviewID, Decisions: decisions(p)})
	if e != nil || result.Skipped != 71 || count(t, db, "assets") != 71 {
		t.Fatal(result, e)
	}
}

func TestAtomicFailure(t *testing.T) {
	s, db := setup(t)
	ctx := context.Background()
	p, e := s.Preview(ctx, dto.ImportPreviewRequest{Path: "../../docs/VRC Assets.xlsx"})
	if e != nil {
		t.Fatal(e)
	}

	_, e = db.Exec(`CREATE TRIGGER fail_import BEFORE INSERT ON assets WHEN (SELECT count(*) FROM assets)=2 BEGIN SELECT RAISE(ABORT,'injected'); END`)
	if e != nil {
		t.Fatal(e)
	}

	r, e := s.Commit(ctx, dto.ImportCommitRequest{PreviewID: p.PreviewID, Decisions: decisions(p)})
	if e == nil || r.Created != 0 || r.Failed != 71 {
		t.Fatal(r, e)
	}

	for _, table := range []string{"assets", "tags", "media", "import_sources", "import_runs"} {
		if count(t, db, table) != 0 {
			t.Fatalf("partial %s", table)
		}
	}

	report, e := s.Report(p.PreviewID)
	if e != nil || len(report) < 3 || string(report[:3]) != "\xef\xbb\xbf" {
		t.Fatal(e)
	}
}

func TestChangedSourceAndBackupFailure(t *testing.T) {
	for _, mode := range []string{"source", "backup"} {
		t.Run(mode, func(t *testing.T) {
			s, db := setup(t)
			ctx := context.Background()
			b, e := os.ReadFile("../../docs/VRC Assets.xlsx")
			if e != nil {
				t.Fatal(e)
			}

			path := filepath.Join(t.TempDir(), "copy.xlsx")
			if e = os.WriteFile(path, b, 0600); e != nil {
				t.Fatal(e)
			}

			p, e := s.Preview(ctx, dto.ImportPreviewRequest{Path: path})
			if e != nil {
				t.Fatal(e)
			}

			if mode == "source" {
				b = append(b, 'x')
				os.WriteFile(path, b, 0600)
			} else {
				_, e = db.Exec(`INSERT INTO assets(id,name,name_key,created_at,updated_at) VALUES('existing','existing','existing','','')`)
				if e != nil {
					t.Fatal(e)
				}

				s.backup = backup.New(filepath.Join(t.TempDir(), "does-not-exist"), db)
			}

			_, e = s.Commit(ctx, dto.ImportCommitRequest{PreviewID: p.PreviewID, Decisions: decisions(p)})
			if e == nil {
				t.Fatal("expected failure")
			}

			want := 0
			if mode == "backup" {
				want = 1
			}

			if count(t, db, "assets") != want || count(t, db, "tags") != 0 {
				t.Fatal("changed library")
			}
		})
	}
}
