package shell

import "testing"

func TestHitTest(t *testing.T) {
	region := Region{X: 40, Y: 48, Width: 280, Height: 340}
	windowX, windowY := 100, 200

	cases := []struct {
		name string
		x, y int
		want bool
	}{
		{name: "outside left", x: 120, y: 300, want: false},
		{name: "outside top", x: 200, y: 230, want: false},
		{name: "inside", x: 200, y: 300, want: true},
		{name: "inside bottom right corner", x: 420, y: 588, want: true},
		{name: "outside bottom", x: 200, y: 600, want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := hitTest(tc.x, tc.y, windowX, windowY, region); got != tc.want {
				t.Fatalf("hitTest(%d,%d) = %v, want %v", tc.x, tc.y, got, tc.want)
			}
		})
	}
}
