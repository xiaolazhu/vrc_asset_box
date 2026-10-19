package excel

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func drawingWorkbook(t *testing.T, target string, crossRow ...bool) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "drawing.xlsx")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}

	z := zip.NewWriter(f)
	files := map[string]string{
		"xl/workbook.xml":                     `<workbook xmlns:r="r"><sheets><sheet name="Sheet" r:id="book19"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels":          `<Relationships><Relationship Id="book19" Target="worksheets/sheet1.xml"/></Relationships>`,
		"xl/worksheets/sheet1.xml":            `<worksheet xmlns:r="r"><sheetData><row r="1"><c r="A1" t="inlineStr"><is><t>Name</t></is></c></row><row r="2"><c r="A2" t="inlineStr"><is><t>Example</t></is></c></row></sheetData><drawing r:id="drawing42"/></worksheet>`,
		"xl/worksheets/_rels/sheet1.xml.rels": `<Relationships><Relationship Id="drawing42" Target="../drawings/drawing1.xml"/></Relationships>`,
		"xl/drawings/drawing1.xml":            `<wsDr xmlns:a="a" xmlns:r="r"><oneCellAnchor><from><col>2</col><row>1</row></from><pic><blipFill><a:blip r:embed="image99"/></blipFill></pic></oneCellAnchor></wsDr>`,
		"xl/drawings/_rels/drawing1.xml.rels": `<Relationships><Relationship Id="image99" Target="` + target + `"/></Relationships>`,
		"xl/media/cover.png":                  "image bytes",
	}
	if len(crossRow) > 0 && crossRow[0] {
		drawing := files["xl/drawings/drawing1.xml"]
		drawing = strings.ReplaceAll(drawing, "oneCellAnchor", "twoCellAnchor")
		drawing = strings.Replace(drawing, "</from>", "</from><to><col>3</col><row>4</row></to>", 1)
		files["xl/drawings/drawing1.xml"] = drawing
	}

	for p, body := range files {
		w, e := z.Create(p)
		if e != nil {
			t.Fatal(e)
		}

		if _, e = w.Write([]byte(body)); e != nil {
			t.Fatal(e)
		}
	}

	if err = z.Close(); err != nil {
		t.Fatal(err)
	}

	if err = f.Close(); err != nil {
		t.Fatal(err)
	}

	return path
}

func TestCrossRowDrawingRequiresConfirmation(t *testing.T) {
	w, err := Read(context.Background(), drawingWorkbook(t, "../media/cover.png", true))
	if err != nil {
		t.Fatal(err)
	}

	defer w.Close()

	image := w.Sheets[0].Rows[0].Images[0]
	if !strings.Contains(image.Issue, "spans rows") || image.Cell != "C2" {
		t.Fatalf("ambiguous anchor missing warning %+v", image)
	}
}

func TestDrawingResolvesRelationshipAndColumn(t *testing.T) {
	w, err := Read(context.Background(), drawingWorkbook(t, "../media/cover.png"))
	if err != nil {
		t.Fatal(err)
	}

	defer w.Close()

	image := w.Sheets[0].Rows[0].Images[0]
	if image.Cell != "C2" || image.Part != "xl/media/cover.png" {
		t.Fatalf("wrong drawing anchor %+v", image)
	}
}

func TestDrawingDoesNotSilentlyLoseInvalidMedia(t *testing.T) {
	w, err := Read(context.Background(), drawingWorkbook(t, "../media/missing.png"))
	if err != nil {
		t.Fatalf("image failure discarded readable data: %v", err)
	}

	defer w.Close()

	row := w.Sheets[0].Rows[0]
	if row.Values["A"] != "Example" ||
		len(row.Images) != 1 ||
		row.Images[0].Part != "" ||
		!strings.Contains(row.Images[0].Issue, "missing") {
		t.Fatalf("missing drawing has no row warning: %+v", row)
	}
}
