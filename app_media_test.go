package main

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	wailsassets "github.com/wailsapp/wails/v2/pkg/assetserver"
)

func TestMediaRoutingBypassesFrontendInDevAndBuild(t *testing.T) {
	a := testApp(t)
	var imageBytes bytes.Buffer
	if err := png.Encode(&imageBytes, image.NewNRGBA(image.Rect(0, 0, 12, 8))); err != nil {
		t.Fatal(err)
	}
	preview, err := a.media.StageBytes(context.Background(), imageBytes.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	coverID, err := a.media.CommitCover(context.Background(), preview.Token)
	if err != nil {
		t.Fatal(err)
	}

	// 模拟 Vite 的 SPA 回退：未知地址也返回 200 HTML。
	frontendRequests := 0
	frontend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		frontendRequests++
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html>Vite frontend</html>"))
	}))
	defer frontend.Close()
	frontendURL, _ := url.Parse(frontend.URL)

	options := a.assetServerOptions()
	built, err := wailsassets.NewAssetHandler(*options, nil)
	if err != nil {
		t.Fatal(err)
	}
	handlers := map[string]http.Handler{
		"dev":   wailsassets.NewExternalAssetsHandler(nil, *options, frontendURL),
		"build": built,
	}
	for mode, handler := range handlers {
		t.Run(mode, func(t *testing.T) {
			for _, path := range []string{preview.PreviewURL, "/media/" + coverID + "/original", "/media/" + coverID + "/thumbnail"} {
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
				if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "image/png" {
					t.Fatalf("%s: status %d, content type %s", path, response.Code, response.Header().Get("Content-Type"))
				}
				if _, err := png.Decode(response.Body); err != nil {
					t.Fatalf("%s is not a PNG: %v", path, err)
				}
			}
			missing := httptest.NewRecorder()
			handler.ServeHTTP(missing, httptest.NewRequest("GET", "/media/staging/missing", nil))
			if missing.Code != http.StatusNotFound {
				t.Fatalf("missing media returned %d instead of 404", missing.Code)
			}
		})
	}
	if frontendRequests != 0 {
		t.Fatalf("media requests reached Vite %d times", frontendRequests)
	}

	response := httptest.NewRecorder()
	handlers["dev"].ServeHTTP(response, httptest.NewRequest("GET", "/src/main.ts", nil))
	if frontendRequests != 1 || response.Code != http.StatusOK {
		t.Fatal("non-media requests no longer reach the frontend")
	}
}
