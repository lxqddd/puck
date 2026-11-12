package services

import (
	"log"
	"math"

	"puck/internal/shell"
)

type Position struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type MascotService struct {
	windows *shell.Windows
}

func NewMascot() *MascotService {
	return &MascotService{}
}

func (s *MascotService) Attach(windows *shell.Windows) {
	s.windows = windows
}

func (s *MascotService) SetHitRegion(x, y, width, height int, scale float64) {
	if s.windows == nil {
		return
	}
	current := s.windows.MascotScale()
	if math.Abs(scale-current) > 0.001 {
		log.Printf("puck: ignore stale hit region scale=%.2f current=%.2f", scale, current)
		return
	}
	log.Printf("puck: mascot hit region x=%d y=%d w=%d h=%d scale=%.2f", x, y, width, height, scale)
	s.windows.Controller().SetHitRegion(shell.Region{X: x, Y: y, Width: width, Height: height})
}

func (s *MascotService) SetClickThrough(enabled bool) {
	if s.windows == nil {
		return
	}
	s.windows.Controller().SetClickThrough(enabled)
}

func (s *MascotService) SetDragging(dragging bool) {
	if s.windows == nil {
		return
	}
	s.windows.Controller().SetDragging(dragging)
}

func (s *MascotService) GetPosition() Position {
	if s.windows == nil || s.windows.Mascot() == nil {
		return Position{}
	}
	x, y := s.windows.Mascot().Position()
	return Position{X: x, Y: y}
}

func (s *MascotService) GetScale() float64 {
	if s.windows == nil {
		return 1
	}
	return s.windows.MascotScale()
}

func (s *MascotService) SetScale(scale float64) float64 {
	if s.windows == nil {
		return scale
	}
	return s.windows.SetMascotScale(scale)
}

func (s *MascotService) ScaleBy(delta float64) float64 {
	if s.windows == nil {
		return 1
	}
	return s.windows.ScaleMascotBy(delta)
}

func (s *MascotService) SetPosition(x, y int) {
	if s.windows == nil || s.windows.Mascot() == nil {
		return
	}
	s.windows.Mascot().SetPosition(x, y)
}

func (s *MascotService) OpenChat() {
	if s.windows != nil {
		s.windows.OpenChat()
	}
}

func (s *MascotService) OpenSettings() {
	if s.windows != nil {
		s.windows.OpenSettings()
	}
}
