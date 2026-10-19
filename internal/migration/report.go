package migration

import (
	"bytes"
	"encoding/csv"
	"strconv"
	"strings"

	"github.com/xiaolazhu/vrc_asset_box/internal/dto"
)

type reportRow struct {
	Sheet                        string
	Row                          int
	Action, Result, Reason, Name string
}

// SourcePath 为桌面保存对话框提供源路径，避免覆盖原始工作簿。
func (s *Service) SourcePath(id string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	v := s.sessions[id]
	if v == nil {
		return "", dto.Error("NOT_FOUND", "Import preview not found")
	}

	return v.path, nil
}

func safeCell(v string) string {
	trim := strings.TrimLeft(v, " \t\r\n")
	if trim != "" && strings.ContainsRune("=+-@", rune(trim[0])) {
		return "'" + v
	}

	return v
}

func (s *Service) Report(id string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	v := s.sessions[id]
	if v == nil {
		return nil, dto.Error("NOT_FOUND", "Import preview or report not found")
	}

	var b bytes.Buffer

	b.WriteString("\xef\xbb\xbf")

	w := csv.NewWriter(&b)
	w.Write([]string{"sheet", "row", "action", "result", "reason", "name"})
	if v.report == nil {
		for _, r := range v.preview.Rows {
			w.Write([]string{
				safeCell(r.Sheet),
				strconv.Itoa(r.Row),
				"",
				"preview",
				safeCell(strings.Join(r.Issues, "\n")),
				safeCell(r.Draft.Name),
			})
		}
	} else {
		for _, r := range v.report {
			w.Write([]string{
				safeCell(r.Sheet),
				strconv.Itoa(r.Row),
				r.Action,
				r.Result,
				safeCell(r.Reason),
				safeCell(r.Name),
			})
		}
	}

	w.Flush()

	return b.Bytes(), w.Error()
}
