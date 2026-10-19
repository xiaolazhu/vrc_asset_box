package booth

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/xiaolazhu/vrc_asset_box/internal/dto"
)

var digits = regexp.MustCompile(`^[0-9]+$`)

var itemPath = regexp.MustCompile(`^(?:/[a-z]{2}(?:-[a-z]{2})?)?/items/([0-9]+)/?$`)

func ParseInput(input string) (string, error) {
	input = strings.TrimSpace(input)
	if digits.MatchString(input) {
		return input, nil
	}

	u, err := url.Parse(input)
	if err == nil &&
		(u.Scheme == "https" || u.Scheme == "http") &&
		u.User == nil &&
		(u.Port() == "" || u.Port() == "443" || u.Port() == "80") {
		host := strings.ToLower(u.Hostname())
		if host == "booth.pm" || strings.HasSuffix(host, ".booth.pm") {
			m := itemPath.FindStringSubmatch(u.Path)
			if len(m) == 2 {
				return m[1], nil
			}
		}
	}

	return "", dto.Error("VALIDATION", "Enter a valid Booth ID or product URL")
}
