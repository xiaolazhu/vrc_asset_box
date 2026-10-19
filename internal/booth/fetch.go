package booth

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/xiaolazhu/vrc_asset_box/internal/dto"
)

type Fetcher struct {
	Client  *http.Client
	BaseURL string
}

func (f Fetcher) Fetch(ctx context.Context, id string) (Product, error) {
	if !digits.MatchString(id) {
		return Product{}, dto.Error("VALIDATION", "Invalid Booth ID")
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	base := f.BaseURL
	if base == "" {
		base = "https://booth.pm/zh-cn/items/"
	}

	client := f.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", base+id, nil)
	if err != nil {
		return Product{}, err
	}

	req.Header.Set("User-Agent", "VRCAssetBox/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return Product{}, &dto.AppError{
			Code:    "NETWORK",
			Message: "Cannot access Booth. Check your network or continue entering details manually.",
			Details: err.Error(),
		}
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return Product{}, dto.Error("NETWORK", fmt.Sprintf("Booth returned HTTP %d. Try again later or enter details manually.", resp.StatusCode))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return Product{}, err
	}

	product, err := ParseHTML(body)
	product.BoothID = id

	return product, err
}
