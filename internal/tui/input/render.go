package input

import (
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/uniseg"
)

type cell struct {
	start, x, y, width int
	text               string
}

func (e *Editor) layout(width int, options Options) ([]cell, int, int) {
	if e.layoutValue == e.value && e.layoutWidth == width && e.layoutOptions == options {
		x, y := e.cursorPosition(e.cells, width, options)
		return e.cells, x, y
	}
	cells := e.cells[:0]
	x, y := 0, 0
	g := uniseg.NewGraphemes(e.value)
	for g.Next() {
		start, _ := g.Positions()
		text, size := g.Str(), g.Width()
		newline := text == "\n"
		if options.Secret && !options.Reveal && !newline {
			text, size = "*", 1
		}
		if newline && !options.Multiline {
			text, size, newline = "↵", 1, false
		}
		if !newline && options.Multiline && x+size > width && x > 0 {
			x = 0
			y++
		}
		cells = append(cells, cell{start, x, y, size, text})
		if newline {
			x = 0
			y++
		} else {
			x += size
		}
	}
	e.cells, e.layoutValue, e.layoutWidth, e.layoutOptions = cells, e.value, width, options
	x, y = e.cursorPosition(cells, width, options)
	return cells, x, y
}

func (e *Editor) cursorPosition(cells []cell, width int, options Options) (int, int) {
	i := sort.Search(len(cells), func(i int) bool { return cells[i].start >= e.cursor })
	if i < len(cells) {
		return cells[i].x, cells[i].y
	}
	if len(cells) == 0 {
		return 0, 0
	}
	return endPosition(cells, width, options.Multiline)
}

func endPosition(cells []cell, width int, multiline bool) (int, int) {
	if len(cells) == 0 {
		return 0, 0
	}
	c := cells[len(cells)-1]
	x, y := c.x+c.width, c.y
	if c.text == "\n" || multiline && x >= width {
		x = 0
		y++
	}
	return x, y
}

func (e *Editor) moveVertical(down bool, options Options) {
	cells, x, y := e.layout(max(1, e.width), options)
	if !e.vertical {
		e.preferredX = x
		e.vertical = true
	}
	x = e.preferredX
	if down {
		y++
	} else {
		y--
	}
	if y < 0 {
		return
	}
	best, distance := -1, int(^uint(0)>>1)
	start := sort.Search(len(cells), func(i int) bool { return cells[i].y >= y })
	for _, c := range cells[start:] {
		if c.y != y {
			break
		}
		d := abs(c.x - x)
		if d < distance {
			best, distance = c.start, d
		}
	}
	endX, endY := endPosition(cells, max(1, e.width), true)
	if endY == y && abs(endX-x) < distance {
		best = len(e.value)
	}
	if best >= 0 {
		e.cursor = best
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// Draw follows the cursor inside the viewport. The caller owns clearing, focus,
// and form navigation; only the focused editor requests a terminal cursor.
func (e *Editor) Draw(screen tcell.Screen, x, y, width, height int, style tcell.Style, focused bool, options Options) {
	if width <= 0 || height <= 0 {
		return
	}
	e.width = width
	cells, cx, cy := e.layout(width, options)
	if focused {
		if options.Multiline {
			e.left = 0
			e.top = max(0, min(e.top, cy))
			if cy >= e.top+height {
				e.top = cy - height + 1
			}
		} else {
			e.top = 0
			e.left = max(0, min(e.left, cx))
			if cx >= e.left+width {
				e.left = cx - width + 1
			}
		}
	}
	start := sort.Search(len(cells), func(i int) bool { return cells[i].y >= e.top && (options.Multiline || cells[i].x >= e.left) })
	for _, c := range cells[start:] {
		if c.y >= e.top+height || !options.Multiline && c.x >= e.left+width {
			break
		}
		if c.text == "\n" || c.x < e.left || c.x+c.width > e.left+width {
			continue
		}
		screen.Put(x+c.x-e.left, y+c.y-e.top, c.text, style)
	}
	if focused {
		screen.ShowCursor(x+cx-e.left, y+cy-e.top)
	}
}

// Rows includes soft wrapping and the trailing cursor line after a full row.
func (e *Editor) Rows(width int, options Options) int {
	cells, _, _ := e.layout(max(1, width), options)
	_, y := endPosition(cells, max(1, width), options.Multiline)
	return y + 1
}

// Preview uses the same masking and layout rules without retaining cursor state.
func Preview(screen tcell.Screen, x, y, width int, value string, style tcell.Style, options Options) {
	value = strings.ReplaceAll(value, "\n", " ↵ ")
	options.Multiline = false
	g := uniseg.NewGraphemes(value)
	used := 0
	for g.Next() {
		text, size := g.Str(), g.Width()
		if options.Secret && !options.Reveal {
			text, size = "*", 1
		}
		if used+size > width {
			break
		}
		screen.Put(x+used, y, text, style)
		used += size
	}
}
