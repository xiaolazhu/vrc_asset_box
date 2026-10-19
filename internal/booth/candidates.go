package booth

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/xiaolazhu/vrc_asset_box/internal/dto"
)

func asciiWord(r rune) bool {
	return unicode.IsLetter(r) && r < 128 || r >= '0' && r <= '9' || r == '_'
}

func Candidates(p Product, tags []dto.Tag) []dto.AvatarCandidate {
	result := []dto.AvatarCandidate{}
	text := p.Name + "\n" + p.Description
	lower := strings.ToLower(text)
	for _, tag := range tags {
		if tag.Kind != "" && tag.Kind != "avatar" {
			continue
		}

		found := false
		for _, name := range append([]string{tag.Name}, tag.Aliases...) {
			needle := strings.ToLower(strings.TrimSpace(name))
			if needle == "" {
				continue
			}

			offset := 0
			for offset < len(lower) {
				index := strings.Index(lower[offset:], needle)
				if index < 0 {
					break
				}

				start := offset + index
				end := start + len(needle)
				offset = end
				first, _ := utf8.DecodeRuneInString(needle)
				last, _ := utf8.DecodeLastRuneInString(needle)
				before := rune(0)
				after := rune(0)
				if start > 0 {
					before, _ = utf8.DecodeLastRuneInString(lower[:start])
				}

				if end < len(lower) {
					after, _ = utf8.DecodeRuneInString(lower[end:])
				}

				if asciiWord(first) && asciiWord(before) || asciiWord(last) && asciiWord(after) {
					continue
				}
				// 从规范化文本中按 rune 截取上下文，避免截断多字节名称。
				prefix := []rune(lower[:start])
				suffix := []rune(lower[end:])
				if len(prefix) > 35 {
					prefix = prefix[len(prefix)-35:]
				}

				if len(suffix) > 65 {
					suffix = suffix[:65]
				}

				result = append(result, dto.AvatarCandidate{
					AvatarID:    tag.ID,
					MatchedText: name,
					Context:     string(prefix) + needle + string(suffix),
				})
				found = true
				break
			}

			if found {
				break
			}
		}
	}

	return result
}
