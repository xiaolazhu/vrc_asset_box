package tags_test

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/xiaolazhu/vrc_asset_box/internal/database"
	"github.com/xiaolazhu/vrc_asset_box/internal/dto"
	"github.com/xiaolazhu/vrc_asset_box/internal/tags"
)

func setup(t *testing.T) (context.Context, *sql.DB, *tags.Service) {
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

	return ctx, db, tags.New(db)
}

func code(t *testing.T, err error, want string) {
	t.Helper()

	var e *dto.AppError
	if !errors.As(err, &e) || e.Code != want {
		t.Fatalf("error=%v; want %s", err, want)
	}
}

func save(t *testing.T, ctx context.Context, s *tags.Service, d dto.TagDraft) dto.Tag {
	t.Helper()

	v, err := s.Save(ctx, d)
	if err != nil {
		t.Fatal(err)
	}

	return v
}

func asset(t *testing.T, ctx context.Context, db *sql.DB, id string, ids ...string) {
	t.Helper()

	_, err := db.ExecContext(
		ctx,
		"INSERT INTO assets(id,name,name_key,created_at,updated_at) VALUES(?,?,?,'2026-01-01T00:00:00Z','2026-01-01T00:00:00Z')",
		id,
		id,
		id,
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, tag := range ids {
		if _, err := db.ExecContext(ctx, "INSERT INTO asset_tags(asset_id,tag_id) VALUES(?,?)", id, tag); err != nil {
			t.Fatal(err)
		}
	}
}

func TestNameKey(t *testing.T) {
	for in, want := range map[string]string{
		"  LUMINA ": "lumina",
		"Lia Alice": "lia alice",
		"　シフォン　":    "シフォン",
		" Chcolat ": "chcolat",
	} {
		if got := tags.NormalizeName(in); got != want {
			t.Fatalf("%q => %q, want %q", in, got, want)
		}
	}
}

func TestSaveNamesAtomicAndIndependentKinds(t *testing.T) {
	ctx, _, s := setup(t)
	a := save(t, ctx, s, dto.TagDraft{Kind: "avatar", Name: " Lumina ", Aliases: []string{" ルミナ "}})
	if a.Name != "Lumina" || !reflect.DeepEqual(a.Aliases, []string{"ルミナ"}) {
		t.Fatalf("unexpected tag: %#v", a)
	}

	save(t, ctx, s, dto.TagDraft{Kind: "free", Name: "LUMINA"})

	_, err := s.Save(ctx, dto.TagDraft{Kind: "avatar", Name: "ルミナ"})
	code(t, err, "TAG_CONFLICT")

	b := save(t, ctx, s, dto.TagDraft{Kind: "avatar", Name: "Shinano", Aliases: []string{"しなの"}})
	_, err = s.Save(ctx, dto.TagDraft{ID: a.ID, Kind: "avatar", Name: "Renamed", Aliases: []string{"しなの"}})
	code(t, err, "TAG_CONFLICT")

	list, err := s.List(ctx, "avatar", " LUMINA ")
	if err != nil || len(list) != 1 || list[0].ID != a.ID || list[0].Name != "Lumina" {
		t.Fatalf("failed update changed original: %#v %v", list, err)
	}

	_, err = s.Save(ctx, dto.TagDraft{Kind: "avatar", Name: "New", Aliases: []string{"fresh", "しなの"}})
	code(t, err, "TAG_CONFLICT")

	list, err = s.List(ctx, "avatar", "")
	if err != nil || len(list) != 2 {
		t.Fatalf("failed create left tag: %#v %v", list, err)
	}

	a = save(t, ctx, s, dto.TagDraft{ID: a.ID, Kind: "avatar", Name: "LUMINA", Aliases: []string{"るみな"}})
	if a.ID == b.ID {
		t.Fatal("rename changed identity")
	}

	list, err = s.List(ctx, "avatar", "ルミナ")
	if err != nil || len(list) != 0 {
		t.Fatalf("removed alias remains: %#v %v", list, err)
	}
}

func TestTagValidation(t *testing.T) {
	ctx, _, s := setup(t)
	for _, d := range []dto.TagDraft{
		{Kind: "bad", Name: "Tag"},
		{Kind: "free", Name: " "},
		{Kind: "free", Name: "Tag", Aliases: []string{"alias"}},
		{Kind: "avatar", Name: "Tag", Aliases: []string{" "}},
		{Kind: "avatar", Name: "Tag", Aliases: []string{"ALIAS", " alias "}},
		{Kind: "avatar", Name: "Tag", Aliases: []string{" tag "}},
	} {
		_, err := s.Save(ctx, d)
		code(t, err, "VALIDATION")
	}

	a := save(t, ctx, s, dto.TagDraft{Kind: "avatar", Name: "Tag"})
	_, err := s.Save(ctx, dto.TagDraft{ID: a.ID, Kind: "free", Name: "Tag"})
	code(t, err, "VALIDATION")

	_, err = s.Save(ctx, dto.TagDraft{ID: "missing", Kind: "free", Name: "Tag"})
	code(t, err, "NOT_FOUND")

	_, err = s.List(ctx, "bad", "")
	code(t, err, "VALIDATION")
}

func TestListSearchesAliasesAndCountsDistinctAssets(t *testing.T) {
	ctx, db, s := setup(t)
	a := save(t, ctx, s, dto.TagDraft{Kind: "avatar", Name: "LUMINA", Aliases: []string{"Lumina-chan", "ルミナ"}})
	asset(t, ctx, db, "one", a.ID)
	asset(t, ctx, db, "two", a.ID)

	list, err := s.List(ctx, "avatar", " luMINA ")
	if err != nil || len(list) != 1 || list[0].Count != 2 || len(list[0].Aliases) != 2 {
		t.Fatalf("list=%#v err=%v", list, err)
	}

	list, err = s.List(ctx, "avatar", "ルミ")
	if err != nil || len(list) != 1 {
		t.Fatalf("alias search=%#v %v", list, err)
	}

	list, err = s.List(ctx, "avatar", "%")
	if err != nil || len(list) != 0 {
		t.Fatalf("keyword must be literal: %#v %v", list, err)
	}

	empty, err := s.List(ctx, "free", "")
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty list=%#v %v", empty, err)
	}
}

func TestMergeMovesDeduplicatesAndPreservesAvatarNames(t *testing.T) {
	ctx, db, s := setup(t)
	a := save(t, ctx, s, dto.TagDraft{Kind: "avatar", Name: "Source", Aliases: []string{"元"}})
	b := save(t, ctx, s, dto.TagDraft{Kind: "avatar", Name: "Target", Aliases: []string{"先"}})
	asset(t, ctx, db, "one", a.ID, b.ID)
	asset(t, ctx, db, "two", a.ID)
	asset(t, ctx, db, "three", b.ID)
	if err := s.Merge(ctx, a.ID, b.ID); err != nil {
		t.Fatal(err)
	}

	list, err := s.List(ctx, "avatar", "")
	if err != nil ||
		len(list) != 1 ||
		list[0].ID != b.ID ||
		list[0].Name != "Target" ||
		list[0].Count != 3 ||
		len(list[0].Aliases) != 3 {
		t.Fatalf("merged=%#v %v", list, err)
	}

	for _, keyword := range []string{"Source", "元", "先"} {
		list, err = s.List(ctx, "avatar", keyword)
		if err != nil || len(list) != 1 || list[0].ID != b.ID {
			t.Fatalf("lost name %s: %#v %v", keyword, list, err)
		}
	}

	var rev int
	if err := db.QueryRowContext(ctx, "SELECT revision FROM assets WHERE id='two'").Scan(&rev); err != nil || rev != 2 {
		t.Fatalf("merge draft revision=%d %v", rev, err)
	}

	if err := s.Delete(ctx, b.ID); err != nil {
		t.Fatal(err)
	}

	var assets, links, names int

	db.QueryRowContext(ctx, "SELECT count(*) FROM assets").Scan(&assets)
	db.QueryRowContext(ctx, "SELECT count(*) FROM asset_tags").Scan(&links)
	db.QueryRowContext(ctx, "SELECT count(*) FROM tag_names").Scan(&names)
	if assets != 3 || links != 0 || names != 0 {
		t.Fatalf("delete: assets=%d links=%d names=%d", assets, links, names)
	}

	code(t, s.Delete(ctx, b.ID), "NOT_FOUND")
}

func TestMergeValidationAndFreeTagNoAliases(t *testing.T) {
	ctx, db, s := setup(t)
	a := save(t, ctx, s, dto.TagDraft{Kind: "free", Name: "Source"})
	b := save(t, ctx, s, dto.TagDraft{Kind: "free", Name: "Target"})
	c := save(t, ctx, s, dto.TagDraft{Kind: "avatar", Name: "Avatar"})
	asset(t, ctx, db, "one", a.ID, b.ID)
	code(t, s.Merge(ctx, a.ID, a.ID), "VALIDATION")
	code(t, s.Merge(ctx, a.ID, c.ID), "VALIDATION")
	code(t, s.Merge(ctx, a.ID, "missing"), "NOT_FOUND")
	if err := s.Merge(ctx, a.ID, b.ID); err != nil {
		t.Fatal(err)
	}

	list, err := s.List(ctx, "free", "")
	if err != nil || len(list) != 1 || list[0].Count != 1 || list[0].Aliases == nil || len(list[0].Aliases) != 0 {
		t.Fatalf("free merge=%#v %v", list, err)
	}
}

func TestMergeRollsBackOnWriteFailure(t *testing.T) {
	ctx, db, s := setup(t)
	a := save(t, ctx, s, dto.TagDraft{Kind: "avatar", Name: "Source"})
	b := save(t, ctx, s, dto.TagDraft{Kind: "avatar", Name: "Target"})
	asset(t, ctx, db, "one", a.ID)
	if _, err := db.ExecContext(ctx, "CREATE TRIGGER fail_delete BEFORE DELETE ON tags BEGIN SELECT RAISE(ABORT,'injected'); END"); err != nil {
		t.Fatal(err)
	}

	code(t, s.Merge(ctx, a.ID, b.ID), "STORAGE")

	var count int

	db.QueryRowContext(ctx, "SELECT count(*) FROM asset_tags WHERE tag_id=?", b.ID).Scan(&count)
	if count != 0 {
		t.Fatal("merge partially committed associations")
	}

	list, err := s.List(ctx, "avatar", "Source")
	if err != nil || len(list) != 1 || list[0].ID != a.ID || list[0].Count != 1 {
		t.Fatalf("merge rollback=%#v %v", list, err)
	}
}

func TestTagMutationsMarkAssetsUpdated(t *testing.T) {
	for _, operation := range []string{"rename", "delete", "merge"} {
		t.Run(operation, func(t *testing.T) {
			ctx, db, s := setup(t)
			a := save(t, ctx, s, dto.TagDraft{Kind: "avatar", Name: "Source"})
			asset(t, ctx, db, "one", a.ID)
			switch operation {
			case "rename":
				save(t, ctx, s, dto.TagDraft{ID: a.ID, Kind: "avatar", Name: "Renamed"})
			case "delete":
				if err := s.Delete(ctx, a.ID); err != nil {
					t.Fatal(err)
				}
			case "merge":
				b := save(t, ctx, s, dto.TagDraft{Kind: "avatar", Name: "Target"})
				if err := s.Merge(ctx, a.ID, b.ID); err != nil {
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
