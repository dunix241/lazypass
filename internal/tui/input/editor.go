// Package input provides UI-thread text editing independent of forms and storage.
package input

import (
	"errors"
	"sort"
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/uniseg"
)

var ErrLimit = errors.New("input byte limit exceeded")

type Options struct {
	WordSeparators string
	Multiline      bool
	Secret         bool
	Reveal         bool
	MaxBytes       int
}

// Editor stores byte offsets at grapheme boundaries. Its zero value is usable.
// Sync binds an external value; changed values reset the cursor to the end.
type Editor struct {
	value         string
	cursor        int
	left, top     int
	width         int
	layoutValue   string
	layoutWidth   int
	layoutOptions Options
	cells         []cell
	boundaryValue string
	boundaries    []int
	preferredX    int
	vertical      bool
}

func (e *Editor) Sync(value string) {
	if value != e.value {
		e.value, e.cursor, e.left, e.top = value, len(value), 0, 0
		e.vertical = false
	}
}
func (e *Editor) Value() string { return e.value }

func (e *Editor) Insert(text string, options Options) error {
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	text = strings.Map(func(r rune) rune {
		if r == '\n' && options.Multiline {
			return r
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, text)
	if text == "" {
		return nil
	}
	if options.MaxBytes > 0 && len(e.value)+len(text) > options.MaxBytes {
		return ErrLimit
	}
	e.replace(e.cursor, e.cursor, text)
	return nil
}

func (e *Editor) positions() []int {
	if e.boundaries == nil || e.boundaryValue != e.value {
		e.boundaries = append(e.boundaries[:0], 0)
		g := uniseg.NewGraphemes(e.value)
		for g.Next() {
			_, end := g.Positions()
			e.boundaries = append(e.boundaries, end)
		}
		e.boundaryValue = e.value
	}
	return e.boundaries
}

func (e *Editor) replace(start, end int, text string) {
	e.value = e.value[:start] + text + e.value[end:]
	positions := e.positions()
	e.cursor = positions[sort.SearchInts(positions, start+len(text))]
	e.vertical = false
}

func (e *Editor) previous() int {
	positions := e.positions()
	return positions[max(0, sort.SearchInts(positions, e.cursor)-1)]
}
func (e *Editor) next() int {
	positions := e.positions()
	return positions[min(len(positions)-1, sort.SearchInts(positions, e.cursor)+1)]
}

func (e *Editor) HandleKey(event *tcell.EventKey, options Options) (bool, error) {
	if event.Key() != tcell.KeyUp && event.Key() != tcell.KeyDown {
		e.vertical = false
	}
	switch event.Key() {
	case tcell.KeyLeft:
		e.cursor = e.previous()
	case tcell.KeyRight:
		e.cursor = e.next()
	case tcell.KeyHome:
		e.cursor = strings.LastIndex(e.value[:e.cursor], "\n") + 1
	case tcell.KeyEnd:
		if end := strings.IndexByte(e.value[e.cursor:], '\n'); end >= 0 {
			e.cursor += end
		} else {
			e.cursor = len(e.value)
		}
	case tcell.KeyCtrlA:
		e.cursor = 0
	case tcell.KeyCtrlE:
		e.cursor = len(e.value)
	case tcell.KeyCtrlU:
		e.replace(0, len(e.value), "")
	case tcell.KeyCtrlW:
		end := e.cursor
		positions := e.positions()
		word := false
		for i := sort.SearchInts(positions, end) - 1; i >= 0; i-- {
			text := e.value[positions[i]:e.cursor]
			space := strings.TrimSpace(text) == ""
			if word && space || !space && strings.ContainsAny(text, options.WordSeparators) {
				break
			}
			word = word || !space
			e.cursor = positions[i]
		}
		e.replace(e.cursor, end, "")
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		e.replace(e.previous(), e.cursor, "")
	case tcell.KeyDelete:
		e.replace(e.cursor, e.next(), "")
	case tcell.KeyUp, tcell.KeyDown:
		if !options.Multiline {
			return false, nil
		}
		e.moveVertical(event.Key() == tcell.KeyDown, options)
	case tcell.KeyCtrlN:
		if !options.Multiline {
			return false, nil
		}
		return true, e.Insert("\n", options)
	case tcell.KeyRune:
		if event.Modifiers()&(tcell.ModAlt|tcell.ModCtrl) != 0 {
			return false, nil
		}
		return true, e.Insert(string(event.Rune()), options)
	default:
		return false, nil
	}
	return true, nil
}
