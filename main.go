package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()
	webviewPath, err := webviewDataPath()
	if err != nil {
		println("Error:", err.Error())
		return
	}

	err = wails.Run(&options.App{
		Title:              "VRC Asset Box",
		Width:              1440,
		Height:             960,
		MinWidth:           1080,
		MinHeight:          720,
		AssetServer:        app.assetServerOptions(),
		BackgroundColour:   &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:          app.startup,
		OnShutdown:         app.shutdown,
		SingleInstanceLock: &options.SingleInstanceLock{UniqueId: "59304693-e7cb-49fb-bdd9-64b5df03d655"},
		Windows:            &windows.Options{WebviewUserDataPath: webviewPath},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
