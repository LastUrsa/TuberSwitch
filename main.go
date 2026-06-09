package main

import (
	"embed"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	config := parseLaunchConfig(os.Args[1:])
	app := NewAppForMode(config.Mode)

	err := wails.Run(buildWailsOptions(app, config))
	if err != nil {
		println("Error:", err.Error())
	}
}

func buildWailsOptions(app *App, config launchConfig) *options.App {
	return &options.App{
		Title:       "TuberSwitch",
		Width:       920,
		Height:      580,
		MinWidth:    860,
		MinHeight:   540,
		StartHidden: config.StartHidden,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: singleInstanceID(),
			OnSecondInstanceLaunch: func(secondInstanceData options.SecondInstanceData) {
				app.handleSecondInstanceLaunch(secondInstanceData.Args)
			},
		},
	}
}
