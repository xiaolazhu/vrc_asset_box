package migration

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiaolazhu/vrc_asset_box/internal/catalog"
	"github.com/xiaolazhu/vrc_asset_box/internal/dto"
	"github.com/xiaolazhu/vrc_asset_box/internal/tags"
)

func TestMultipleImagesExplicitNoCoverOrDefaultSkip(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(fmt.Sprintf("explicit=%v", explicit), func(t *testing.T) {
			s, db := setup(t)
			ctx := context.Background()

			var imageBytes bytes.Buffer
			if e := png.Encode(&imageBytes, image.NewNRGBA(image.Rect(0, 0, 3, 2))); e != nil {
				t.Fatal(e)
			}

			var workbookBytes bytes.Buffer

			writer := zip.NewWriter(&workbookBytes)
			parts := map[string]string{
				"xl/workbook.xml":                     `<workbook xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Custom" r:id="r1"/></sheets></workbook>`,
				"xl/_rels/workbook.xml.rels":          `<Relationships><Relationship Id="r1" Target="worksheets/sheet1.xml"/></Relationships>`,
				"xl/worksheets/sheet1.xml":            `<worksheet><sheetData><row r="1">` + cell("B1", "Name") + cell("C1", "Image") + `</row><row r="2">` + cell("B2", "Multiple pictures") + cell("C2", "local.png") + `</row></sheetData></worksheet>`,
				"xl/worksheets/_rels/sheet1.xml.rels": `<Relationships><Relationship Id="drawing1" Target="../drawings/drawing1.xml"/></Relationships>`,
				"xl/drawings/drawing1.xml":            `<wsDr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><oneCellAnchor><from><col>2</col><row>1</row></from><pic><blipFill><a:blip r:embed="image1"/></blipFill></pic></oneCellAnchor></wsDr>`,
				"xl/drawings/_rels/drawing1.xml.rels": `<Relationships><Relationship Id="image1" Target="../media/image1.png"/></Relationships>`,
			}
			// 必须提供工作表与绘图的关联，才能发现嵌入图片选项。
			parts["xl/worksheets/sheet1.xml"] = strings.Replace(
				parts["xl/worksheets/sheet1.xml"],
				"</worksheet>",
				`<drawing xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" r:id="drawing1"/></worksheet>`,
				1,
			)
			for name, value := range parts {
				f, e := writer.Create(name)
				if e != nil {
					t.Fatal(e)
				}

				if _, e = f.Write([]byte(value)); e != nil {
					t.Fatal(e)
				}
			}

			f, e := writer.Create("xl/media/image1.png")
			if e != nil {
				t.Fatal(e)
			}

			if _, e = f.Write(imageBytes.Bytes()); e != nil {
				t.Fatal(e)
			}

			if e = writer.Close(); e != nil {
				t.Fatal(e)
			}

			root := t.TempDir()
			path := filepath.Join(root, "multiple.xlsx")
			if e = os.WriteFile(path, workbookBytes.Bytes(), 0600); e != nil {
				t.Fatal(e)
			}

			if e = os.WriteFile(filepath.Join(root, "local.png"), imageBytes.Bytes(), 0600); e != nil {
				t.Fatal(e)
			}

			p, e := s.Preview(ctx, dto.ImportPreviewRequest{Path: path})
			if e != nil {
				t.Fatal(e)
			}

			if len(p.Rows) != 1 ||
				len(p.Rows[0].ImageOptions) != 2 ||
				p.Rows[0].Draft.CoverToken != "" ||
				len(p.Rows[0].Issues) == 0 {
				t.Fatalf("invalid multi-image preview: %+v", p.Rows)
			}

			request := dto.ImportCommitRequest{PreviewID: p.PreviewID}
			if explicit {
				request.Decisions = []dto.ImportDecision{{RowKey: p.Rows[0].Key, Action: "create", ResolvedDraft: p.Rows[0].Draft}}
			}

			result, e := s.Commit(ctx, request)
			if e != nil {
				t.Fatal(e)
			}

			if explicit {
				var cover sql.NullString
				if e = db.QueryRow("SELECT cover_id FROM assets").Scan(&cover); e != nil || cover.Valid || result.Created != 1 || count(t, db, "media") != 0 {
					t.Fatal(result, cover, e)
				}
			} else if result.Skipped != 1 || count(t, db, "assets") != 0 {
				t.Fatal(result)
			}
		})
	}
}

func TestRepeatedMergeAccumulatesFields(t *testing.T) {
	s, db := setup(t)
	ctx := context.Background()
	tag, e := tags.New(db).Save(ctx, dto.TagDraft{Kind: "avatar", Name: "Original"})
	if e != nil {
		t.Fatal(e)
	}

	asset, e := catalog.New(db).Save(ctx, dto.AssetDraft{Name: "same item", BoothID: "9", AvatarIDs: []string{tag.ID}})
	if e != nil {
		t.Fatal(e)
	}

	path := workbook(t, `<row r="2">`+cell("A2", "9")+cell("B2", "same item")+cell("D2", "First;通用")+`</row><row r="3">`+cell("A3", "9")+cell("B3", "same item")+cell("D3", "Second")+`</row>`)
	p, e := s.Preview(ctx, dto.ImportPreviewRequest{Path: path})
	if e != nil {
		t.Fatal(e)
	}

	ds := decisions(p)
	for i := range ds {
		ds[i].Action = "merge"
		ds[i].TargetAssetID = asset.ID
		ds[i].TargetRevision = asset.Revision
	}

	ds[0].ResolvedDraft.Notes = "first note"
	ds[1].ResolvedDraft.FolderPath = `D:\Assets`
	for i, key := range []string{"free:first", "free:second"} {
		ds[i].ResolvedDraft.NewTags = append(ds[i].ResolvedDraft.NewTags, dto.NewTag{Key: key, Kind: "free", Name: key})
		ds[i].ResolvedDraft.FreeTagIDs = []string{key}
	}

	r, e := s.Commit(ctx, dto.ImportCommitRequest{PreviewID: p.PreviewID, Decisions: ds})
	if e != nil || r.Merged != 2 {
		t.Fatal(r, e)
	}

	got, e := catalog.New(db).Get(ctx, asset.ID)
	if e != nil {
		t.Fatal(e)
	}

	if !got.Universal ||
		len(got.AvatarIDs) != 3 ||
		len(got.FreeTagIDs) != 2 ||
		got.Notes != "first note" ||
		got.FolderPath != `D:\Assets` {
		t.Fatalf("merge lost fields: %+v", got.AssetDraft)
	}
}

func TestRepeatedMergeScalarConflictRollsBack(t *testing.T) {
	s, db := setup(t)
	ctx := context.Background()
	asset, e := catalog.New(db).Save(ctx, dto.AssetDraft{Name: "same item", BoothID: "9"})
	if e != nil {
		t.Fatal(e)
	}

	path := workbook(t, `<row r="2">`+cell("A2", "9")+cell("B2", "same item")+`</row><row r="3">`+cell("A3", "9")+cell("B3", "same item")+`</row>`)
	p, e := s.Preview(ctx, dto.ImportPreviewRequest{Path: path})
	if e != nil {
		t.Fatal(e)
	}

	ds := decisions(p)
	for i := range ds {
		ds[i].Action = "merge"
		ds[i].TargetAssetID = asset.ID
		ds[i].TargetRevision = asset.Revision
	}

	ds[0].ResolvedDraft.Notes = "first choice"
	ds[1].ResolvedDraft.Notes = "different choice"
	r, e := s.Commit(ctx, dto.ImportCommitRequest{PreviewID: p.PreviewID, Decisions: ds})
	if e == nil || !strings.Contains(e.Error(), "IMPORT_CONFLICT") || r.Merged != 0 || r.Failed != 2 {
		t.Fatal(r, e)
	}

	got, e := catalog.New(db).Get(ctx, asset.ID)
	if e != nil || got.Notes != "" || got.Revision != 1 || count(t, db, "import_sources") != 0 {
		t.Fatal(got, e)
	}

	ds[1].ResolvedDraft.Notes = "first choice"
	r, e = s.Commit(ctx, dto.ImportCommitRequest{PreviewID: p.PreviewID, Decisions: ds})
	if e != nil || r.Merged != 2 {
		t.Fatal(r, e)
	}
}

func TestDuplicateRowsMergeIntoEarlierCreatedRow(t *testing.T) {
	s, db := setup(t)
	ctx := context.Background()
	path := workbook(t, `<row r="2">`+cell("A2", "44")+cell("B2", "chosen name")+cell("D2", "First;通用")+`</row><row r="3">`+cell("A3", "44")+cell("B3", "other name")+cell("D3", "Second")+`</row><row r="4">`+cell("A4", "44")+cell("B4", "third name")+`</row>`)
	p, e := s.Preview(ctx, dto.ImportPreviewRequest{Path: path})
	if e != nil {
		t.Fatal(e)
	}

	if p.Rows[1].ExistingRowKey != p.Rows[0].Key || p.Rows[2].ExistingRowKey != p.Rows[0].Key {
		t.Fatal("duplicate preview omitted target row")
	}

	ds := decisions(p)
	ds[0].ResolvedDraft.Notes = "first note"
	for i := 1; i < len(ds); i++ {
		ds[i].Action = "merge"
		ds[i].TargetAssetID = "row:" + p.Rows[i-1].Key
		ds[i].TargetRevision = 0
		ds[i].ResolvedDraft.Name = "chosen name"
	}

	ds[1].ResolvedDraft.FolderPath = `D:\Assets`
	r, e := s.Commit(ctx, dto.ImportCommitRequest{PreviewID: p.PreviewID, Decisions: ds})
	if e != nil ||
		r.Created != 1 ||
		r.Merged != 2 ||
		count(t, db, "assets") != 1 ||
		count(t, db, "import_sources") != 3 {
		t.Fatal(r, e)
	}

	var id string
	if e = db.QueryRow("SELECT id FROM assets").Scan(&id); e != nil {
		t.Fatal(e)
	}

	got, e := catalog.New(db).Get(ctx, id)
	if e != nil ||
		len(got.AvatarIDs) != 2 ||
		!got.Universal ||
		got.Notes != "first note" ||
		got.FolderPath != `D:\Assets` {
		t.Fatal(got, e)
	}
}

func TestRowReferenceRejectsForwardSkippedAndDifferentID(t *testing.T) {
	for _, mode := range []string{"forward", "skipped", "different"} {
		t.Run(mode, func(t *testing.T) {
			s, db := setup(t)
			ctx := context.Background()
			path := workbook(t, `<row r="2">`+cell("A2", "44")+cell("B2", "same")+`</row><row r="3">`+cell("A3", "44")+cell("B3", "same")+`</row>`)
			p, e := s.Preview(ctx, dto.ImportPreviewRequest{Path: path})
			if e != nil {
				t.Fatal(e)
			}

			ds := decisions(p)
			if mode == "forward" {
				ds[0].Action = "merge"
				ds[0].TargetAssetID = "row:" + p.Rows[1].Key
			} else {
				ds[1].Action = "merge"
				ds[1].TargetAssetID = "row:" + p.Rows[0].Key
				if mode == "skipped" {
					ds[0].Action = "skip"
				} else {
					ds[1].ResolvedDraft.BoothID = "55"
				}
			}

			_, e = s.Commit(ctx, dto.ImportCommitRequest{PreviewID: p.PreviewID, Decisions: ds})
			if e == nil || count(t, db, "assets") != 0 {
				t.Fatal(e)
			}
		})
	}
}

func TestMappedLocalImagePathsStageIndependentCopies(t *testing.T) {
	s, db := setup(t)
	ctx := context.Background()

	var pngBytes bytes.Buffer
	if e := png.Encode(&pngBytes, image.NewNRGBA(image.Rect(0, 0, 3, 2))); e != nil {
		t.Fatal(e)
	}

	absolute := filepath.Join(t.TempDir(), "absolute.png")
	rows := `<row r="2">` + cell("B2", "relative") + cell("C2", "cover.png") + `</row><row r="3">` + cell("B3", "absolute") + cell("C3", absolute) + `</row>`
	path := workbook(t, rows)
	relative := filepath.Join(filepath.Dir(path), "cover.png")
	for _, p := range []string{relative, absolute} {
		if e := os.WriteFile(p, pngBytes.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
	}

	p, e := s.Preview(ctx, dto.ImportPreviewRequest{Path: path, Mappings: []dto.SheetMapping{{Sheet: "Custom", Included: true, NameColumn: "B", ImageColumn: "C"}}})
	if e != nil {
		t.Fatal(e)
	}

	for _, r := range p.Rows {
		if len(r.ImageOptions) != 1 || r.Draft.CoverToken == "" {
			t.Fatalf("path did not stage %+v", r)
		}
	}

	os.Remove(relative)
	os.Remove(absolute)

	r, e := s.Commit(ctx, dto.ImportCommitRequest{PreviewID: p.PreviewID, Decisions: decisions(p)})
	if e != nil || r.Created != 2 || count(t, db, "media") != 1 {
		t.Fatal(r, e)
	}
}

func TestMappedImagePathIssuesNeverFetchOrExecute(t *testing.T) {
	s, _ := setup(t)
	ctx := context.Background()
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
	}))
	defer server.Close()

	path := workbook(t, `<row r="2">`+cell("B2", "http")+cell("C2", server.URL+"/cover.png")+`</row><row r="3">`+cell("B3", "missing")+cell("C3", "missing.png")+`</row><row r="4">`+cell("B4", "formula")+cell("C4", `=HYPERLINK(&quot;https://example.com&quot;)`)+`</row><row r="5">`+cell("B5", "corrupt")+cell("C5", "corrupt.png")+`</row>`)
	if e := os.WriteFile(filepath.Join(filepath.Dir(path), "corrupt.png"), []byte("invalid image data"), 0600); e != nil {
		t.Fatal(e)
	}

	p, e := s.Preview(ctx, dto.ImportPreviewRequest{Path: path, Mappings: []dto.SheetMapping{{Sheet: "Custom", Included: true, NameColumn: "B", ImageColumn: "C"}}})
	if e != nil {
		t.Fatal(e)
	}

	for _, r := range p.Rows {
		if len(r.ImageOptions) != 0 || len(r.Issues) == 0 {
			t.Fatalf("missing image-path issue: %+v", r)
		}
	}

	if requests != 0 {
		t.Fatal("image path fetched HTTP")
	}
}
