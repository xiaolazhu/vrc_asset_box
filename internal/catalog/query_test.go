package catalog_test

import (
	"context"
	"database/sql"
	"reflect"
	"testing"

	"github.com/xiaolazhu/vrc_asset_box/internal/catalog"
	"github.com/xiaolazhu/vrc_asset_box/internal/dto"
)

type filterFixture struct {
	ctx                       context.Context
	db                        *sql.DB
	s                         *catalog.Service
	milfy, shinano, red, blue dto.Tag
	assets                    map[string]dto.Asset
}

func filters(t *testing.T) filterFixture {
	ctx, db, s := assetSetup(t)
	f := filterFixture{ctx: ctx, db: db, s: s, assets: map[string]dto.Asset{}}
	f.milfy = assetTag(t, ctx, db, "avatar", "Milfy", "ミルフィ")
	f.shinano = assetTag(t, ctx, db, "avatar", "Shinano")
	f.red = assetTag(t, ctx, db, "free", "赤")
	f.blue = assetTag(t, ctx, db, "free", "Blue")
	for _, d := range []dto.AssetDraft{
		{
			Name:       "Milfy dress",
			CategoryID: "cloth",
			AvatarIDs:  []string{f.milfy.ID},
			FreeTagIDs: []string{f.red.ID},
			Notes:      "silk",
			BoothID:    "123",
			FolderPath: `C:\dress`,
		},
		{
			Name:       "Shinano dress",
			CategoryID: "cloth",
			AvatarIDs:  []string{f.shinano.ID},
			FreeTagIDs: []string{f.blue.ID},
		},
		{
			Name:       "Both",
			CategoryID: "cloth",
			AvatarIDs:  []string{f.milfy.ID, f.shinano.ID},
			FreeTagIDs: []string{f.red.ID, f.blue.ID},
		},
		{
			Name:       "Universal dress",
			CategoryID: "cloth",
			Universal:  true,
			FreeTagIDs: []string{f.red.ID},
		},
		{Name: "Universal hair", CategoryID: "hair", Universal: true},
		{Name: "Unknown"},
	} {
		f.assets[d.Name] = assetSave(t, ctx, s, d)
	}

	return f
}

func names(t *testing.T, f filterFixture, q dto.AssetQuery, want ...string) {
	t.Helper()

	q.Sort = "name"
	q.Limit = 100
	p, err := f.s.List(f.ctx, q)
	if err != nil {
		t.Fatal(err)
	}

	got := []string{}
	for _, a := range p.Items {
		got = append(got, a.Name)
		if a.Avatars == nil || a.FreeTags == nil || a.AvatarIDs == nil || a.FreeTagIDs == nil {
			t.Fatalf("nil lists %#v", a)
		}
	}

	if !reflect.DeepEqual(got, want) || p.Total != len(want) {
		t.Fatalf("query=%#v got=%v total=%d want=%v", q, got, p.Total, want)
	}
}

func TestAvatarCombinedFiltersConstrainUniversalBranch(t *testing.T) {
	f := filters(t)
	names(
		t,
		f,
		dto.AssetQuery{
			CategoryIDs: []string{"cloth"},
			AvatarIDs:   []string{f.milfy.ID, f.shinano.ID},
			AvatarMatch: "all",
		},
		"Both",
		"Universal dress",
	)
	names(
		t,
		f,
		dto.AssetQuery{CategoryIDs: []string{"cloth"}, AvatarIDs: []string{f.milfy.ID, f.shinano.ID}},
		"Both",
		"Milfy dress",
		"Shinano dress",
		"Universal dress",
	)
	names(t, f, dto.AssetQuery{AvatarMode: "unknown"}, "Unknown")
	names(t, f, dto.AssetQuery{AvatarMode: "universal", CategoryIDs: []string{"cloth"}}, "Universal dress")
	names(t, f, dto.AssetQuery{AvatarIDs: []string{f.milfy.ID}, FreeTagIDs: []string{f.blue.ID}, Folder: "unlinked"}, "Both")
	names(t, f, dto.AssetQuery{AvatarIDs: []string{f.milfy.ID}, Keyword: "silk"}, "Milfy dress")
}

func TestCategoryFreeFolderAndLiteralKeywordFilters(t *testing.T) {
	f := filters(t)
	names(t, f, dto.AssetQuery{CategoryIDs: []string{"hair"}, Uncategorized: true}, "Universal hair", "Unknown")
	names(t, f, dto.AssetQuery{FreeTagIDs: []string{f.red.ID, f.blue.ID}, FreeMatch: "all"}, "Both")
	names(t, f, dto.AssetQuery{FreeTagIDs: []string{f.blue.ID}}, "Both", "Shinano dress")
	names(t, f, dto.AssetQuery{FreeTagIDs: []string{f.red.ID}, FreeMode: "exclude"}, "Shinano dress", "Universal hair", "Unknown")
	names(t, f, dto.AssetQuery{FreeTagIDs: []string{f.red.ID, f.blue.ID}, FreeMatch: "all", FreeMode: "exclude"}, "Milfy dress", "Shinano dress", "Universal dress", "Universal hair", "Unknown")
	names(t, f, dto.AssetQuery{Folder: "linked"}, "Milfy dress")
	names(t, f, dto.AssetQuery{Keyword: "ミルフィ SILK"}, "Milfy dress")
	names(t, f, dto.AssetQuery{Keyword: "blue shinano"}, "Both", "Shinano dress")

	a := assetSave(t, f.ctx, f.s, dto.AssetDraft{Name: `100%_\literal`})
	_ = a
	names(t, f, dto.AssetQuery{Keyword: `%_\`}, `100%_\literal`)

	p, err := f.s.List(f.ctx, dto.AssetQuery{Keyword: "no matches"})
	if err != nil || p.Items == nil || len(p.Items) != 0 || p.Total != 0 {
		t.Fatalf("empty page %#v %v", p, err)
	}
}

func TestNumericPriorityLinkAndOverflowSafeBoothSorting(t *testing.T) {
	ctx, db, s := assetSetup(t)
	f := filterFixture{ctx: ctx, db: db, s: s}
	for _, d := range []dto.AssetDraft{
		{Name: "z exact", BoothID: "123"},
		{Name: "a 123 text", BoothID: "2"},
		{Name: "Huge", BoothID: "999999999999999999999999999999"},
		{Name: "Ten", BoothID: "00010"},
		{Name: "Empty"},
	} {
		assetSave(t, ctx, s, d)
	}

	names(t, f, dto.AssetQuery{Keyword: "123"}, "z exact", "a 123 text")
	names(t, f, dto.AssetQuery{Keyword: "https://shop.booth.pm/items/123?foo=bar"}, "z exact")

	p, err := s.List(ctx, dto.AssetQuery{Sort: "boothId", Limit: 2, Offset: 1})
	if err != nil || p.Total != 5 || len(p.Items) != 2 || p.Items[0].Name != "Ten" || p.Items[1].Name != "z exact" {
		t.Fatalf("numeric page %#v %v", p, err)
	}

	p, err = s.List(ctx, dto.AssetQuery{Sort: "boothId", Descending: true, Limit: 100})
	if err != nil || p.Items[0].Name != "Huge" || p.Items[4].Name != "Empty" {
		t.Fatalf("numeric desc %#v %v", p, err)
	}
}
