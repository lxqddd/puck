package shell

import (
	"log"
	"math"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"puck/internal/cursor"
	puckevents "puck/internal/events"
)

const (
	mascotWidth        = 360
	mascotHeight       = 420
	mascotMarginX      = 16
	mascotMarginBottom = 24
	MascotDefaultScale = 0.6
	mascotMinScale     = 0.5
	mascotMaxScale     = 1.5
	mascotScaleStep    = 0.1
)

type Windows struct {
	app          *application.App
	mascot       *application.WebviewWindow
	controller   *MascotController
	mu           sync.Mutex // 聊天/设置窗口的创建
	scaleMu      sync.Mutex // 缩放状态与持久化回调
	scale        float64
	persistScale func(float64)
}

func NewWindows(app *application.App) *Windows {
	return &Windows{app: app, scale: MascotDefaultScale}
}

func (w *Windows) CreateMascot() *MascotController {
	width, height := scaledMascotSize(w.MascotScale())
	options := application.WebviewWindowOptions{
		Name:              "mascot",
		Title:             "Puck",
		Width:             width,
		Height:            height,
		Frameless:         true,
		AlwaysOnTop:       true,
		DisableResize:     true,
		BackgroundType:    application.BackgroundTypeTransparent,
		BackgroundColour:  application.NewRGBA(0, 0, 0, 0),
		IgnoreMouseEvents: true,
		URL:               "/mascot.html",
		Windows: application.WindowsWindow{
			HiddenOnTaskbar:                   true,
			BackdropType:                      application.None,
			DisableFramelessWindowDecorations: true,
		},
		Mac: application.MacWindow{
			Backdrop:      application.MacBackdropTransparent,
			DisableShadow: true,
			CornerType:    application.MacWindowCornerTypeSquare,
			WindowClass:   application.MacWindowClassPanel,
			PanelPreferences: application.MacPanelPreferences{
				NonActivating:          true,
				BecomesKeyOnlyIfNeeded: true,
				FloatingPanel:          true,
			},
			CollectionBehavior: application.MacWindowCollectionBehaviorCanJoinAllSpaces |
				application.MacWindowCollectionBehaviorFullScreenAuxiliary |
				application.MacWindowCollectionBehaviorIgnoresCycle,
		},
	}

	w.mascot = w.app.Window.NewWithOptions(options)
	w.mascot.OnWindowEvent(events.Mac.WebViewDidFinishNavigation, func(*application.WindowEvent) {
		log.Printf("puck: mascot page finished loading")
	})
	w.controller = NewMascotController(w.mascot)
	go w.controller.Run()
	go w.positionMascotWhenReady()
	return w.controller
}

// 屏幕信息在 app.Run 之后才可用，因此启动后轮询，就绪后把宠物放到光标所在屏幕的左下角。
func (w *Windows) positionMascotWhenReady() {
	for i := 0; i < 50; i++ {
		screen := w.app.Screen.GetPrimary()
		if screen != nil {
			if point, ok := cursor.Position(); ok {
				nearest := w.app.Screen.ScreenNearestDipPoint(application.Point{X: point.X, Y: point.Y})
				log.Printf("puck: cursor at (%d,%d), primary=%s, nearest=%v", point.X, point.Y, screen.ID, nearest != nil)
				if nearest != nil {
					screen = nearest
				}
			}
			area := screen.WorkArea
			width, height := scaledMascotSize(w.MascotScale())
			x, y := bottomRightPosition(area, width, height)
			w.mascot.SetPosition(x, y)
			w.mascot.Show()
			bounds := w.mascot.Bounds()
			sizeW, sizeH := w.mascot.Size()
			log.Printf("puck: mascot positioned at (%d,%d), screen=%s workArea=%+v, wails bounds=%+v size=%dx%d", x, y, screen.ID, area, bounds, sizeW, sizeH)
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	log.Printf("puck: screens unavailable, mascot keeps its default position")
}

func (w *Windows) Mascot() *application.WebviewWindow {
	return w.mascot
}

func (w *Windows) Controller() *MascotController {
	return w.controller
}

func clampMascotScale(scale float64) float64 {
	if math.IsNaN(scale) || math.IsInf(scale, 0) {
		return MascotDefaultScale
	}
	scale = math.Round(scale*10) / 10
	if scale < mascotMinScale {
		return mascotMinScale
	}
	if scale > mascotMaxScale {
		return mascotMaxScale
	}
	return scale
}

func scaledMascotSize(scale float64) (int, int) {
	return int(math.Round(mascotWidth * scale)), int(math.Round(mascotHeight * scale))
}

// bottomRightPosition 计算桌宠启动时在工作区右下角的位置。
func bottomRightPosition(area application.Rect, width, height int) (int, int) {
	x := area.X + area.Width - width - mascotMarginX
	if x < area.X {
		x = area.X
	}
	y := area.Y + area.Height - height - mascotMarginBottom
	if y < area.Y {
		y = area.Y
	}
	return x, y
}

// SetInitialMascotScale 必须在 CreateMascot 之前调用。
func (w *Windows) SetInitialMascotScale(scale float64) {
	w.scaleMu.Lock()
	w.scale = clampMascotScale(scale)
	w.scaleMu.Unlock()
}

func (w *Windows) SetScalePersistence(persist func(float64)) {
	w.scaleMu.Lock()
	w.persistScale = persist
	w.scaleMu.Unlock()
}

func (w *Windows) MascotScale() float64 {
	w.scaleMu.Lock()
	defer w.scaleMu.Unlock()
	return w.scale
}

// SetMascotScale 按比例调整窗口大小，并保持左下角位置不动。
// 整个读-改-写序列在 scaleMu 内完成，避免连续滚轮缩放交错。
func (w *Windows) SetMascotScale(scale float64) float64 {
	w.scaleMu.Lock()
	applied, persist := w.setMascotScaleLocked(scale)
	w.scaleMu.Unlock()
	w.broadcastScale(applied, persist)
	return applied
}

func (w *Windows) ScaleMascotBy(delta float64) float64 {
	w.scaleMu.Lock()
	applied, persist := w.setMascotScaleLocked(w.scale + delta)
	w.scaleMu.Unlock()
	w.broadcastScale(applied, persist)
	return applied
}

func (w *Windows) setMascotScaleLocked(scale float64) (float64, func(float64)) {
	scale = clampMascotScale(scale)
	persist := w.persistScale
	if w.mascot == nil || w.scale == scale {
		w.scale = scale
		return scale, persist
	}

	old := w.mascot.Bounds()
	oldScale := w.scale
	w.scale = scale

	newWidth, newHeight := scaledMascotSize(scale)
	if w.controller != nil && oldScale > 0 {
		w.controller.ScaleRegion(scale / oldScale)
	}

	bottom := old.Y + old.Height
	bounds := w.clampToWorkArea(application.Rect{
		X:      old.X,
		Y:      bottom - newHeight,
		Width:  newWidth,
		Height: newHeight,
	})
	w.mascot.SetBounds(bounds)
	log.Printf("puck: mascot scale %.2f (%dx%d -> %dx%d) at (%d,%d)", scale, old.Width, old.Height, newWidth, newHeight, bounds.X, bounds.Y)
	return scale, persist
}

func (w *Windows) broadcastScale(scale float64, persist func(float64)) {
	if persist != nil {
		persist(scale)
	}
	if app := application.Get(); app != nil {
		app.Event.Emit(puckevents.MascotScale, puckevents.MascotScaleEvent{Scale: scale})
	}
}

// clampToWorkArea 防止放大后窗口被顶出可视区。
func (w *Windows) clampToWorkArea(bounds application.Rect) application.Rect {
	screen := w.app.Screen.GetPrimary()
	if screen == nil {
		return bounds
	}
	center := application.Point{X: bounds.X + bounds.Width/2, Y: bounds.Y + bounds.Height/2}
	if nearest := w.app.Screen.ScreenNearestDipPoint(center); nearest != nil {
		screen = nearest
	}
	area := screen.WorkArea
	if maxX := area.X + area.Width - bounds.Width - mascotMarginX; bounds.X > maxX {
		bounds.X = maxX
	}
	if bounds.X < area.X {
		bounds.X = area.X
	}
	if maxY := area.Y + area.Height - bounds.Height - mascotMarginBottom; bounds.Y > maxY {
		bounds.Y = maxY
	}
	if bounds.Y < area.Y {
		bounds.Y = area.Y
	}
	return bounds
}

func (w *Windows) OpenChat() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if win, ok := w.app.Window.GetByName("chat"); ok {
		win.Show()
		win.Focus()
		return
	}

	chat := w.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "chat",
		Title:            "Puck",
		Width:            820,
		Height:           680,
		MinWidth:         560,
		MinHeight:        480,
		URL:              "/chat.html",
		BackgroundType:   application.BackgroundTypeSolid,
		BackgroundColour: application.NewRGB(251, 247, 239),
		Mac: application.MacWindow{
			TitleBar:                application.MacTitleBarHiddenInset,
			InvisibleTitleBarHeight: 38,
		},
	})
	chat.Show()
	chat.Focus()
}

func (w *Windows) OpenSettings() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if win, ok := w.app.Window.GetByName("settings"); ok {
		win.Show()
		win.Focus()
		return
	}

	settings := w.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "settings",
		Title:            "Puck · 设置",
		Width:            640,
		Height:           560,
		MinWidth:         520,
		MinHeight:        420,
		URL:              "/settings.html",
		BackgroundType:   application.BackgroundTypeSolid,
		BackgroundColour: application.NewRGB(251, 247, 239),
		Mac: application.MacWindow{
			TitleBar:                application.MacTitleBarHiddenInset,
			InvisibleTitleBarHeight: 38,
		},
	})
	settings.Show()
	settings.Focus()
}

func (w *Windows) CreateTray() {
	tray := w.app.SystemTray.New()
	tray.SetLabel("Puck")
	tray.SetTooltip("Puck · 桌角的小伙伴")

	menu := application.NewMenu()
	menu.Add("打开对话").OnClick(func(*application.Context) { w.OpenChat() })
	menu.Add("设置…").OnClick(func(*application.Context) { w.OpenSettings() })
	menu.AddSeparator()
	menu.Add("放大").OnClick(func(*application.Context) { w.ScaleMascotBy(mascotScaleStep) })
	menu.Add("缩小").OnClick(func(*application.Context) { w.ScaleMascotBy(-mascotScaleStep) })
	menu.Add("重置大小").OnClick(func(*application.Context) { w.SetMascotScale(MascotDefaultScale) })
	menu.AddSeparator()
	menu.Add("显示 / 隐藏 Puck").OnClick(func(*application.Context) { w.ToggleMascot() })
	menu.AddSeparator()
	menu.Add("退出").OnClick(func(*application.Context) { w.app.Quit() })
	tray.SetMenu(menu)
	tray.OnClick(func() { w.OpenChat() })
}

func (w *Windows) ToggleMascot() {
	if w.mascot == nil {
		return
	}
	if w.mascot.IsVisible() {
		w.mascot.Hide()
		return
	}
	w.mascot.Show()
}
