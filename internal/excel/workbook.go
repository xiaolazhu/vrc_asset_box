package excel

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
)

type ImageRef struct {
	Part  string `json:"part"`
	Cell  string `json:"cell"`
	Issue string `json:"issue,omitempty"`
}

type Row struct {
	Number int               `json:"number"`
	Values map[string]string `json:"values"`
	Images []ImageRef        `json:"images"`
}

type Sheet struct {
	Name    string            `json:"name"`
	Headers map[string]string `json:"headers"`
	Rows    []Row             `json:"rows"`
	Issues  []string          `json:"issues,omitempty"`
}

type Workbook struct {
	SHA256 string
	Sheets []Sheet
	reader *zip.ReadCloser
	files  map[string]*zip.File
}

type relation struct {
	ID     string `xml:"Id,attr"`
	Target string `xml:"Target,attr"`
	Mode   string `xml:"TargetMode,attr"`
}

type relationships struct {
	Items []relation `xml:"Relationship"`
}

type textItem struct {
	Inner string `xml:",innerxml"`
}

type cell struct {
	Ref    string   `xml:"r,attr"`
	Type   string   `xml:"t,attr"`
	VM     int      `xml:"vm,attr"`
	Value  string   `xml:"v"`
	Inline textItem `xml:"is"`
}

type sheetXML struct {
	Rows []struct {
		Number int    `xml:"r,attr"`
		Cells  []cell `xml:"c"`
	} `xml:"sheetData>row"`
	Drawings []struct {
		ID string `xml:"id,attr"`
	} `xml:"drawing"`
}

func (w *Workbook) Close() error {
	return w.reader.Close()
}

func (w *Workbook) read(part string, limit int64) ([]byte, error) {
	f := w.files[part]
	if f == nil {
		return nil, fmt.Errorf("Workbook is missing %s", part)
	}

	if f.UncompressedSize64 > uint64(limit) {
		return nil, fmt.Errorf("File content is too large: %s", part)
	}

	r, err := f.Open()
	if err != nil {
		return nil, err
	}

	defer r.Close()

	b, err := io.ReadAll(io.LimitReader(r, limit+1))
	if len(b) > int(limit) {
		return nil, fmt.Errorf("File content exceeds the limit: %s", part)
	}

	return b, err
}

func (w *Workbook) parse(part string, out any) error {
	b, err := w.read(part, 32<<20)
	if err != nil {
		return err
	}

	return xml.Unmarshal(b, out)
}

func (w *Workbook) Image(ref ImageRef) ([]byte, error) {
	if !strings.HasPrefix(ref.Part, "xl/media/") {
		return nil, fmt.Errorf("Invalid image path")
	}

	return w.read(ref.Part, 20<<20)
}

func resolve(base, target string) string {
	if strings.HasPrefix(target, "/") {
		return strings.TrimPrefix(path.Clean(target), "/")
	}

	return path.Clean(path.Join(path.Dir(base), target))
}

func (w *Workbook) rels(part string) (map[string]string, error) {
	var r relationships
	if err := w.parse(path.Join(path.Dir(part), "_rels", path.Base(part)+".rels"), &r); err != nil {
		return nil, err
	}

	out := map[string]string{}
	for _, v := range r.Items {
		if v.Mode != "External" {
			out[v.ID] = resolve(part, v.Target)
		}
	}

	return out, nil
}

func texts(inner string) string {
	d := xml.NewDecoder(strings.NewReader("<root>" + inner + "</root>"))

	var b strings.Builder

	for {
		t, err := d.Token()
		if err != nil {
			break
		}

		if start, ok := t.(xml.StartElement); ok && start.Name.Local == "t" {
			var s string
			if d.DecodeElement(&s, &start) == nil {
				b.WriteString(s)
			}
		}
	}

	return b.String()
}

func Read(ctx context.Context, filename string) (*Workbook, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	h := sha256.New()
	_, err = io.Copy(h, f)
	f.Close()
	if err != nil {
		return nil, err
	}

	reader, err := zip.OpenReader(filename)
	if err != nil {
		return nil, fmt.Errorf("Cannot read XLSX: %w", err)
	}

	w := &Workbook{
		SHA256: hex.EncodeToString(h.Sum(nil)),
		reader: reader,
		files:  map[string]*zip.File{},
	}
	fail := func(e error) (*Workbook, error) {
		w.Close()
		return nil, e
	}
	for _, v := range reader.File {
		w.files[v.Name] = v
	}

	var book struct {
		Sheets []struct {
			Name string `xml:"name,attr"`
			ID   string `xml:"id,attr"`
		} `xml:"sheets>sheet"`
	}
	if err = w.parse("xl/workbook.xml", &book); err != nil {
		return fail(err)
	}

	rels, err := w.rels("xl/workbook.xml")
	if err != nil {
		return fail(err)
	}

	var shared struct {
		Items []textItem `xml:"si"`
	}
	if w.files["xl/sharedStrings.xml"] != nil {
		if err = w.parse("xl/sharedStrings.xml", &shared); err != nil {
			return fail(err)
		}
	}

	values := make([]string, len(shared.Items))
	for i, v := range shared.Items {
		values[i] = texts(v.Inner)
	}

	rich, richErr := w.richImages()
	for _, s := range book.Sheets {
		if err = ctx.Err(); err != nil {
			return fail(err)
		}

		part, ok := rels[s.ID]
		if !ok {
			return fail(fmt.Errorf("Sheet %s is missing relationships", s.Name))
		}

		var parsed sheetXML
		if err = w.parse(part, &parsed); err != nil {
			return fail(err)
		}

		sheet := Sheet{Name: s.Name, Headers: map[string]string{}, Rows: []Row{}}
		drawings, imageIssues := w.drawingImages(part, parsed)
		sheet.Issues = imageIssues
		presentRows := map[int]bool{}
		for _, r := range parsed.Rows {
			presentRows[r.Number] = true
		}

		for row, images := range drawings {
			if !presentRows[row] {
				for _, image := range images {
					sheet.Issues = append(sheet.Issues, fmt.Sprintf("Drawing image is on empty row %d (%s). No other associated row was inferred.", row, image.Cell))
				}
			}
		}

		for _, r := range parsed.Rows {
			row := Row{Number: r.Number, Values: map[string]string{}, Images: []ImageRef{}}
			for _, c := range r.Cells {
				col := strings.TrimRight(c.Ref, "0123456789")
				v := c.Value
				if c.Type == "s" {
					i, e := strconv.Atoi(v)
					if e != nil || i < 0 || i >= len(values) {
						return fail(fmt.Errorf("%s!%s: Invalid string index", s.Name, c.Ref))
					}

					v = values[i]
				}

				if c.Type == "inlineStr" {
					v = texts(c.Inline.Inner)
				}

				if c.VM > 0 {
					image, ok := rich[c.VM]
					if richErr != nil {
						row.Images = append(row.Images, ImageRef{Cell: c.Ref, Issue: fmt.Sprintf("%s!%s: Cannot read richData image: %v", s.Name, c.Ref, richErr)})
					} else if !ok {
						row.Images = append(row.Images, ImageRef{Cell: c.Ref, Issue: fmt.Sprintf("%s!%s: Image metadata is invalid or unsupported. Confirm importing without an image or skip this row.", s.Name, c.Ref)})
					} else {
						row.Images = append(row.Images, ImageRef{Part: image, Cell: c.Ref})
					}

					v = ""
				}

				if v != "" {
					row.Values[col] = v
				}
			}

			row.Images = append(row.Images, drawings[r.Number]...)
			if len(row.Values) == 0 && len(row.Images) == 0 {
				continue
			}

			if len(sheet.Headers) == 0 {
				sheet.Headers = row.Values
				for _, image := range row.Images {
					if image.Issue != "" {
						sheet.Issues = append(sheet.Issues, fmt.Sprintf("Image in header area: %s", image.Issue))
					}
				}

				continue
			}

			sheet.Rows = append(sheet.Rows, row)
		}

		w.Sheets = append(w.Sheets, sheet)
	}

	return w, nil
}
