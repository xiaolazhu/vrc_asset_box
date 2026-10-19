package booth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchErrorsAndProduct(t *testing.T) {
	for _, status := range []int{403, 404, 429, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
			}))
			defer s.Close()

			if _, err := (Fetcher{BaseURL: s.URL + "/"}).Fetch(context.Background(), "123"); err == nil {
				t.Fatal("HTTP failure accepted")
			}
		})
	}

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ioText := `<meta property="og:title" content="My item - BOOTH"><meta property="og:image" content="https://booth.pximg.net/test.png">`
		_, _ = w.Write([]byte(ioText))
	}))
	defer s.Close()

	p, err := (Fetcher{BaseURL: s.URL + "/"}).Fetch(context.Background(), "123")
	if err != nil || p.Name != "My item" || p.BoothID != "123" {
		t.Fatalf("%+v %v", p, err)
	}
}

func TestCoverHostValidation(t *testing.T) {
	for _, raw := range []string{
		"http://booth.pximg.net/a",
		"https://booth.pximg.net.evil.example/a",
		"https://user@booth.pximg.net/a",
		"https://127.0.0.1/a",
		"https://booth.pximg.net:8000/a",
	} {
		if validCoverURL(raw) {
			t.Fatal(raw)
		}
	}

	if !validCoverURL("https://booth.pximg.net/a.png") {
		t.Fatal("valid CDN rejected")
	}
}
