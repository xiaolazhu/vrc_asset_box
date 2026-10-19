package booth_test

import (
	"testing"

	"github.com/xiaolazhu/vrc_asset_box/internal/booth"
	"github.com/xiaolazhu/vrc_asset_box/internal/dto"
)

func TestParseProductAndCandidates(t *testing.T) {
	p, err := booth.ParseHTML([]byte(`<html><head><meta property="og:title" content="衣服 - BOOTH"><meta property="og:image" content="https://booth.pximg.net/cover.png"></head><body><div class="product-description">Milfy対応。まぬかにも対応。Someone is not Moe.</div></body></html>`))
	if err != nil {
		t.Fatal(err)
	}

	if p.Name != "衣服" || p.CoverURL == "" {
		t.Fatal(p)
	}

	matches := booth.Candidates(p, []dto.Tag{
		{ID: "milfy", Name: "Milfy"},
		{ID: "manuka", Name: "Manuka", Aliases: []string{"まぬか"}},
		{ID: "moe", Name: "Moe"},
	})
	if len(matches) != 3 {
		t.Fatal(matches)
	}

	p.Description = "Someone"
	if len(booth.Candidates(p, []dto.Tag{{ID: "moe", Name: "Moe"}})) != 0 {
		t.Fatal("word fragment matched")
	}
}

func TestParseHTMLSupportsStandardImageFallbacks(t *testing.T) {
	p, err := booth.ParseHTML([]byte(`<meta property="og:title" content="Fallback - BOOTH"><meta name="twitter:image" content="https://images.booth.pm/fallback.jpg">`))
	if err != nil || p.CoverURL != "https://images.booth.pm/fallback.jpg" {
		t.Fatalf("fallback image = %+v, %v", p, err)
	}
}
