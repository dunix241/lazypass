package shared

import (
	"lazypass/internal/tui/input"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type ConfirmationOptions struct {
	Title        string
	Message      string
	RequireText  string
	ConfirmLabel string
	Destructive  bool
	Width        int
	OnConfirm    func()
	OnCancel     func()
}

// Confirmation owns dialog interaction; callers only update message/busy state
// while asynchronous preparation runs. All methods belong to the UI thread.
type Confirmation struct {
	Options ConfirmationOptions
	Busy    bool
	closed  bool
	editor  input.Editor
}

func NewConfirmation(options ConfirmationOptions) *Confirmation {
	if options.ConfirmLabel == "" {
		options.ConfirmLabel = "yes"
	}
	if options.Width <= 0 {
		options.Width = 60
	}

	return &Confirmation{Options: options}
}

func (c *Confirmation) cancel() {
	if c.closed {
		return
	}

	c.closed = true
	if c.Options.OnCancel != nil {
		c.Options.OnCancel()
	}
}

func (c *Confirmation) confirm() {
	if c.closed || !c.canConfirm() {
		return
	}

	c.closed = true
	if c.Options.OnConfirm != nil {
		c.Options.OnConfirm()
	}
}

func (c *Confirmation) canConfirm() bool {
	return !c.Busy && (c.Options.RequireText == "" || c.editor.Value() == c.Options.RequireText)
}

func (c *Confirmation) actions() []Action {
	typed := c.Options.RequireText != ""
	hint := "y"
	if typed {
		hint = "Enter"
	}

	return []Action{
		{
			Hint:    hint,
			Label:   c.Options.ConfirmLabel,
			Enabled: c.canConfirm,
			Match: func(e *tcell.EventKey) bool {
				if typed {
					return e.Key() == tcell.KeyEnter
				}
				return e.Key() == tcell.KeyRune && (e.Rune() == 'y' || e.Rune() == 'Y')
			},
			Invoke: c.confirm,
		},
		{
			Hint:  "<esc>",
			Label: "cancel",
			Match: func(e *tcell.EventKey) bool {
				if e.Key() == tcell.KeyEscape {
					return true
				}
				return !typed && e.Key() == tcell.KeyRune && (e.Rune() == 'n' || e.Rune() == 'N' || e.Rune() == 'q')
			},
			Invoke: c.cancel,
		},
	}
}

func (c *Confirmation) HandleKey(event *tcell.EventKey) {
	if c == nil || c.closed {
		return
	}

	if HandleAction(event, c.actions()) {
		return
	}

	if !c.Busy && c.Options.RequireText != "" {
		_, _ = c.editor.HandleKey(event, c.inputOptions())
	}
}

func (c *Confirmation) Paste(text string) {
	if c != nil && !c.closed && !c.Busy && c.Options.RequireText != "" {
		_ = c.editor.Insert(text, c.inputOptions())
	}
}

func (c *Confirmation) Draw(screen tcell.Screen, frame Frame) {
	if c == nil || c.closed {
		return
	}

	screen.HideCursor()
	r, lines := c.layout(frame.Viewport)
	if r.W < 20 {
		return
	}
	color := frame.Palette.Focus
	if c.Options.Destructive {
		color = tcell.ColorRed
	}

	panel := Panel{
		Bounds:      r,
		Title:       c.Options.Title,
		Border:      color,
		TitleColor:  color,
		Background:  frame.Palette.Surface,
		FooterColor: frame.Palette.Muted,
		Actions:     c.actions(),
	}
	panel.Draw(screen, func(content Rect) {
		limit := content.H
		if c.Options.RequireText != "" {
			limit = max(0, limit-3)
		}

		for row := 0; row < len(lines) && row < limit; row++ {
			PrintAt(screen, content.X, content.Y+row, lines[row], frame.Palette.Text)
		}

		if c.Options.RequireText != "" && content.H >= 3 {
			y := content.Y + content.H - 3
			PrintAt(screen, content.X, y, Truncate("Type exactly: "+c.Options.RequireText, content.W), frame.Palette.Text)
			c.editor.Draw(screen, content.X, y+2, content.W, 1, tcell.StyleDefault.Foreground(frame.Palette.Focus).Background(frame.Palette.Surface), !c.Busy, c.inputOptions())
		}
	})
}

func (c *Confirmation) HandleMouse(action tview.MouseAction, event *tcell.EventMouse, frame Frame) {
	if c == nil || c.closed || action != tview.MouseLeftClick {
		return
	}
	r, _ := c.layout(frame.Viewport)
	if r.W < 20 {
		return
	}
	footer := Footer{Actions: c.actions()}
	x, y := event.Position()
	if item, ok := footer.HitAction(Rect{X: r.X + 2, Y: r.Y + r.H - 2, W: max(0, r.W-4), H: 1}, FooterOptions{MaxRows: 1}, x, y); ok && item.Invoke != nil {
		item.Invoke()
	}
}

func (c *Confirmation) layout(viewport Rect) (Rect, []string) {
	width := min(c.Options.Width, max(0, viewport.W-2))
	lines := WrapText(c.Options.Message, max(0, width-4))
	height := len(lines) + 5
	if c.Options.RequireText != "" {
		height += 3
	}
	return CenteredPanel(viewport, width, height), lines
}

func (c *Confirmation) inputOptions() input.Options {
	return input.Options{MaxBytes: max(256, len(c.Options.RequireText))}
}
