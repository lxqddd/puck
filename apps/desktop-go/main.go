package main

import (
	"embed"
	"log"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"puck/internal/events"
	"puck/internal/secure"
	"puck/internal/services"
	"puck/internal/shell"
	"puck/internal/store"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	events.Register()

	settingsStore := store.NewSettingsStore()
	keys := secure.New()

	mascotService := services.NewMascot()
	chatService := services.NewChat(settingsStore, keys)
	settingsService := services.NewSettings(settingsStore, keys)

	app := application.New(application.Options{
		Name:        "Puck",
		Description: "住在你桌角的 AI 小伙伴",
		Services: []application.Service{
			application.NewService(mascotService),
			application.NewService(chatService),
			application.NewService(settingsService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
	})

	windows := shell.NewWindows(app)

	settings, _ := settingsStore.Load()
	initialScale := settings.MascotScale
	if initialScale <= 0 {
		initialScale = 1
	}
	windows.SetInitialMascotScale(initialScale)
	var persistGeneration atomic.Uint64
	windows.SetScalePersistence(func(scale float64) {
		generation := persistGeneration.Add(1)
		time.AfterFunc(400*time.Millisecond, func() {
			if persistGeneration.Load() != generation {
				return
			}
			_ = settingsStore.Update(func(current *store.Settings) {
				current.MascotScale = scale
			})
		})
	})

	windows.CreateMascot()
	windows.CreateTray()
	mascotService.Attach(windows)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
