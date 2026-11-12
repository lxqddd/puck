//go:build !darwin

package cursor

type Point struct {
	X int
	Y int
}

func Position() (Point, bool) {
	return Point{}, false
}
