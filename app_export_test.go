package main

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/xiaolazhu/vrc_asset_box/internal/dto"
	"github.com/xiaolazhu/vrc_asset_box/internal/excel"
)

func TestExcelExportIncludesEntireLibrary(t *testing.T) {
	a := testApp(t)
	for i := range 65 {
		if _, err := a.SaveAsset(dto.AssetDraft{Name: fmt.Sprintf("手工素材 %d", i), Notes: "导出测试"}); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(t.TempDir(), "all.xlsx")
	if err := a.exportExcel(context.Background(), path, func(string, int, int, bool) {}); err != nil {
		t.Fatal(err)
	}
	w, err := excel.Read(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if len(w.Sheets[0].Rows) != 65 {
		t.Fatalf("only exported %d rows", len(w.Sheets[0].Rows))
	}
	for _, row := range w.Sheets[0].Rows {
		if row.Values["A"] != "Uncategorized" || row.Values["B"] != "" || row.Values["H"] != "导出测试" {
			t.Fatalf("lost manual asset fields: %+v", row)
		}
	}
}
