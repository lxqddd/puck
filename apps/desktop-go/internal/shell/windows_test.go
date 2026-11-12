package shell

import (
	"math"
	"testing"
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
		{in: math.NaN(), want: 1},
	}
	for _, tc := range cases {
		if got := clampMascotScale(tc.in); got != tc.want {
			t.Fatalf("clampMascotScale(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
