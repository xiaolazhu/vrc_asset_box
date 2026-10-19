package excel_test

import (
	"context"
	"testing"

	"github.com/xiaolazhu/vrc_asset_box/internal/excel"
)

func TestActualWorkbookRichImages(t *testing.T) {
	w, err := excel.Read(context.Background(), "../../docs/VRC Assets.xlsx")
	if err != nil {
		t.Fatal(err)
	}

	defer w.Close()

	if w.SHA256 != "e06809ce792697f63cef1626036b69155bedcc8a36571507b8d38bbc0b325e19" {
		t.Fatal("fixture changed")
	}

	wants := []int{31, 14, 9, 1, 16}
	if len(w.Sheets) != 5 {
		t.Fatal(len(w.Sheets))
	}

	total := 0
	for i, s := range w.Sheets {
		if len(s.Rows) != wants[i] {
			t.Fatalf("%s rows=%d", s.Name, len(s.Rows))
		}

		for _, r := range s.Rows {
			if len(r.Images) != 1 {
				t.Fatalf("%s row %d has %d images", s.Name, r.Number, len(r.Images))
			}

			b, err := w.Image(r.Images[0])
			if err != nil || len(b) == 0 {
				t.Fatalf("image %v %v", r.Images, err)
			}

			total++
		}
	}

	if total != 71 {
		t.Fatal(total)
	}

	if got := w.Sheets[0].Rows[0].Images[0].Part; got != "xl/media/image1.png" {
		t.Fatal(got)
	}

	if got := w.Sheets[4].Rows[15].Images[0].Part; got != "xl/media/image71.png" {
		t.Fatal(got)
	}
}
