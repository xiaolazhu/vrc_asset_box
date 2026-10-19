package catalog_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/xiaolazhu/vrc_asset_box/internal/catalog"
	"github.com/xiaolazhu/vrc_asset_box/internal/database"
	"github.com/xiaolazhu/vrc_asset_box/internal/dto"
)

func categorySetup(t *testing.T) (context.Context, *sql.DB, *catalog.Service) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10_000_000_000)
	t.Cleanup(cancel)

	db, err := database.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		db.Close()
	})

	return ctx, db, catalog.New(db)
}

func categoryCode(t *testing.T, err error, want string) {
	t.Helper()

	var e *dto.AppError
	if !errors.As(err, &e) || e.Code != want {
		t.Fatalf("error=%v; want %s", err, want)
	}
}

func categorySave(t *testing.T, ctx context.Context, s *catalog.Service, d dto.CategoryDraft) dto.Category {
	t.Helper()

	v, err := s.SaveCategory(ctx, d)
	if err != nil {
		t.Fatal(err)
	}

	return v
}

func categoryAsset(t *testing.T, ctx context.Context, db *sql.DB, id, cat string) {
	t.Helper()
	if _, err := db.ExecContext(
		ctx,
		"INSERT INTO assets(id,name,name_key,category_id,created_at,updated_at) VALUES(?,?,?,?, '2026-01-01T00:00:00Z','2026-01-01T00:00:00Z')",
		id,
		id,
		id,
		cat,
	); err != nil {
		t.Fatal(err)
	}
}

func TestCategoriesNormalizeRenameAndValidate(t *testing.T) {
	ctx, db, s := categorySetup(t)
	a := categorySave(t, ctx, s, dto.CategoryDraft{Name: " Custom ", Position: 5})
	if a.Name != "Custom" {
		t.Fatalf("name=%q", a.Name)
	}

	categoryAsset(t, ctx, db, "one", a.ID)

	_, err := s.SaveCategory(ctx, dto.CategoryDraft{Name: "custom"})
	categoryCode(t, err, "VALIDATION")
	for _, name := range []string{" ", "全部素材", "未分类", "All assets", "Uncategorized"} {
		_, err = s.SaveCategory(ctx, dto.CategoryDraft{Name: name})
		categoryCode(t, err, "VALIDATION")
	}

	_, err = s.SaveCategory(ctx, dto.CategoryDraft{ID: "missing", Name: "Missing"})
	categoryCode(t, err, "NOT_FOUND")

	a = categorySave(t, ctx, s, dto.CategoryDraft{ID: a.ID, Name: "Edited", Position: 6})
	if a.Count != 1 {
		t.Fatalf("count after rename=%d", a.Count)
	}

	list, err := s.ListCategories(ctx)
	if err != nil || len(list) != 5 || list[4].ID != a.ID || list[4].Name != "Edited" {
		t.Fatalf("categories=%#v %v", list, err)
	}
}

func TestDeleteCategoryMovesOrUncategorizesWithoutDeletingAssets(t *testing.T) {
	ctx, db, s := categorySetup(t)
	a := categorySave(t, ctx, s, dto.CategoryDraft{Name: "A", Position: 5})
	b := categorySave(t, ctx, s, dto.CategoryDraft{Name: "B", Position: 6})
	categoryAsset(t, ctx, db, "one", a.ID)
	categoryCode(t, s.DeleteCategory(ctx, a.ID, "missing"), "NOT_FOUND")
	categoryCode(t, s.DeleteCategory(ctx, a.ID, a.ID), "VALIDATION")
	if err := s.DeleteCategory(ctx, a.ID, b.ID); err != nil {
		t.Fatal(err)
	}

	var cat string
	var rev int
	if err := db.QueryRowContext(ctx, "SELECT category_id,revision FROM assets WHERE id='one'").Scan(&cat, &rev); err != nil || cat != b.ID || rev != 2 {
		t.Fatalf("moved=%s rev=%d %v", cat, rev, err)
	}

	if err := s.DeleteCategory(ctx, b.ID, ""); err != nil {
		t.Fatal(err)
	}

	var nullable sql.NullString
	if err := db.QueryRowContext(ctx, "SELECT category_id,revision FROM assets WHERE id='one'").Scan(&nullable, &rev); err != nil || nullable.Valid || rev != 3 {
		t.Fatalf("uncategorized=%#v rev=%d %v", nullable, rev, err)
	}

	categoryCode(t, s.DeleteCategory(ctx, a.ID, ""), "NOT_FOUND")
}

func TestReorderCategoriesRequiresExactPermutation(t *testing.T) {
	ctx, _, s := categorySetup(t)
	list, err := s.ListCategories(ctx)
	if err != nil {
		t.Fatal(err)
	}

	ids := make([]string, len(list))
	for i := range list {
		ids[len(list)-1-i] = list[i].ID
	}

	categoryCode(t, s.ReorderCategories(ctx, ids[:len(ids)-1]), "VALIDATION")

	bad := append([]string{}, ids...)
	bad[0] = bad[1]
	categoryCode(t, s.ReorderCategories(ctx, bad), "VALIDATION")

	bad[0] = "missing"
	categoryCode(t, s.ReorderCategories(ctx, bad), "VALIDATION")
	if err := s.ReorderCategories(ctx, ids); err != nil {
		t.Fatal(err)
	}

	list, err = s.ListCategories(ctx)
	if err != nil {
		t.Fatal(err)
	}

	for i, c := range list {
		if c.ID != ids[i] || c.Position != i {
			t.Fatalf("position %d=%#v", i, c)
		}
	}
}

func TestCategoryDeleteRollsBackAssetMove(t *testing.T) {
	ctx, db, s := categorySetup(t)
	a := categorySave(t, ctx, s, dto.CategoryDraft{Name: "A", Position: 5})
	b := categorySave(t, ctx, s, dto.CategoryDraft{Name: "B", Position: 6})
	categoryAsset(t, ctx, db, "one", a.ID)
	if _, err := db.ExecContext(ctx, "CREATE TRIGGER fail_delete BEFORE DELETE ON categories BEGIN SELECT RAISE(ABORT,'injected'); END"); err != nil {
		t.Fatal(err)
	}

	categoryCode(t, s.DeleteCategory(ctx, a.ID, b.ID), "STORAGE")

	var cat string
	var rev int
	if err := db.QueryRowContext(ctx, "SELECT category_id,revision FROM assets WHERE id='one'").Scan(&cat, &rev); err != nil || cat != a.ID || rev != 1 {
		t.Fatalf("partial move=%s rev=%d %v", cat, rev, err)
	}
}

func TestCategoryChangesMarkAssetsUpdated(t *testing.T) {
	for _, operation := range []string{"rename", "move", "unclassify"} {
		t.Run(operation, func(t *testing.T) {
			ctx, db, s := categorySetup(t)
			a := categorySave(t, ctx, s, dto.CategoryDraft{Name: "Source", Position: 5})
			categoryAsset(t, ctx, db, "one", a.ID)
			switch operation {
			case "rename":
				categorySave(t, ctx, s, dto.CategoryDraft{ID: a.ID, Name: "Renamed", Position: 5})
			case "move":
				b := categorySave(t, ctx, s, dto.CategoryDraft{Name: "Target", Position: 6})
				if err := s.DeleteCategory(ctx, a.ID, b.ID); err != nil {
					t.Fatal(err)
				}
			case "unclassify":
				if err := s.DeleteCategory(ctx, a.ID, ""); err != nil {
					t.Fatal(err)
				}
			}

			var stamp string
			var revision int
			if err := db.QueryRowContext(ctx, "SELECT updated_at,revision FROM assets WHERE id='one'").Scan(&stamp, &revision); err != nil {
				t.Fatal(err)
			}

			parsed, err := time.Parse(time.RFC3339Nano, stamp)
			if err != nil || !parsed.After(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) || revision != 2 {
				t.Fatalf("updated_at=%q revision=%d err=%v", stamp, revision, err)
			}
		})
	}
}
