package migration

import (
	"archive/zip"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiaolazhu/vrc_asset_box/internal/dto"
)

func abnormalDrawingWorkbook(t *testing.T, kind string) string {
	t.Helper()

	source := workbook(t, `<row r="2">`+cell("B2", "Affected asset")+`</row><row r="3">`+cell("B3", "Another asset")+`</row>`)
	input, e := zip.OpenReader(source)
	if e != nil {
		t.Fatal(e)
	}

	parts := map[string][]byte{}
	for _, f := range input.File {
		r, e := f.Open()
		if e != nil {
			t.Fatal(e)
		}

		b, e := io.ReadAll(r)
		r.Close()
		if e != nil {
			t.Fatal(e)
		}

		parts[f.Name] = b
	}

	input.Close()

	parts["xl/worksheets/sheet1.xml"] = []byte(strings.Replace(string(parts["xl/worksheets/sheet1.xml"]), "</worksheet>", `<drawing xmlns:r="r" r:id="drawing1"/></worksheet>`, 1))
	parts["xl/worksheets/_rels/sheet1.xml.rels"] = []byte(`<Relationships><Relationship Id="drawing1" Target="../drawings/drawing1.xml"/></Relationships>`)
	anchor := `<oneCellAnchor><from><col>2</col><row>1</row></from><pic><blipFill><a:blip r:embed="missing"/></blipFill></pic></oneCellAnchor>`
	if kind == "absolute" {
		anchor = `<absoluteAnchor><pos x="1" y="1"/><pic><blipFill><a:blip r:embed="missing"/></blipFill></pic></absoluteAnchor>`
	}

	if kind == "unlocated" {
		anchor = `<oneCellAnchor><pic><blipFill><a:blip r:embed="missing"/></blipFill></pic></oneCellAnchor>`
	}

	parts["xl/drawings/drawing1.xml"] = []byte(`<wsDr xmlns:a="a" xmlns:r="r">` + anchor + `</wsDr>`)
	parts["xl/drawings/_rels/drawing1.xml.rels"] = []byte(`<Relationships><Relationship Id="missing" Target="../media/missing.png"/></Relationships>`)
	if kind == "invalid_xml" {
		parts["xl/drawings/drawing1.xml"] = []byte("<invalid")
	}

	if kind == "richdata" {
		parts["xl/worksheets/sheet1.xml"] = []byte(strings.Replace(
			string(parts["xl/worksheets/sheet1.xml"]),
			cell("B2", "Affected asset"),
			cell("B2", "Affected asset")+`<c r="C2" t="e" vm="1"><v>#VALUE!</v></c>`,
			1,
		))
		parts["xl/worksheets/sheet1.xml"] = []byte(strings.Replace(string(parts["xl/worksheets/sheet1.xml"]), `<drawing xmlns:r="r" r:id="drawing1"/>`, "", 1))
		parts["xl/metadata.xml"] = []byte("<invalid")
	}

	filename := filepath.Join(t.TempDir(), "image-issues.xlsx")
	f, e := os.Create(filename)
	if e != nil {
		t.Fatal(e)
	}

	w := zip.NewWriter(f)
	for name, b := range parts {
		entry, e := w.Create(name)
		if e != nil {
			t.Fatal(e)
		}

		if _, e = entry.Write(b); e != nil {
			t.Fatal(e)
		}
	}

	if e = w.Close(); e != nil {
		t.Fatal(e)
	}

	if e = f.Close(); e != nil {
		t.Fatal(e)
	}

	return filename
}

func TestImageIssuesPreserveDataAndRequireExplicitDecision(t *testing.T) {
	for _, kind := range []string{"missing", "absolute", "unlocated", "invalid_xml", "richdata"} {
		t.Run(kind, func(t *testing.T) {
			s, db := setup(t)
			known := kind == "missing" || kind == "richdata"
			ctx := context.Background()
			path := abnormalDrawingWorkbook(t, kind)
			p, e := s.Preview(ctx, dto.ImportPreviewRequest{Path: path})
			if e != nil {
				t.Fatalf("image issue aborted preview: %v", e)
			}

			if len(p.Rows) != 2 || p.Rows[0].Draft.Name != "Affected asset" || p.Rows[1].Draft.Name != "Another asset" {
				t.Fatalf("text lost: %+v", p.Rows)
			}

			if len(p.Rows[0].Issues) == 0 || len(p.Rows[0].ImageOptions) != 0 {
				t.Fatalf("missing image warning: %+v", p.Rows[0])
			}

			if known && len(p.Rows[1].Issues) != 0 {
				t.Fatal("known row image failure marked unrelated row")
			}

			if !known && len(p.Rows[1].Issues) == 0 {
				t.Fatal("unlocatable sheet image was guessed onto one row")
			}

			report, e := s.Report(p.PreviewID)
			if e != nil || !strings.Contains(string(report), "image") {
				t.Fatalf("preview report lost warning: %s %v", report, e)
			}

			request := dto.ImportCommitRequest{PreviewID: p.PreviewID}
			if known {
				request.Decisions = []dto.ImportDecision{{RowKey: p.Rows[1].Key, Action: "create", ResolvedDraft: p.Rows[1].Draft}}
			}

			result, e := s.Commit(ctx, request)
			if e != nil {
				t.Fatal(e)
			}

			wantSkipped := 2
			if known {
				wantSkipped = 1
			}

			if result.Skipped != wantSkipped {
				t.Fatal(result)
			}

			report, e = s.Report(p.PreviewID)
			if e != nil || !strings.Contains(string(report), "image") {
				t.Fatalf("final report lost warning: %s %v", report, e)
			}

			p, e = s.Preview(ctx, dto.ImportPreviewRequest{Path: path})
			if e != nil {
				t.Fatal(e)
			}

			result, e = s.Commit(ctx, dto.ImportCommitRequest{PreviewID: p.PreviewID, Decisions: []dto.ImportDecision{{RowKey: p.Rows[0].Key, Action: "create", ResolvedDraft: p.Rows[0].Draft}}})
			if e != nil || result.Created != 1 || count(t, db, "media") != 0 {
				t.Fatal(result, e)
			}

			report, e = s.Report(p.PreviewID)
			if e != nil || !strings.Contains(string(report), "image") {
				t.Fatalf("explicit consent lost audit warning: %s %v", report, e)
			}

			excluded, e := s.Preview(ctx, dto.ImportPreviewRequest{Path: path, Mappings: []dto.SheetMapping{{Sheet: "Custom", Included: false}}})
			if e != nil || len(excluded.Rows) != 0 {
				t.Fatal(excluded, e)
			}
		})
	}
}
