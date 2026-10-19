package excel

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/xiaolazhu/vrc_asset_box/internal/dto"
)

func TestExportCoverFormats(t *testing.T) {
	var jpg, pngCover bytes.Buffer
	img := image.NewNRGBA(image.Rect(0, 0, 8, 16))
	if err := jpeg.Encode(&jpg, img, nil); err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(&pngCover, img); err != nil {
		t.Fatal(err)
	}
	webpCover, err := base64.StdEncoding.DecodeString("UklGRh4AAABXRUJQVlA4TBEAAAAvAAAAAAfQ//73v/+BiOh/AAA=")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, embedded string
		contents       []byte
	}{
		{"jpg", "jpg", jpg.Bytes()},
		{"png", "png", pngCover.Bytes()},
		{"webp", "png", webpCover},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			// 故意不使用图片扩展名，验证按真实格式识别。
			coverPath := filepath.Join(root, "cover.data")
			if err := os.WriteFile(coverPath, tc.contents, 0600); err != nil {
				t.Fatal(err)
			}
			filename := filepath.Join(root, "export.xlsx")
			if err := Export(context.Background(), filename, []ExportRow{{CoverPath: coverPath}}, nil); err != nil {
				t.Fatal(err)
			}
			archive, err := zip.OpenReader(filename)
			if err != nil {
				t.Fatal(err)
			}
			defer archive.Close()
			file, err := archive.Open("xl/media/image1." + tc.embedded)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			contents, err := io.ReadAll(file)
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := image.Decode(bytes.NewReader(contents)); err != nil {
				t.Fatal("invalid embedded image", err)
			}
			if tc.name != "webp" && !bytes.Equal(contents, tc.contents) {
				t.Fatal("JPEG or PNG was unnecessarily re-encoded")
			}
			original, _ := os.ReadFile(coverPath)
			if !bytes.Equal(original, tc.contents) {
				t.Fatal("source cover was changed")
			}
			types, err := archive.Open("[Content_Types].xml")
			if err != nil {
				t.Fatal(err)
			}
			defer types.Close()
			declarations, _ := io.ReadAll(types)
			if !bytes.Contains(declarations, []byte(`Extension="`+tc.embedded+`"`)) {
				t.Fatal("missing image content type")
			}
		})
	}
}

func TestExportPreservesTextTagsAndEmbeddedCover(t *testing.T) {
	root := t.TempDir()
	var cover bytes.Buffer
	if err := jpeg.Encode(&cover, image.NewRGBA(image.Rect(0, 0, 8, 16)), nil); err != nil {
		t.Fatal(err)
	}
	coverPath := filepath.Join(root, "cover.jpg")
	if err := os.WriteFile(coverPath, cover.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(root, "素材.xlsx")
	rows := []ExportRow{{
		Asset: dto.Asset{
			AssetDraft: dto.AssetDraft{Name: "=1+1 & <衣服>", BoothID: "001234", Universal: true, Notes: "第一行\n第二行", FolderPath: `Z:\素材`},
			Avatars:    []dto.Tag{{Name: "Shinano"}}, FreeTags: []dto.Tag{{Name: "日常"}},
		},
		Category: "衣服", CoverPath: coverPath,
	}}
	if err := Export(context.Background(), filename, rows, nil); err != nil {
		t.Fatal(err)
	}
	w, err := Read(context.Background(), filename)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if len(w.Sheets) != 1 || len(w.Sheets[0].Rows) != 1 {
		t.Fatalf("unexpected workbook: %+v", w.Sheets)
	}
	row := w.Sheets[0].Rows[0]
	for column, want := range map[string]string{"A": "衣服", "B": "001234", "C": "=1+1 & <衣服>", "E": "Shinano", "F": "Universal", "G": "日常", "H": "第一行\n第二行", "I": `Z:\素材`} {
		if row.Values[column] != want {
			t.Errorf("column %s = %q, want %q", column, row.Values[column], want)
		}
	}
	if len(row.Images) != 1 {
		t.Fatalf("missing embedded cover: %+v", row)
	}
	gotCover, err := w.Image(row.Images[0])
	if err != nil || !bytes.Equal(gotCover, cover.Bytes()) {
		t.Fatalf("cover was not preserved: %v", err)
	}
}

func TestExportEmptyAndCanceled(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "empty.xlsx")
	if err := Export(context.Background(), filename, nil, nil); err != nil {
		t.Fatal(err)
	}
	w, err := Read(context.Background(), filename)
	if err != nil {
		t.Fatal(err)
	}
	if len(w.Sheets) != 1 || len(w.Sheets[0].Rows) != 0 || w.Sheets[0].Headers["E"] != "Avatar tags" {
		t.Fatalf("empty workbook lost headers: %+v", w.Sheets)
	}
	w.Close()
	before, _ := os.ReadFile(filename)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Export(ctx, filename, nil, nil); err == nil {
		t.Fatal("canceled export succeeded")
	}
	after, _ := os.ReadFile(filename)
	if !bytes.Equal(before, after) {
		t.Fatal("cancellation replaced the existing file")
	}
}
