package catalog_test

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/xiaolazhu/vrc_asset_box/internal/catalog"
	"github.com/xiaolazhu/vrc_asset_box/internal/database"
	"github.com/xiaolazhu/vrc_asset_box/internal/dto"
	"github.com/xiaolazhu/vrc_asset_box/internal/tags"
)

func assetSetup(t *testing.T) (context.Context, *sql.DB, *catalog.Service) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
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

func assetSave(t *testing.T, ctx context.Context, s *catalog.Service, d dto.AssetDraft) dto.Asset {
	t.Helper()

	a, err := s.Save(ctx, d)
	if err != nil {
		t.Fatal(err)
	}

	return a
}

func assetCode(t *testing.T, err error, want string) *dto.AppError {
	t.Helper()

	var e *dto.AppError
	if !errors.As(err, &e) || e.Code != want {
		t.Fatalf("err=%v want=%s", err, want)
	}

	return e
}

func assetTag(t *testing.T, ctx context.Context, db *sql.DB, kind, name string, aliases ...string) dto.Tag {
	t.Helper()

	a, err := tags.New(db).Save(ctx, dto.TagDraft{Kind: kind, Name: name, Aliases: aliases})
	if err != nil {
		t.Fatal(err)
	}

	return a
}

func TestAssetSaveMissingBoothAndDuplicateConflict(t *testing.T) {
	ctx, _, s := assetSetup(t)
	a := assetSave(t, ctx, s, dto.AssetDraft{Name: " Same name "})
	b := assetSave(t, ctx, s, dto.AssetDraft{Name: "Same name"})
	if a.ID == b.ID ||
		a.Name != "Same name" ||
		a.Revision != 1 ||
		a.AvatarIDs == nil ||
		a.FreeTagIDs == nil ||
		a.NewTags == nil ||
		a.Avatars == nil ||
		a.FreeTags == nil {
		t.Fatalf("invalid saved assets: %#v %#v", a, b)
	}

	a.BoothID = "6964558"
	a = assetSave(t, ctx, s, a.AssetDraft)
	_, err := s.Save(ctx, dto.AssetDraft{Name: "duplicate", BoothID: a.BoothID})
	e := assetCode(t, err, "DUPLICATE_BOOTH")
	if e.ExistingAssetID != a.ID {
		t.Fatalf("missing duplicate identity: %#v", e)
	}

	for _, d := range []dto.AssetDraft{
		{Name: " "},
		{Name: "bad", BoothID: "無"},
		{Name: "bad", BoothID: "12a"},
		{Name: "bad", CoverID: "missing"},
		{Name: "bad", CategoryID: "missing"},
	} {
		_, err = s.Save(ctx, d)
		if err == nil {
			t.Fatalf("accepted invalid draft %#v", d)
		}
	}

	_, err = s.Get(ctx, "missing")
	assetCode(t, err, "NOT_FOUND")
}

func TestClearBoothPreservesFieldsAndRejectsStaleDraft(t *testing.T) {
	ctx, db, s := assetSetup(t)
	if _, err := db.ExecContext(ctx, `INSERT INTO media VALUES('cover','hash','original','thumb','image/png',1,1,'2026')`); err != nil {
		t.Fatal(err)
	}

	avatar := assetTag(t, ctx, db, "avatar", "Milfy", "ミルフィ")
	free := assetTag(t, ctx, db, "free", "服")
	a := assetSave(
		t,
		ctx,
		s,
		dto.AssetDraft{
			Name:       "Dress",
			BoothID:    "123",
			CategoryID: "cloth",
			CoverID:    "cover",
			Notes:      "line 1\nline 2",
			FolderPath: `Z:\offline`,
			Universal:  true,
			AvatarIDs:  []string{avatar.ID},
			FreeTagIDs: []string{free.ID},
		},
	)
	old := a.AssetDraft
	a.BoothID = ""
	b := assetSave(t, ctx, s, a.AssetDraft)
	if b.ID != a.ID ||
		b.CreatedAt != a.CreatedAt ||
		b.CoverID != a.CoverID ||
		b.Notes != a.Notes ||
		b.FolderPath != a.FolderPath ||
		b.CategoryID != a.CategoryID ||
		!b.Universal ||
		!reflect.DeepEqual(b.AvatarIDs, a.AvatarIDs) ||
		!reflect.DeepEqual(b.FreeTagIDs, a.FreeTagIDs) ||
		b.CoverURL != "/media/cover/original" ||
		b.ThumbnailURL != "/media/cover/thumbnail" ||
		b.Revision != 2 {
		t.Fatalf("clearing ID damaged fields: %#v", b)
	}

	_, err := s.Save(ctx, old)
	assetCode(t, err, "STALE_DRAFT")

	var missingID bool
	if err = db.QueryRowContext(ctx, "SELECT booth_id IS NULL FROM assets WHERE id=?", b.ID).Scan(&missingID); err != nil || !missingID {
		t.Fatalf("empty booth must be NULL: %v", err)
	}
}

func TestNewDraftTagsAreAtomicAndResolveKeys(t *testing.T) {
	ctx, db, s := assetSetup(t)
	d := dto.AssetDraft{
		Name:       "Draft",
		AvatarIDs:  []string{"new-avatar"},
		FreeTagIDs: []string{"new-free"},
		NewTags: []dto.NewTag{{
			Key:     "new-avatar",
			Kind:    "avatar",
			Name:    " Lumina ",
			Aliases: []string{" ルミナ "},
		}, {Key: "new-free", Kind: "free", Name: " Useful "}},
	}
	if _, err := db.ExecContext(ctx, `CREATE TRIGGER reject_assets BEFORE INSERT ON assets BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
		t.Fatal(err)
	}

	_, err := s.Save(ctx, d)
	assetCode(t, err, "STORAGE")

	var count int

	db.QueryRowContext(ctx, "SELECT count(*) FROM tags").Scan(&count)
	if count != 0 {
		t.Fatal("failed asset save committed tags")
	}

	if _, err = db.ExecContext(ctx, "DROP TRIGGER reject_assets"); err != nil {
		t.Fatal(err)
	}

	a := assetSave(t, ctx, s, d)
	if len(a.Avatars) != 1 ||
		a.Avatars[0].Name != "Lumina" ||
		!reflect.DeepEqual(a.Avatars[0].Aliases, []string{"ルミナ"}) ||
		len(a.FreeTags) != 1 ||
		len(a.NewTags) != 0 ||
		a.AvatarIDs[0] == "new-avatar" {
		t.Fatalf("unresolved draft: %#v", a)
	}

	d.Name = "Conflict"
	_, err = s.Save(ctx, d)
	assetCode(t, err, "TAG_CONFLICT")
	db.QueryRowContext(ctx, "SELECT count(*) FROM assets").Scan(&count)
	if count != 1 {
		t.Fatal("tag conflict inserted asset")
	}

	_, err = s.Save(ctx, dto.AssetDraft{Name: "wrong kind", AvatarIDs: a.FreeTagIDs})
	assetCode(t, err, "VALIDATION")
}

func TestSaveInTxSharesCallerTransaction(t *testing.T) {
	ctx, db, s := assetSetup(t)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}

	id, err := s.SaveInTx(ctx, tx, dto.AssetDraft{Name: "Inside transaction"})
	if err != nil {
		tx.Rollback()
		t.Fatal(err)
	}

	tx.Rollback()

	_, err = s.Get(ctx, id)
	assetCode(t, err, "NOT_FOUND")
}
