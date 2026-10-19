package catalog_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/xiaolazhu/vrc_asset_box/internal/dto"
)

func TestBatchTagUnionRemovalCategoryAndRevision(t *testing.T) {
	f := filters(t)
	a := f.assets["Milfy dress"]
	b := f.assets["Shinano dress"]
	if err := f.s.Batch(f.ctx, dto.BatchRequest{
		IDs:       []string{a.ID, a.ID, b.ID},
		Operation: "addTags",
		TagIDs:    []string{f.blue.ID, f.blue.ID},
	}); err != nil {
		t.Fatal(err)
	}

	a, err := f.s.Get(f.ctx, a.ID)
	if err != nil || len(a.Avatars) != 1 || len(a.FreeTags) != 2 || a.Revision != 2 {
		t.Fatalf("union %#v %v", a, err)
	}

	if err = f.s.Batch(f.ctx, dto.BatchRequest{IDs: []string{a.ID, b.ID}, Operation: "removeTags", TagIDs: []string{f.blue.ID}}); err != nil {
		t.Fatal(err)
	}

	a, err = f.s.Get(f.ctx, a.ID)
	if err != nil || len(a.Avatars) != 1 || len(a.FreeTags) != 1 || a.FreeTags[0].ID != f.red.ID || a.Revision != 3 {
		t.Fatalf("remove %#v %v", a, err)
	}

	if err = f.s.Batch(f.ctx, dto.BatchRequest{IDs: []string{a.ID, b.ID}, Operation: "category", CategoryID: "hair"}); err != nil {
		t.Fatal(err)
	}

	a, err = f.s.Get(f.ctx, a.ID)
	if err != nil || a.CategoryID != "hair" || a.Name != "Milfy dress" || a.Revision != 4 {
		t.Fatalf("category %#v %v", a, err)
	}

	if err = f.s.Batch(f.ctx, dto.BatchRequest{IDs: []string{a.ID}, Operation: "category"}); err != nil {
		t.Fatal(err)
	}

	a, err = f.s.Get(f.ctx, a.ID)
	if err != nil || a.CategoryID != "" {
		t.Fatalf("uncategorized %#v %v", a, err)
	}
}

func TestBatchAllOrNothingAndDeleteKeepsExternalFiles(t *testing.T) {
	f := filters(t)
	a := f.assets["Milfy dress"]
	b := f.assets["Shinano dress"]
	for _, r := range []dto.BatchRequest{
		{IDs: []string{a.ID, "missing"}, Operation: "delete"},
		{
			IDs:       []string{a.ID, b.ID},
			Operation: "addTags",
			TagIDs:    []string{f.blue.ID, "missing"},
		},
		{IDs: []string{a.ID}, Operation: "category", CategoryID: "missing"},
	} {
		if err := f.s.Batch(f.ctx, r); err == nil {
			t.Fatalf("accepted %#v", r)
		}
	}

	if _, err := f.db.ExecContext(f.ctx, `CREATE TRIGGER fail_one BEFORE UPDATE ON assets WHEN OLD.name='Shinano dress' BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
		t.Fatal(err)
	}

	err := f.s.Batch(f.ctx, dto.BatchRequest{IDs: []string{a.ID, b.ID}, Operation: "addTags", TagIDs: []string{f.blue.ID}})
	assetCode(t, err, "STORAGE")

	a, err = f.s.Get(f.ctx, a.ID)
	if err != nil || a.Revision != 1 || len(a.FreeTags) != 1 {
		t.Fatalf("partial batch %#v %v", a, err)
	}

	folder := t.TempDir()
	file := filepath.Join(folder, "external.txt")
	if err = os.WriteFile(file, []byte("precious"), 0600); err != nil {
		t.Fatal(err)
	}

	a.FolderPath = folder
	a = assetSave(t, f.ctx, f.s, a.AssetDraft)
	if err = f.s.Batch(f.ctx, dto.BatchRequest{IDs: []string{a.ID}, Operation: "delete"}); err != nil {
		t.Fatal(err)
	}

	if _, err = os.Stat(file); err != nil {
		t.Fatalf("deleted external file: %v", err)
	}

	_, err = f.s.Get(f.ctx, a.ID)
	assetCode(t, err, "NOT_FOUND")
}
