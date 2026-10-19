package main

import (
	"net/http"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

func (a *App) assetServerOptions() *assetserver.Options {
	return &assetserver.Options{
		Assets:     assets,
		Handler:    http.HandlerFunc(a.serveMedia),
		Middleware: a.mediaMiddleware,
	}
}

// 开发模式的资源请求先经过 Vite；图片必须在 SPA 回退之前交给 Go。
func (a *App) mediaMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/media/") {
			a.serveMedia(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}
