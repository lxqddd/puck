package shell

import (
	"math"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"puck/internal/cursor"
)

type Region struct {
	X      int
	Y      int
	Width  int
	Height int
}

// MascotController 轮询全局光标，在角色命中区内关闭点击穿透。
// 透明窗口一旦完全穿透就收不到前端鼠标事件，所以命中判定必须放在 Go 侧。
type MascotController struct {
	win application.Window

	mu           sync.Mutex
	region       Region
	hasRegion    bool
	clickThrough bool
	ignoring     bool
	dragging     bool

	stopOnce sync.Once
	stop     chan struct{}
}

func NewMascotController(win application.Window) *MascotController {
	return &MascotController{
		win:          win,
		clickThrough: true,
		ignoring:     true,
		stop:         make(chan struct{}),
	}
}

func (c *MascotController) Run() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-c.stop:
			return
		case <-ticker.C:
			c.evaluate()
		}
	}
}

func (c *MascotController) Stop() {
	c.stopOnce.Do(func() { close(c.stop) })
}

func (c *MascotController) SetHitRegion(region Region) {
	c.mu.Lock()
	c.region = region
	c.hasRegion = region.Width > 0 && region.Height > 0
	c.mu.Unlock()
	c.evaluate()
}

// ScaleRegion 在窗口缩放后立即把已有命中区按比例换算，前端随后会重新上报精确值。
func (c *MascotController) ScaleRegion(ratio float64) {
	if ratio <= 0 {
		return
	}
	c.mu.Lock()
	if c.hasRegion {
		c.region = Region{
			X:      int(math.Round(float64(c.region.X) * ratio)),
			Y:      int(math.Round(float64(c.region.Y) * ratio)),
			Width:  int(math.Round(float64(c.region.Width) * ratio)),
			Height: int(math.Round(float64(c.region.Height) * ratio)),
		}
	}
	c.mu.Unlock()
	c.evaluate()
}

func (c *MascotController) SetClickThrough(enabled bool) {
	c.mu.Lock()
	c.clickThrough = enabled
	c.mu.Unlock()
	c.evaluate()
}

// SetDragging 在拖拽期间强制保持可交互，避免光标短暂离开命中区时窗口开始穿透。
func (c *MascotController) SetDragging(dragging bool) {
	c.mu.Lock()
	c.dragging = dragging
	if dragging {
		c.setIgnoreLocked(false)
	}
	c.mu.Unlock()
	if !dragging {
		c.evaluate()
	}
}

func (c *MascotController) evaluate() {
	point, ok := cursor.Position()
	if !ok {
		return
	}
	wx, wy := c.win.Position()

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.dragging {
		return
	}
	if !c.hasRegion {
		c.setIgnoreLocked(c.clickThrough)
		return
	}

	c.setIgnoreLocked(c.clickThrough && !hitTest(point.X, point.Y, wx, wy, c.region))
}

func hitTest(x, y, windowX, windowY int, region Region) bool {
	return x >= windowX+region.X && x <= windowX+region.X+region.Width &&
		y >= windowY+region.Y && y <= windowY+region.Y+region.Height
}

func (c *MascotController) setIgnoreLocked(ignore bool) {
	if ignore == c.ignoring {
		return
	}
	c.ignoring = ignore
	c.win.SetIgnoreMouseEvents(ignore)
}
