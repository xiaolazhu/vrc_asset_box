package excel

import (
	"fmt"
	"strconv"
	"strings"
)

func (w *Workbook) drawingImages(part string, sheet sheetXML) (map[int][]ImageRef, []string) {
	out := map[int][]ImageRef{}
	issues := []string{}
	if len(sheet.Drawings) == 0 {
		return out, issues
	}

	rels, err := w.rels(part)
	if err != nil {
		return out, []string{fmt.Sprintf("Cannot read sheet image relationships or determine affected rows: %v", err)}
	}

	type anchor struct {
		From *struct {
			Row *int `xml:"row"`
			Col *int `xml:"col"`
		} `xml:"from"`
		To *struct {
			Row int `xml:"row"`
			Col int `xml:"col"`
		} `xml:"to"`
		Pic struct {
			Blip struct {
				Embed string `xml:"embed,attr"`
				Link  string `xml:"link,attr"`
			} `xml:"blipFill>blip"`
		} `xml:"pic"`
	}

	for _, ref := range sheet.Drawings {
		drawing, ok := rels[ref.ID]
		if !ok {
			issues = append(issues, fmt.Sprintf("Drawing image relationships are missing (%s); affected rows cannot be determined", ref.ID))
			continue
		}

		links, err := w.rels(drawing)
		if err != nil {
			issues = append(issues, fmt.Sprintf("Cannot read drawing image relationships (%s) or determine affected rows: %v", drawing, err))
			continue
		}

		var d struct {
			Two      []anchor `xml:"twoCellAnchor"`
			One      []anchor `xml:"oneCellAnchor"`
			Absolute []anchor `xml:"absoluteAnchor"`
		}
		if err = w.parse(drawing, &d); err != nil {
			issues = append(issues, fmt.Sprintf("Cannot parse drawing image (%s) or determine affected rows: %v", drawing, err))
			continue
		}

		for _, a := range d.Absolute {
			if a.Pic.Blip.Embed != "" || a.Pic.Blip.Link != "" {
				issues = append(issues, fmt.Sprintf("Unsupported absoluteAnchor image (%s) cannot be located in a cell. No associated row was inferred. Confirm importing without an image or skip this row.", drawing))
			}
		}

		for _, a := range append(d.Two, d.One...) {
			if a.Pic.Blip.Embed == "" && a.Pic.Blip.Link == "" {
				continue
			}

			if a.From == nil ||
				a.From.Row == nil ||
				a.From.Col == nil ||
				*a.From.Row < 0 ||
				*a.From.Row >= 1048576 ||
				*a.From.Col < 0 ||
				*a.From.Col >= 16384 {
				issues = append(issues, fmt.Sprintf("Drawing image has no valid starting point (%s); affected rows cannot be determined. No associated row was inferred.", drawing))
				continue
			}

			row := *a.From.Row + 1
			cell := columnName(*a.From.Col) + strconv.Itoa(row)
			p, ok := links[a.Pic.Blip.Embed]
			if !ok || !strings.HasPrefix(p, "xl/media/") || w.files[p] == nil {
				out[row] = append(out[row], ImageRef{Cell: cell, Issue: fmt.Sprintf("Drawing image on row %d is missing or unsupported (%s, %s). Confirm importing without an image or skip this row.", row, drawing, a.Pic.Blip.Embed)})
				continue
			}

			issue := ""
			if a.To != nil && a.To.Row != *a.From.Row {
				issue = fmt.Sprintf("Image spans rows %d to %d and is shown on its starting row. Confirm its association or skip it.", row, a.To.Row+1)
			}

			out[row] = append(out[row], ImageRef{Part: p, Cell: cell, Issue: issue})
		}
	}

	return out, issues
}

func columnName(index int) string {
	s := ""
	for index >= 0 {
		s = string(rune('A'+index%26)) + s
		index = index/26 - 1
	}

	return s
}
