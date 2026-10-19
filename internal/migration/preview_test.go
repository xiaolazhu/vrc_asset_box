package migration

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiaolazhu/vrc_asset_box/internal/catalog"
	"github.com/xiaolazhu/vrc_asset_box/internal/dto"
	"github.com/xiaolazhu/vrc_asset_box/internal/excel"
	"github.com/xiaolazhu/vrc_asset_box/internal/tags"
)

func TestPresetSheetNamesMatchLocalizedCategories(t *testing.T) {
	for _, sheet := range []string{"Cloth", "Clothing", "衣服", "衣装", "의상"} {
		mapping := defaultMapping(excel.Sheet{Name: sheet}, map[string]string{"衣装": "cloth"})
		if mapping.CategoryID != "cloth" || mapping.NewCategoryName != "" {
			t.Fatalf("sheet %q did not match localized preset: %+v", sheet, mapping)
		}
	}
	mapping := defaultMapping(excel.Sheet{Name: "Hair"}, map[string]string{"hair": "custom", "頭髮": "hair"})
	if mapping.CategoryID != "custom" {
		t.Fatalf("exact user category match was overridden: %+v", mapping)
	}
}

func workbook(t *testing.T, rows string) string {
	t.Helper()

	var b bytes.Buffer

	w := zip.NewWriter(&b)
	for name, content := range map[string]string{
		"xl/workbook.xml":            `<workbook xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Custom" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<Relationships><Relationship Id="rId1" Target="worksheets/sheet1.xml"/></Relationships>`,
		"xl/worksheets/sheet1.xml":   `<worksheet><sheetData><row r="1"><c r="A1" t="inlineStr"><is><t>ID</t></is></c><c r="B1" t="inlineStr"><is><t>Name</t></is></c><c r="D1" t="inlineStr"><is><t>Avatar</t></is></c></row>` + rows + `</sheetData></worksheet>`,
	} {
		f, e := w.Create(name)
		if e != nil {
			t.Fatal(e)
		}

		f.Write([]byte(content))
	}

	if e := w.Close(); e != nil {
		t.Fatal(e)
	}

	path := filepath.Join(t.TempDir(), "test.xlsx")
	if e := os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}

	return path
}

func cell(ref, value string) string {
	return fmt.Sprintf(`<c r="%s" t="inlineStr"><is><t>%s</t></is></c>`, ref, value)
}

func TestEnglishAndChineseImportMarkers(t *testing.T) {
	for _, markers := range [][2]string{{"无", "通用"}, {"None", "Universal"}} {
		t.Run(markers[0], func(t *testing.T) {
			s, _ := setup(t)
			path := workbook(t, `<row r="2">`+cell("A2", markers[0])+cell("B2", "Manual asset")+cell("D2", markers[1])+`</row>`)
			preview, err := s.Preview(context.Background(), dto.ImportPreviewRequest{Path: path})
			if err != nil {
				t.Fatal(err)
			}
			if len(preview.Rows) != 1 || preview.Rows[0].Draft.BoothID != "" || !preview.Rows[0].Draft.Universal || len(preview.Rows[0].Draft.NewTags) != 0 {
				t.Fatalf("import markers were treated as user data: %+v", preview.Rows)
			}
		})
	}
}

func TestMappingAliasesAndTokenization(t *testing.T) {
	s, db := setup(t)
	ctx := context.Background()
	tag, e := tags.New(db).Save(ctx, dto.TagDraft{Kind: "avatar", Name: "Lia Alice", Aliases: []string{"リア"}})
	if e != nil {
		t.Fatal(e)
	}

	path := workbook(t, `<row r="2">`+cell("A2", "无")+cell("B2", "=formula, name")+cell("D2", "リア，New Name;new name / 通用|Other、Another")+`</row><row r="3">`+cell("A3", "12")+cell("D3", "Lia Alice")+`</row>`)
	p, e := s.Preview(ctx, dto.ImportPreviewRequest{Path: path})
	if e != nil {
		t.Fatal(e)
	}

	d := p.Rows[0].Draft
	if len(p.Rows) != 2 ||
		d.BoothID != "" ||
		!d.Universal ||
		len(d.AvatarIDs) != 4 ||
		d.AvatarIDs[0] != tag.ID ||
		len(d.NewTags) != 3 {
		t.Fatalf("%+v", d)
	}

	if len(p.Rows[1].Issues) == 0 {
		t.Fatal("blank name not flagged")
	}

	if count(t, db, "categories") != 4 || count(t, db, "tags") != 1 {
		t.Fatal("preview wrote data")
	}

	r, e := s.Commit(ctx, dto.ImportCommitRequest{PreviewID: p.PreviewID, Decisions: []dto.ImportDecision{{RowKey: p.Rows[0].Key, Action: "create", ResolvedDraft: d}, {RowKey: p.Rows[1].Key, Action: "skip"}}})
	if e != nil || r.Created != 1 || r.Skipped != 1 {
		t.Fatal(r, e)
	}

	csvBytes, e := s.Report(p.PreviewID)
	if e != nil {
		t.Fatal(e)
	}

	records, e := csv.NewReader(strings.NewReader(string(csvBytes[3:]))).ReadAll()
	if e != nil || records[1][5] != "'=formula, name" {
		t.Fatal(records, e)
	}

	mapping := dto.SheetMapping{
		Sheet:           "Custom",
		Included:        true,
		NameColumn:      "D",
		BoothColumn:     "",
		NewCategoryName: "手动分类",
	}
	p, e = s.Preview(ctx, dto.ImportPreviewRequest{Path: path, Mappings: []dto.SheetMapping{mapping}})
	if e != nil ||
		p.Rows[0].Draft.Name != "リア，New Name;new name / 通用|Other、Another" ||
		p.Rows[0].Draft.Universal {
		t.Fatal(p, e)
	}
}

func TestOldPreviewCannotDuplicateNoIDSource(t *testing.T) {
	s, db := setup(t)
	ctx := context.Background()
	path := workbook(t, `<row r="2">`+cell("B2", "no ID")+`</row>`)
	p, e := s.Preview(ctx, dto.ImportPreviewRequest{Path: path})
	if e != nil {
		t.Fatal(e)
	}

	old, e := s.Preview(ctx, dto.ImportPreviewRequest{Path: path})
	if e != nil {
		t.Fatal(e)
	}

	if _, e = s.Commit(ctx, dto.ImportCommitRequest{PreviewID: p.PreviewID, Decisions: decisions(p)}); e != nil {
		t.Fatal(e)
	}

	r, e := s.Commit(ctx, dto.ImportCommitRequest{PreviewID: old.PreviewID, Decisions: decisions(old)})
	if e == nil || r.Failed != 1 || count(t, db, "assets") != 1 {
		t.Fatal(r, e)
	}
}

func TestMergeRevisionAndMultipleRows(t *testing.T) {
	s, db := setup(t)
	ctx := context.Background()
	asset, e := catalog.New(db).Save(ctx, dto.AssetDraft{Name: "existing"})
	if e != nil {
		t.Fatal(e)
	}

	path := workbook(t, `<row r="2">`+cell("B2", "first")+`</row><row r="3">`+cell("B3", "second")+`</row>`)
	p, e := s.Preview(ctx, dto.ImportPreviewRequest{Path: path})
	if e != nil {
		t.Fatal(e)
	}

	ds := decisions(p)
	for i := range ds {
		ds[i].Action = "merge"
		ds[i].TargetAssetID = asset.ID
		ds[i].TargetRevision = asset.Revision
		ds[i].ResolvedDraft.Name = "chosen name"
	}

	result, e := s.Commit(ctx, dto.ImportCommitRequest{PreviewID: p.PreviewID, Decisions: ds})
	if e != nil || result.Merged != 2 || count(t, db, "assets") != 1 || count(t, db, "import_sources") != 2 {
		t.Fatal(result, e)
	}

	p, e = s.Preview(ctx, dto.ImportPreviewRequest{Path: path})
	if e != nil {
		t.Fatal(e)
	}

	a, e := catalog.New(db).Get(ctx, asset.ID)
	if e != nil {
		t.Fatal(e)
	}

	_, e = catalog.New(db).Save(ctx, a.AssetDraft)
	if e != nil {
		t.Fatal(e)
	}

	ds = decisions(p)
	ds[0].Action = "merge"
	ds[0].TargetAssetID = asset.ID
	ds[0].TargetRevision = p.Rows[0].ExistingRevision
	result, e = s.Commit(ctx, dto.ImportCommitRequest{PreviewID: p.PreviewID, Decisions: ds})
	if e == nil || result.Merged != 0 || result.Failed != 1 {
		t.Fatal(result, e)
	}
}
