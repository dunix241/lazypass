package shared

// Rect uses absolute terminal coordinates.
type Rect struct{ X, Y, W, H int }

func (r Rect) Contains(x, y int) bool {
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

func (r Rect) Translate(x, y int) Rect {
	r.X += x
	r.Y += y
	return r
}

// Frame is the common geometry and presentation snapshot for a UI event.
type Frame struct {
	Viewport       Rect
	Card           Rect
	Compact, Short bool
	Palette        Palette
	NerdFont       bool
}

// MouseResult lets the root primitive retain ownership of mouse capture.
type MouseResult struct {
	Consumed bool
	Capture  bool
}
