package input

import (
	"errors"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func key(e *Editor, k tcell.Key, options Options) {
	_, _ = e.HandleKey(tcell.NewEventKey(k, 0, tcell.ModNone), options)
}

func TestCursorInsertionAndGraphemeDeletion(t *testing.T) {
	var e Editor
	e.Sync("a🙂e\u0301z")
	key(&e, tcell.KeyLeft, Options{})
	key(&e, tcell.KeyBackspace2, Options{})
	if e.Value() != "a🙂z" {
		t.Fatalf("split grapheme: %q", e.Value())
	}
	if err := e.Insert("X", Options{}); err != nil {
		t.Fatal(err)
	}
	if e.Value() != "a🙂Xz" {
		t.Fatal(e.Value())
	}
	key(&e, tcell.KeyHome, Options{})
	key(&e, tcell.KeyDelete, Options{})
	if e.Value() != "🙂Xz" {
		t.Fatal(e.Value())
	}
}

func TestPasteLimitAndMultiline(t *testing.T) {
	var e Editor
	if err := e.Insert("a\r\nb\x00", Options{Multiline: true, MaxBytes: 3}); err != nil {
		t.Fatal(err)
	}
	if e.Value() != "a\nb" {
		t.Fatal(e.Value())
	}
	if err := e.Insert("x", Options{MaxBytes: 3}); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
	key(&e, tcell.KeyHome, Options{})
	if e.cursor != 2 {
		t.Fatal("home must remain on logical line")
	}
}

func TestViewportFollowsCursorAndMasks(t *testing.T) {
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(20, 10)
	var e Editor
	e.Sync(strings.Repeat("x", 30) + "end")
	e.Draw(s, 1, 1, 8, 1, tcell.StyleDefault, true, Options{})
	x, y, visible := s.GetCursor()
	if !visible || x < 1 || x >= 9 || y != 1 {
		t.Fatalf("cursor outside viewport: %d %d %t", x, y, visible)
	}
	text, _, _ := s.Get(x-1, y)
	if text != "d" {
		t.Fatalf("tail not visible: %q", text)
	}
	key(&e, tcell.KeyHome, Options{})
	e.Draw(s, 1, 1, 8, 1, tcell.StyleDefault, true, Options{Secret: true})
	text, _, _ = s.Get(1, 1)
	if text != "*" {
		t.Fatal("password leaked")
	}
	x, _, _ = s.GetCursor()
	if x != 1 {
		t.Fatal("cursor did not follow home")
	}
}

func TestWrappedVerticalMovement(t *testing.T) {
	var e Editor
	e.Sync("abcdefghijkl")
	e.width = 4
	options := Options{Multiline: true}
	key(&e, tcell.KeyUp, options)
	if e.cursor != 8 {
		t.Fatalf("up: %d", e.cursor)
	}
	key(&e, tcell.KeyDown, options)
	if e.cursor != 12 {
		t.Fatalf("down: %d", e.cursor)
	}
}

func TestEmptyPasteDoesNotMoveCursor(t *testing.T) {
	var e Editor
	e.Sync("abc")
	key(&e, tcell.KeyHome, Options{})
	if err := e.Insert("\x00\t", Options{}); err != nil {
		t.Fatal(err)
	}
	if e.cursor != 0 || e.Value() != "abc" {
		t.Fatalf("empty paste changed editor: cursor=%d value=%q", e.cursor, e.Value())
	}
}

func TestVerticalNavigationRetainsPreferredColumn(t *testing.T) {
	var e Editor
	e.Sync("abcdef\nx\nabcdef")
	e.width = 20
	options := Options{Multiline: true}
	key(&e, tcell.KeyUp, options)
	key(&e, tcell.KeyUp, options)
	if e.cursor != 6 {
		t.Fatalf("preferred column lost across short line: %d", e.cursor)
	}
}

func BenchmarkCachedViewport(b *testing.B) {
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		b.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(80, 24)
	var e Editor
	e.Sync(strings.Repeat("long entry text ", 4000))
	options := Options{Multiline: true}
	e.Draw(s, 1, 1, 40, 4, tcell.StyleDefault, true, options)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Draw(s, 1, 1, 40, 4, tcell.StyleDefault, true, options)
	}
}
