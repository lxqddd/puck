package shell

import (
	"math"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestScaledMascotSize(t *testing.T) {
	cases := []struct {
		scale float64
		w, h  int
	}{
		{scale: 0.5, w: 180, h: 210},
		{scale: 1, w: 360, h: 420},
		{scale: 1.2, w: 432, h: 504},
		{scale: 1.5, w: 540, h: 630},
	}
	for _, tc := range cases {
		w, h := scaledMascotSize(tc.scale)
		if w != tc.w || h != tc.h {
			t.Fatalf("scaledMascotSize(%v) = %dx%d, want %dx%d", tc.scale, w, h, tc.w, tc.h)
		}
	}
}

func TestClampMascotScale(t *testing.T) {
	cases := []struct {
		in, want float64
	}{
		{in: 0.2, want: 0.5},
		{in: 0.8, want: 0.8},
		{in: 1.0, want: 1.0},
		{in: 1.24, want: 1.2},
		{in: 1.26, want: 1.3},
		{in: 2.0, want: 1.5},
		{in: math.NaN(), want: MascotDefaultScale},
	}
	for _, tc := range cases {
		if got := clampMascotScale(tc.in); got != tc.want {
			t.Fatalf("clampMascotScale(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestBottomRightPosition(t *testing.T) {
	cases := []struct {
		name          string
		area          application.Rect
		width, height int
		wantX, wantY  int
	}{
		{
			name:  "primary work area",
			area:  application.Rect{X: 0, Y: 30, Width: 1792, Height: 1090},
			width: 216, height: 252,
			wantX: 1560, wantY: 844,
		},
		{
			name:  "left-side display with negative origin",
			area:  application.Rect{X: -2560, Y: 0, Width: 2560, Height: 1440},
			width: 216, height: 252,
			wantX: -232, wantY: 1164,
		},
		{
			name:  "window larger than work area clamps to origin",
			area:  application.Rect{X: 0, Y: 30, Width: 400, Height: 300},
			width: 500, height: 400,
			wantX: 0, wantY: 30,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			x, y := bottomRightPosition(tc.area, tc.width, tc.height)
			if x != tc.wantX || y != tc.wantY {
				t.Fatalf("bottomRightPosition() = (%d,%d), want (%d,%d)", x, y, tc.wantX, tc.wantY)
			}
		})
	}
}

func TestBubblePlacement(t *testing.T) {
	area := application.Rect{X: 0, Y: 30, Width: 1792, Height: 1090}
	cases := []struct {
		name     string
		mascot   application.Rect
		area     application.Rect
		wantSide string
		wantX    int
		wantY    int
	}{
		{
			name:     "mascot on right half puts bubble on the left",
			mascot:   application.Rect{X: 1596, Y: 886, Width: 179, Height: 209},
			area:     area,
			wantSide: "left",
			wantX:    1369,
			wantY:    889,
		},
		{
			name:     "mascot on left half puts bubble on the right",
			mascot:   application.Rect{X: 40, Y: 500, Width: 179, Height: 210},
			area:     area,
			wantSide: "right",
			wantX:    186,
			wantY:    503,
		},
		{
			name:     "bubble clamps inside a small work area",
			mascot:   application.Rect{X: 0, Y: 0, Width: 180, Height: 210},
			area:     application.Rect{Width: 300, Height: 300},
			wantSide: "right",
			wantX:    32,
			wantY:    8,
		},
		{
			name:     "negative origin on a left-side display",
			mascot:   application.Rect{X: -2440, Y: 600, Width: 180, Height: 210},
			area:     application.Rect{X: -2560, Y: 0, Width: 2560, Height: 1440},
			wantSide: "right",
			wantX:    -2294,
			wantY:    603,
		},
		{
			name:     "visual gap stays tight at 1.5x scale",
			mascot:   application.Rect{X: 1200, Y: 500, Width: 540, Height: 630},
			area:     area,
			wantSide: "left",
			wantX:    1043,
			wantY:    650,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			side, bounds := bubblePlacement(tc.mascot, tc.area)
			if side != tc.wantSide {
				t.Fatalf("bubblePlacement() side = %q, want %q", side, tc.wantSide)
			}
			if bounds.X != tc.wantX || bounds.Y != tc.wantY {
				t.Fatalf("bubblePlacement() = (%d,%d), want (%d,%d)", bounds.X, bounds.Y, tc.wantX, tc.wantY)
			}
			if bounds.Width != bubbleWidth || bounds.Height != bubbleHeight {
				t.Fatalf("bubblePlacement() size = %dx%d, want %dx%d", bounds.Width, bounds.Height, bubbleWidth, bubbleHeight)
			}
		})
	}
}
