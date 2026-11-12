//go:build darwin

package cursor

/*
#cgo LDFLAGS: -framework ApplicationServices
#include <ApplicationServices/ApplicationServices.h>

static int puckMouseLocation(double *x, double *y) {
	CGEventRef event = CGEventCreate(NULL);
	if (event == NULL) {
		return 0;
	}
	CGPoint point = CGEventGetLocation(event);
	CFRelease(event);
	*x = point.x;
	*y = point.y;
	return 1;
}
*/
import "C"

type Point struct {
	X int
	Y int
}

func Position() (Point, bool) {
	var x, y C.double
	if C.puckMouseLocation(&x, &y) == 0 {
		return Point{}, false
	}
	return Point{X: int(x), Y: int(y)}, true
}
