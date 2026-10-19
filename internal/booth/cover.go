package booth

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/xiaolazhu/vrc_asset_box/internal/dto"
)

func validCoverURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" && u.Port() != "443" {
		return false
	}

	host := strings.ToLower(u.Hostname())

	return host == "booth.pximg.net" || strings.HasSuffix(host, ".booth.pximg.net") || host == "images.booth.pm"
}

// 封面仅从 Booth 的图片 CDN 获取，重定向目标也必须遵守同一限制。
func FetchCover(ctx context.Context, raw string) ([]byte, error) {
	if !validCoverURL(raw) {
		return nil, dto.Error("NETWORK", "Product cover URL is unsupported. Choose a cover manually.")
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 || !validCoverURL(req.URL.String()) {
			return dto.Error("NETWORK", "Cover redirect URL is unsupported")
		}

		return nil
	}}
	req, err := http.NewRequestWithContext(ctx, "GET", raw, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, dto.Error("NETWORK", "Failed to download cover. Retry or choose a cover manually.")
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, dto.Error("NETWORK", "Cover is temporarily unavailable. Retry or choose a cover manually.")
	}

	b, err := io.ReadAll(io.LimitReader(resp.Body, (20<<20)+1))
	if err != nil {
		return nil, err
	}

	if len(b) > 20<<20 {
		return nil, dto.Error("VALIDATION", "Product cover exceeds 20 MiB. Choose one manually.")
	}

	return b, nil
}
