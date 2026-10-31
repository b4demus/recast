package main

import (
	"embed"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var frontend embed.FS

const (
	version = "1.0.0"
	minW    = 720
	minH    = 520
)

// instanceID can be swapped with -ldflags "-X main.instanceID=…" so a dev
// build runs beside the installed one.
var instanceID = "recast-9b2e41d7-ffmpeg-gui"

func main() {
	app := NewApp()
	w, h := app.data.Window.W, app.data.Window.H
	if w < minW || h < minH {
		w, h = 900, 640
	}
	bg := &options.RGBA{R: 20, G: 20, B: 20, A: 255}
	if app.data.Settings.Theme == "light" {
		bg = &options.RGBA{R: 232, G: 230, B: 223, A: 255}
	}
	err := wails.Run(&options.App{
		Title:            "recast",
		Width:            w,
		Height:           h,
		MinWidth:         minW,
		MinHeight:        minH,
		Frameless:        true,
		BackgroundColour: bg,
		AssetServer:      &assetserver.Options{Assets: frontend},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		DragAndDrop:      &options.DragAndDrop{EnableFileDrop: true, DisableWebViewDrop: true},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               instanceID,
			OnSecondInstanceLaunch: app.secondInstance,
		},
		Bind: []any{app},
		Windows: &windows.Options{
			WebviewUserDataPath: filepath.Join(app.dataDir, "webview"),
			Theme:               windows.SystemDefault,
		},
	})
	if err != nil {
		println(err.Error())
	}
}
