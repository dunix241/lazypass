package shared

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/uniseg"
)

type Action struct {
	OnClick  func()
	Key      *tcell.EventKey
	Priority int
	Label    string
	Hint     string
	Match    func(*tcell.EventKey) bool
	Enabled  func() bool
	Invoke   func()
}

// Footer shares action presentation between panel and screen-level placements.
// Actions are kept whole; narrower layouts omit trailing actions instead of
// truncating a key binding halfway through.
type Footer struct {
	Text    string
	Actions []Action
	Color   tcell.Color
}

func (f Footer) TextForWidth(width int) string {
	if len(f.Actions) == 0 {
		return Truncate(f.Text, width)
	}

	var text strings.Builder
	used := 0
	for _, action := range f.Actions {
		label := strings.TrimSpace(action.Hint + " " + action.Label)
		if label == "" {
			continue
		}
		labelWidth := uniseg.StringWidth(label)
		separatorWidth := 0
		if text.Len() > 0 {
			separatorWidth = 5
		}

		if used+separatorWidth+labelWidth > width {
			break
		}

		if separatorWidth > 0 {
			text.WriteString("  •  ")
		}
		text.WriteString(label)
		used += separatorWidth + labelWidth
	}

	return text.String()
}

func (f Footer) Draw(screen tcell.Screen, r Rect) {
	text := f.TextForWidth(r.W)
	PrintAt(screen, r.X+max(0, (r.W-uniseg.StringWidth(text))/2), r.Y, text, f.Color)
}

func HandleAction(event *tcell.EventKey, actions []Action) bool {
	for _, action := range actions {
		if action.Match != nil && action.Match(event) {
			if (action.Enabled == nil || action.Enabled()) && action.Invoke != nil {
				action.Invoke()
			}
			return true
		}
	}
	return false
}

type Panel struct {
	Bounds                                      Rect
	Title                                       string
	Border, Background, TitleColor, FooterColor tcell.Color
	Actions                                     []Action
	Footer                                      string
}

func CenteredPanel(viewport Rect, width, height int) Rect {
	width = min(width, max(0, viewport.W-2))
	height = min(height, max(0, viewport.H-2))
	return Rect{X: viewport.X + (viewport.W-width)/2, Y: viewport.Y + (viewport.H-height)/2, W: width, H: height}
}

func (p Panel) Draw(screen tcell.Screen, render func(Rect)) {
	r := p.Bounds
	if r.W < 2 || r.H < 2 {
		return
	}

	DrawBox(screen, r, p.Border, p.Background)
	if p.Title != "" {
		PrintAt(screen, r.X+2, r.Y, Truncate(" "+p.Title+" ", r.W-4), p.TitleColor)
	}

	content := Rect{X: r.X + 2, Y: r.Y + 1, W: max(0, r.W-4), H: max(0, r.H-2)}
	footer := Footer{Text: p.Footer, Actions: p.Actions, Color: p.FooterColor}
	if footer.Text != "" || len(footer.Actions) > 0 {
		content.H = max(0, content.H-2)
		footer.Draw(screen, Rect{X: r.X + 2, Y: r.Y + r.H - 2, W: max(0, r.W-4), H: 1})
	}

	if render != nil {
		render(content)
	}
}

func ActionBar(screen tcell.Screen, r Rect, actions []Action, color tcell.Color) {
	Footer{Actions: actions, Color: color}.Draw(screen, r)
}

func PrintActionText(screen tcell.Screen, r Rect, text string, color tcell.Color) {
	Footer{Text: text, Color: color}.Draw(screen, r)
}
