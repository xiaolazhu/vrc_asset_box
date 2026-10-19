package excel

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf16"

	_ "golang.org/x/image/webp"

	"github.com/xiaolazhu/vrc_asset_box/internal/dto"
)

// ExportRow 包含已取得一致快照的资料和受管封面路径。
type ExportRow struct {
	Asset     dto.Asset
	Category  string
	CoverPath string
}

// Export 生成可独立浏览的工作簿。先写临时文件，取消或失败不会覆盖已有文件。
func Export(ctx context.Context, filename string, rows []ExportRow, progress func(int, int)) error {
	if len(rows) > 1048575 {
		return fmt.Errorf("The asset count exceeds the Excel worksheet row limit")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	file, err := os.CreateTemp(filepath.Dir(filename), ".vrc-export-*.xlsx")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()

	archive := zip.NewWriter(file)
	if err = writeExport(ctx, archive, rows, progress); err != nil {
		return err
	}
	if err = archive.Close(); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}

	return os.Rename(file.Name(), filename)
}

func writeExport(ctx context.Context, archive *zip.Writer, rows []ExportRow, progress func(int, int)) error {
	for _, part := range exportParts {
		if err := writeExportPart(archive, part.name, []byte(xml.Header+part.body)); err != nil {
			return err
		}
	}

	var sheet, drawing, relationships strings.Builder
	sheet.WriteString(xml.Header + exportSheetStart)
	drawing.WriteString(xml.Header + exportDrawingStart)
	relationships.WriteString(xml.Header + exportRelationshipsStart)
	if err := writeExportRow(&sheet, 1, []string{"Category", "Booth ID", "Name", "Image", "Avatar tags", "Universal compatibility", "Custom tags", "Notes", "Linked folder", "Created", "Updated"}); err != nil {
		return err
	}

	for index, row := range rows {
		if err := ctx.Err(); err != nil {
			return err
		}
		universal := ""
		if row.Asset.Universal {
			universal = "Universal"
		}
		values := []string{row.Category, row.Asset.BoothID, row.Asset.Name, "", tagNames(row.Asset.Avatars), universal, tagNames(row.Asset.FreeTags), row.Asset.Notes, row.Asset.FolderPath, row.Asset.CreatedAt, row.Asset.UpdatedAt}
		if err := writeExportRow(&sheet, index+2, values); err != nil {
			return err
		}
		if row.CoverPath != "" {
			if err := writeExportCover(archive, &drawing, &relationships, row.CoverPath, index); err != nil {
				return fmt.Errorf("%s: %w", row.Asset.Name, err)
			}
		}
		if progress != nil {
			progress(index+1, len(rows))
		}
	}

	fmt.Fprintf(&sheet, `</sheetData><autoFilter ref="A1:K%d"/><drawing r:id="rId1"/></worksheet>`, len(rows)+1)
	drawing.WriteString(`</xdr:wsDr>`)
	relationships.WriteString(`</Relationships>`)
	for _, part := range []struct{ name, body string }{
		{"xl/worksheets/sheet1.xml", sheet.String()},
		{"xl/drawings/drawing1.xml", drawing.String()},
		{"xl/drawings/_rels/drawing1.xml.rels", relationships.String()},
	} {
		if err := writeExportPart(archive, part.name, []byte(part.body)); err != nil {
			return err
		}
	}
	return nil
}

func writeExportPart(archive *zip.Writer, name string, contents []byte) error {
	part, err := archive.Create(name)
	if err != nil {
		return err
	}
	_, err = part.Write(contents)
	return err
}

func writeExportRow(sheet *strings.Builder, number int, values []string) error {
	height, style := 78, 0
	if number == 1 {
		height, style = 28, 1
	}
	fmt.Fprintf(sheet, `<row r="%d" ht="%d" customHeight="1">`, number, height)
	for index, value := range values {
		if len(utf16.Encode([]rune(value))) > 32767 {
			return fmt.Errorf("Text in row %d exceeds the Excel cell length limit", number)
		}
		// 所有内容显式写为文本，保留 Booth ID 前导零，并防止名称被当作公式。
		cell := string(rune('A'+index)) + strconv.Itoa(number)
		fmt.Fprintf(sheet, `<c r="%s" t="inlineStr" s="%d"><is><t xml:space="preserve">`, cell, style)
		if err := xml.EscapeText(sheet, []byte(value)); err != nil {
			return err
		}
		sheet.WriteString(`</t></is></c>`)
	}
	sheet.WriteString(`</row>`)
	return nil
}

func tagNames(tags []dto.Tag) string {
	names := make([]string, 0, len(tags))
	for _, tag := range tags {
		names = append(names, tag.Name)
	}
	return strings.Join(names, "\n")
}

func writeExportCover(archive *zip.Writer, drawing, relationships *strings.Builder, path string, index int) error {
	cover, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	config, format, err := image.DecodeConfig(bytes.NewReader(cover))
	if err != nil {
		return err
	}
	extension := ".png"
	switch format {
	case "jpeg":
		extension = ".jpg"
	case "png":
		// PNG 可直接嵌入，保留透明度且避免重复编码。
	case "webp":
		// 使用 PNG 提高 Excel 兼容性；只转换导出副本。
		decoded, _, err := image.Decode(bytes.NewReader(cover))
		if err != nil {
			return err
		}
		var converted bytes.Buffer
		if err = png.Encode(&converted, decoded); err != nil {
			return err
		}
		cover = converted.Bytes()
	default:
		return fmt.Errorf("Invalid cover thumbnail format")
	}

	width, height := 100, 90
	if config.Width*height > config.Height*width {
		height = max(1, config.Height*width/config.Width)
	} else {
		width = max(1, config.Width*height/config.Height)
	}

	name := fmt.Sprintf("image%d%s", index+1, extension)
	if err := writeExportPart(archive, "xl/media/"+name, cover); err != nil {
		return err
	}

	fmt.Fprintf(relationships, `<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/%s"/>`, index+1, name)
	fmt.Fprintf(drawing, exportCoverAnchor, index+1, width*9525, height*9525, index+1, index+1, index+1)
	return nil
}
