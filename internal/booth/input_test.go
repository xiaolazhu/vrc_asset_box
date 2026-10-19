package booth_test

import (
	"testing"

	"github.com/xiaolazhu/vrc_asset_box/internal/booth"
)

func TestParseInput(t *testing.T) {
	for _, v := range []string{
		"6964558",
		"https://booth.pm/zh-cn/items/6964558?x=1",
		"https://shop.booth.pm/items/6964558",
	} {
		id, err := booth.ParseInput(v)
		if err != nil || id != "6964558" {
			t.Fatalf("%s: %s %v", v, id, err)
		}
	}

	for _, v := range []string{
		"",
		"无",
		"https://booth.pm.evil.test/items/1",
		"https://booth.pm/items/1/extra",
		"https://evil.test/items/1",
	} {
		if _, err := booth.ParseInput(v); err == nil {
			t.Fatal("accepted " + v)
		}
	}
}
