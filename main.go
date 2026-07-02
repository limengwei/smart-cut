package main

import (
	"embed"
	"log"

	"smart-cut/app"
	"smart-cut/internal/adapter"
	"smart-cut/internal/config"
	"smart-cut/internal/eventbus"
	"smart-cut/internal/service"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	configManager := config.NewConfigManager("")

	bus := eventbus.NewEventBus(nil)

	settings, _ := configManager.Load()
	resolver := adapter.NewBinaryResolver(settings.Binaries, "resources/bin")

	whisperAdapter := adapter.NewWhisperAdapter(resolver)
	ffmpegAdapter := adapter.NewFFmpegAdapter(resolver)
	llmAdapter := adapter.NewLLMAdapter()

	projectService := service.NewProjectService("")
	editService := service.NewEditService()
	transcribeService := service.NewTranscribeService(whisperAdapter, ffmpegAdapter, bus, editService)
	analyzeService := service.NewAnalyzeService(llmAdapter, bus, editService)
	exportService := service.NewExportService(ffmpegAdapter, bus)

	appInstance := app.NewApp(
		projectService,
		transcribeService,
		analyzeService,
		editService,
		exportService,
		configManager,
		resolver,
	)

	wailsApp := application.New(application.Options{
		Name:        "smart-cut",
		Description: "AI 口播视频自动剪辑工具",
		Services: []application.Service{
			application.NewService(appInstance),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	bus.SetEmitFunc(func(name string, data interface{}) {
		wailsApp.Event.Emit(name, data)
	})

	wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "Smart-Cut",
		Width:            1440,
		Height:           900,
		MinWidth:         1280,
		MinHeight:        720,
		BackgroundColour: application.NewRGB(24, 24, 27),
		URL:              "/",
	})

	err := wailsApp.Run()
	if err != nil {
		log.Fatal(err)
	}
}