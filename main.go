package main

import (
	"embed"
	"log"
	"path/filepath"
	"runtime"

	"taskpilot/internal/core"
	"taskpilot/services"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

func main() {
	// Initialize shared core (DB + stores).
	appCore, err := core.NewAppCore()
	if err != nil {
		log.Fatal("startup failed:", err)
	}

	// Create services.
	projectSvc := &services.ProjectService{Core: appCore}
	taskSvc := &services.TaskService{Core: appCore}
	aiSvc := &services.AIService{Core: appCore}
	configSvc := &services.ConfigService{
		Core:            appCore,
		OnConfigChanged: aiSvc.ReloadClient,
	}
	logSvc := &services.LogService{LogDir: filepath.Join(appCore.DataDir, "logs")}

	// Initialize AI client from stored config.
	aiSvc.ReloadClient()

	// Wire auto-tagging via closure to avoid circular dependencies.
	taskSvc.AutoTagFunc = func(title, description string, existingTags []string) ([]string, error) {
		client := aiSvc.GetAIClient()
		if client == nil {
			return nil, nil
		}
		return client.AutoTagTask(title, description, existingTags)
	}

	// Create the application.
	app := application.New(application.Options{
		Name:        "TaskPilot",
		Description: "AI 驱动的智能任务管理",
		Services: []application.Service{
			application.NewService(projectSvc),
			application.NewService(taskSvc),
			application.NewService(aiSvc),
			application.NewService(configSvc),
			application.NewService(logSvc),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
	})

	// ── Main Window ─────────────────────────────────────────────────────
	mainWindow := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:     "main",
		Title:    "TaskPilot",
		Width:    1280,
		Height:   820,
		MinWidth: 900,
		MinHeight: 600,
		URL:      "/",
		BackgroundColour: application.NewRGB(255, 255, 255),
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
	})

	// ── Quick-Add Window (hidden by default) ────────────────────────────
	quickAddWindow := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:            "quick-add",
		Title:           "快速添加任务",
		Width:           560,
		Height:          380,
		Hidden:          true,
		Frameless:       true,
		AlwaysOnTop:     true,
		HideOnFocusLost: true,
		HideOnEscape:    true,
		URL:             "/#/quick-add",
		BackgroundType:   application.BackgroundTypeTranslucent,
		BackgroundColour: application.NewRGBA(255, 255, 255, 0),
		Mac: application.MacWindow{
			Backdrop: application.MacBackdropTranslucent,
		},
	})

	// ── Chat Window (hidden by default) ─────────────────────────────────
	chatWindow := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:            "chat",
		Title:           "TaskPilot AI",
		Width:           520,
		Height:          640,
		Hidden:          true,
		Frameless:       true,
		AlwaysOnTop:     true,
		HideOnFocusLost: true,
		HideOnEscape:    true,
		URL:             "/#/chat",
		BackgroundType:   application.BackgroundTypeTranslucent,
		BackgroundColour: application.NewRGBA(255, 255, 255, 0),
		Mac: application.MacWindow{
			Backdrop: application.MacBackdropTranslucent,
		},
	})

	// ── Application Menu ────────────────────────────────────────────────
	appMenu := app.NewMenu()

	// macOS 必须：添加标准应用菜单，否则菜单栏无法正确注册，所有快捷键失效。
	if runtime.GOOS == "darwin" {
		appMenu.AddRole(application.AppMenu)
	}

	fileMenu := appMenu.AddSubmenu("文件")
	fileMenu.Add("快速添加任务").
		SetAccelerator("CmdOrCtrl+Shift+N").
		OnClick(func(ctx *application.Context) {
			quickAddWindow.Center()
			quickAddWindow.Show()
			quickAddWindow.Focus()
		})
	fileMenu.AddSeparator()
	fileMenu.Add("退出").
		SetAccelerator("CmdOrCtrl+Q").
		OnClick(func(ctx *application.Context) {
			app.Quit()
		})

	viewMenu := appMenu.AddSubmenu("视图")
	viewMenu.Add("今日任务").
		SetAccelerator("CmdOrCtrl+1").
		OnClick(func(ctx *application.Context) {
			mainWindow.Show()
			mainWindow.Focus()
		})
	viewMenu.Add("AI 助手").
		SetAccelerator("CmdOrCtrl+Shift+C").
		OnClick(func(ctx *application.Context) {
			chatWindow.Center()
			chatWindow.Show()
			chatWindow.Focus()
		})

	app.Menu.SetApplicationMenu(appMenu)

	// ── Shortcut Events from Frontend ───────────────────────────────────
	app.Event.On("shortcut:action", func(e *application.CustomEvent) {
		actionId, ok := e.Data.(string)
		if !ok {
			return
		}
		switch actionId {
		case "task.quickAdd":
			quickAddWindow.Center()
			quickAddWindow.Show()
			quickAddWindow.Focus()
		case "ai.chatWindow":
			chatWindow.Center()
			chatWindow.Show()
			chatWindow.Focus()
		}
	})

	// ── System Tray ─────────────────────────────────────────────────────
	systray := app.SystemTray.New()
	systray.SetIcon(appIcon)
	systray.SetTooltip("TaskPilot")

	trayMenu := app.NewMenu()
	trayMenu.Add("显示主窗口").OnClick(func(ctx *application.Context) {
		mainWindow.Show()
		mainWindow.Focus()
	})
	trayMenu.Add("快速添加任务").OnClick(func(ctx *application.Context) {
		quickAddWindow.Show()
		quickAddWindow.Focus()
	})
	trayMenu.Add("AI 助手").OnClick(func(ctx *application.Context) {
		chatWindow.Show()
		chatWindow.Focus()
	})
	trayMenu.AddSeparator()
	trayMenu.Add("退出").OnClick(func(ctx *application.Context) {
		app.Quit()
	})
	systray.SetMenu(trayMenu)

	systray.OnClick(func() {
		if mainWindow.IsVisible() {
			mainWindow.Hide()
		} else {
			mainWindow.Show()
			mainWindow.Focus()
		}
	})

	// On macOS, set activation policy to accessory if we want tray-only mode.
	_ = runtime.GOOS

	// ── Run ─────────────────────────────────────────────────────────────
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
