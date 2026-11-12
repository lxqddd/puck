package shell

import (
	"log"
	"math"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
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

	bubbleWidth  = 260
	bubbleHeight = 140
	bubbleMargin = 8

	// 桌宠 SVG 的 viewBox 与身体横向范围；窗口按 xMidYMid meet 等比缩放，左右会有留白。
	mascotViewBoxWidth  = 240.0
	mascotViewBoxHeight = 300.0
	mascotBodyLeft      = 46.0
	// 气泡尾巴尖端到桌宠身体边缘的视觉间距。
	bubbleVisibleGap = 6
	// 气泡窗口边缘到尾巴尖端的距离，需与 bubble.scss 的 12px 内边距和尾巴外伸保持同步。
	bubbleTailReach = 5
)

type Windows struct {
	app          *application.App
	mascot       *application.WebviewWindow
	bubble       *application.WebviewWindow
	controller   *MascotController
	trayIcon     []byte
	mu           sync.Mutex // 聊天/设置窗口的创建
	bubbleMu     sync.Mutex
	bubbleState  BubbleState
	bubbleShown  bool
	bubbleGen    atomic.Uint64
	scaleMu      sync.Mutex // 缩放状态与持久化回调
	scale        float64
	persistScale func(float64)
}

type BubbleState struct {
	Text string `json:"text"`
	Side string `json:"side"`
}

func NewWindows(app *application.App) *Windows {
	return &Windows{app: app, scale: MascotDefaultScale}
}

func (w *Windows) SetTrayIcon(icon []byte) {
	w.trayIcon = append([]byte(nil), icon...)
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
		if app := application.Get(); app != nil {
			app.Event.Emit(puckevents.NavigateRoute, puckevents.NavigateRouteEvent{Route: "/chat"})
		}
		return
	}

	chat := w.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "chat",
		Title:            "Puck",
		Width:            820,
		Height:           680,
		MinWidth:         720,
		MinHeight:        540,
		URL:              "/app.html",
		BackgroundType:   application.BackgroundTypeSolid,
		BackgroundColour: application.NewRGB(251, 247, 239),
		Mac: application.MacWindow{
			TitleBar:                application.MacTitleBarHiddenInset,
			InvisibleTitleBarHeight: 38,
		},
	})
	chat.Show()
	chat.Focus()
	if screen := w.screenUnderCursor(); screen != nil {
		width, height := chat.Size()
		area := screen.WorkArea
		bounds := w.clampToWorkArea(application.Rect{
			X:      area.X + (area.Width-width)/2,
			Y:      area.Y + (area.Height-height)/2,
			Width:  width,
			Height: height,
		})
		chat.SetPosition(bounds.X, bounds.Y)
	}
	width, height := chat.Size()
	log.Printf("puck: chat window opened %dx%d", width, height)
	w.attachChatWebviewSync(chat)
}

func (w *Windows) screenUnderCursor() *application.Screen {
	screen := w.app.Screen.GetPrimary()
	if point, ok := cursor.Position(); ok {
		if nearest := w.app.Screen.ScreenNearestDipPoint(application.Point{X: point.X, Y: point.Y}); nearest != nil {
			return nearest
		}
	}
	return screen
}

var (
	chatSyncGeneration atomic.Uint64
	chatSyncing        atomic.Bool
	chatSyncCooldown   atomic.Int64
)

// attachChatWebviewSync 兜底处理 macOS 上窗口缩放后 webview 未跟随的偶发情况：
// 在打开后和停止拖拽缩放后，用 1px 的尺寸抖动强制 webview 重新布局。
func (w *Windows) attachChatWebviewSync(chat *application.WebviewWindow) {
	chat.OnWindowEvent(events.Common.WindowDidResize, func(*application.WindowEvent) {
		if chatSyncing.Load() || time.Now().UnixMilli() < chatSyncCooldown.Load() {
			return
		}
		generation := chatSyncGeneration.Add(1)
		go func() {
			time.Sleep(200 * time.Millisecond)
			if chatSyncGeneration.Load() != generation || chatSyncing.Load() {
				return
			}
			w.syncChatWebview(chat)
		}()
	})
	go func() {
		time.Sleep(350 * time.Millisecond)
		w.syncChatWebview(chat)
	}()
}

func (w *Windows) syncChatWebview(chat *application.WebviewWindow) {
	if !chatSyncing.CompareAndSwap(false, true) {
		return
	}
	width, height := chat.Size()
	if width <= 0 || height <= 0 {
		chatSyncing.Store(false)
		return
	}
	chat.SetSize(width+1, height)
	time.Sleep(24 * time.Millisecond)
	chat.SetSize(width, height)
	time.Sleep(40 * time.Millisecond)
	chatSyncCooldown.Store(time.Now().Add(500 * time.Millisecond).UnixMilli())
	chatSyncing.Store(false)
	log.Printf("puck: chat webview synced to %dx%d", width, height)
}

func (w *Windows) CreateTray() {
	tray := w.app.SystemTray.New()
	if len(w.trayIcon) > 0 {
		if runtime.GOOS == "darwin" {
			tray.SetTemplateIcon(w.trayIcon)
		} else {
			tray.SetIcon(w.trayIcon)
		}
	} else {
		tray.SetLabel("Puck")
	}
	tray.SetTooltip("Puck · 桌角的小伙伴")

	menu := application.NewMenu()
	menu.Add("打开对话").OnClick(func(*application.Context) { w.OpenChat() })
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
		w.HideBubble()
		return
	}
	w.mascot.Show()
}

// ShowBubble 在桌宠旁边显示一个独立的透明气泡窗口。
// 气泡固定尺寸，不随桌宠缩放；桌宠位于屏幕右半边时放到左侧，否则放到右侧。
func (w *Windows) ShowBubble(text string, durationMs int) {
	text = strings.TrimSpace(text)
	if text == "" {
		w.HideBubble()
		return
	}
	if w.mascot == nil {
		return
	}

	side, bounds := w.bubblePlacement()
	bubble := w.ensureBubbleWindow()
	if bubble == nil {
		return
	}

	generation := w.bubbleGen.Add(1)
	w.bubbleMu.Lock()
	wasShown := w.bubbleShown
	w.bubbleShown = true
	w.bubbleState = BubbleState{Text: text, Side: side}
	w.bubbleMu.Unlock()

	// 每次显示都重新贴到当前桌宠位置，缩放或移动后气泡不会停留在旧位置。
	bubble.SetPosition(bounds.X, bounds.Y)
	if !wasShown {
		bubble.Show()
		x, y := w.mascot.Position()
		width, height := w.mascot.Size()
		log.Printf("puck: bubble side=%s at (%d,%d), mascot=(%d,%d %dx%d)", side, bounds.X, bounds.Y, x, y, width, height)
	}
	if app := application.Get(); app != nil {
		app.Event.Emit(puckevents.BubbleShow, puckevents.BubbleShowEvent{Text: text, Side: side})
	}

	if durationMs > 0 {
		time.AfterFunc(time.Duration(durationMs)*time.Millisecond, func() {
			if w.bubbleGen.Load() == generation {
				w.HideBubble()
			}
		})
	}
}

func (w *Windows) HideBubble() {
	w.bubbleGen.Add(1)
	w.bubbleMu.Lock()
	w.bubbleState = BubbleState{}
	w.bubbleShown = false
	shown := w.bubble
	w.bubbleMu.Unlock()
	if shown != nil {
		shown.Hide()
	}
}

func (w *Windows) BubbleState() BubbleState {
	w.bubbleMu.Lock()
	defer w.bubbleMu.Unlock()
	return w.bubbleState
}

func (w *Windows) ensureBubbleWindow() *application.WebviewWindow {
	w.bubbleMu.Lock()
	defer w.bubbleMu.Unlock()
	if w.bubble != nil {
		return w.bubble
	}
	w.bubble = w.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:              "bubble",
		Title:             "Puck",
		Width:             bubbleWidth,
		Height:            bubbleHeight,
		Hidden:            true,
		Frameless:         true,
		AlwaysOnTop:       true,
		DisableResize:     true,
		BackgroundType:    application.BackgroundTypeTransparent,
		BackgroundColour:  application.NewRGBA(0, 0, 0, 0),
		IgnoreMouseEvents: true,
		URL:               "/bubble.html",
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
	})
	return w.bubble
}

func (w *Windows) bubblePlacement() (string, application.Rect) {
	if w.mascot == nil {
		return bubblePlacement(application.Rect{}, application.Rect{Width: 1920, Height: 1080})
	}
	x, y := w.mascot.Position()
	width, height := w.mascot.Size()
	mascot := application.Rect{X: x, Y: y, Width: width, Height: height}

	screen := w.app.Screen.GetPrimary()
	center := application.Point{X: x + width/2, Y: y + height/2}
	if nearest := w.app.Screen.ScreenNearestDipPoint(center); nearest != nil {
		screen = nearest
	}
	area := application.Rect{Width: 1920, Height: 1080}
	if screen != nil {
		area = screen.WorkArea
	}
	return bubblePlacement(mascot, area)
}

// bubblePlacement 根据桌宠可见身体与工作区计算气泡位置，纯函数便于测试。
func bubblePlacement(mascot, area application.Rect) (string, application.Rect) {
	inset := mascotBodyInset(mascot.Width, mascot.Height)
	visibleLeft := mascot.X + inset
	visibleRight := mascot.X + mascot.Width - inset

	side := "right"
	x := visibleRight + bubbleVisibleGap - bubbleTailReach
	if mascot.X+mascot.Width/2 >= area.X+area.Width/2 {
		side = "left"
		x = visibleLeft - bubbleVisibleGap - bubbleWidth + bubbleTailReach
	}
	y := mascot.Y + int(float64(mascot.Height)*0.35) - bubbleHeight/2

	minX := area.X + bubbleMargin
	maxX := area.X + area.Width - bubbleWidth - bubbleMargin
	if maxX < minX {
		maxX = minX
	}
	if x < minX {
		x = minX
	}
	if x > maxX {
		x = maxX
	}

	minY := area.Y + bubbleMargin
	maxY := area.Y + area.Height - bubbleHeight - bubbleMargin
	if maxY < minY {
		maxY = minY
	}
	if y < minY {
		y = minY
	}
	if y > maxY {
		y = maxY
	}

	return side, application.Rect{X: x, Y: y, Width: bubbleWidth, Height: bubbleHeight}
}

// mascotBodyInset 计算 SVG 等比缩放后的左右留白与身体左边缘之和。
func mascotBodyInset(width, height int) int {
	scale := math.Min(float64(width)/mascotViewBoxWidth, float64(height)/mascotViewBoxHeight)
	letterbox := (float64(width) - mascotViewBoxWidth*scale) / 2
	return int(math.Round(letterbox + mascotBodyLeft*scale))
}
