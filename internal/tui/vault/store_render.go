package vault

import (
	"lazypass/internal/tui/input"
	"lazypass/internal/tui/shared"
	"lazypass/internal/vault"
	"strings"

	"github.com/gdamore/tcell/v2"
)

func (s *StoreForm) Draw(screen tcell.Screen, frame shared.Frame) {
	screen.HideCursor()
	if s.qrImage != nil {
		s.drawQRDisplay(screen, frame)
		return
	}
	r, visible := s.storeFormRect(frame.Viewport)
	colors := frame.Palette
	title := " Store generated password "
	if s.manual {
		title = " Add to vault "
	}
	if s.loadingEntry || s.editEntry != nil {
		title = " Edit entry "
	}
	shared.Panel{Bounds: r, Title: strings.TrimSpace(title), TitleColor: colors.Accent, Border: colors.Focus, Background: colors.Surface}.Draw(screen, nil)
	if s.qrImport {
		shared.PrintAt(screen, r.X+2, r.Y+2, "Import authenticator QR image", colors.Accent)
		shared.PrintAt(screen, r.X+2, r.Y+4, "PNG/JPEG file path:", colors.Muted)
		_, editor, options := s.activeInput()
		editor.Draw(screen, r.X+2, r.Y+5, r.W-4, 1, tcell.StyleDefault.Foreground(colors.Text).Background(colors.Surface), !s.saving, options)
		hint := shared.FitHints(r.W-4, "Enter file  •  Ctrl-V image  •  Esc back", "Ctrl-V image  •  Esc back")
		if s.saving {
			hint = "Importing…  •  Esc cancel"
		}
		shared.PrintPanelFooter(screen, r, hint, colors.Muted)
		return
	}
	if s.locationOnly && s.deps.LocationLabel != nil {
		shared.PrintAt(screen, r.X+2, r.Y+1, shared.Truncate(s.deps.LocationLabel(s.location), r.W-4), colors.Muted)
	}
	if s.loadingEntry {
		shared.PrintAt(screen, r.X+2, r.Y+3, "Loading entry…", colors.Muted)
		return
	}
	if s.choosingType {
		for i, action := range s.createActions() {
			color := colors.Text
			prefix := "  "
			if i == s.actionIndex {
				color = colors.Focus
				prefix = "> "
			}
			shared.PrintAt(screen, r.X+2, r.Y+2+i, prefix+action.label, color)
		}
		shared.PrintPanelFooter(screen, r, "↑/↓ select", colors.Muted)
		return
	}
	if s.loadingEntries {
		shared.PrintAt(screen, r.X+2, r.Y+3, "Loading entries…", colors.Muted)
		return
	}
	if s.editingValue {
		label := s.storePath
		if s.deps.LocationLabel != nil {
			path, err := vault.ParsePath(s.storePath)
			if err == nil {
				if len(path) > 0 {
					label = s.deps.LocationLabel(path[:len(path)-1]) + "/" + path[len(path)-1]
				}
			}
		}
		shared.PrintAt(screen, r.X+2, r.Y+2, shared.Truncate("Entry: "+label, r.W-4), colors.Muted)
		fields := s.entryFields()
		for _, row := range s.fieldRows(r) {
			i := row.index
			field := fields[i]
			color := colors.Text
			focused := i == s.field
			if focused {
				color = colors.Focus
			}
			label := field.label + ": "
			y := row.y
			shared.PrintAt(screen, r.X+2, y, label, color)
			x, width := r.X+2+len(label), s.fieldWidth(r, field)
			options := field.inputOptions()
			style := tcell.StyleDefault.Foreground(color).Background(colors.Surface)
			for dy := 0; dy < row.height; dy++ {
				for dx := 0; dx < width; dx++ {
					screen.SetContent(x+dx, y+dy, ' ', nil, style)
				}
			}
			if focused || field.multiline {
				s.editor(field.key, s.displayField(i)).Draw(screen, x, y, width, row.height, style, focused, options)
			} else {
				input.Preview(screen, x, y, width, s.displayField(i), style, options)
			}
		}
		hint := "<tab> field"
		if fields[s.field].key == "authenticator" {
			hint += "  •  F2 import  •  F3 show QR"
		}
		if fields[s.field].multiline {
			hint += "  •  <ctrl-n> newline"
		}
		shared.PrintPanelFooter(screen, r, shared.FitHints(r.W-4, hint, "<tab> field"), colors.Muted)
		return
	}
	shared.PrintAt(screen, r.X+2, r.Y+2, "Destination (folder/name):", colors.Muted)
	if s.locationOnly {
		label := "Select location (Tab/Enter; Shift-Tab up):"
		if s.selectedLocation() {
			label = "Entry name (Shift-Tab changes location):"
		}
		shared.PrintAt(screen, r.X+2, r.Y+2, shared.Truncate(label, r.W-4), colors.Muted)
	}
	_, editor, options := s.activeInput()
	style := tcell.StyleDefault.Foreground(colors.Text).Background(colors.Surface)
	for dx := 0; dx < r.W-4; dx++ {
		screen.SetContent(r.X+2+dx, r.Y+3, ' ', nil, style)
	}
	editor.Draw(screen, r.X+2, r.Y+3, r.W-4, 1, style, true, options)
	if s.storeMessage != "" {
		shared.PrintAt(screen, r.X+2, r.Y+5, shared.Truncate(s.storeMessage, r.W-4), colors.Warning)
	} else if visible > 0 {
		start, end := s.suggestionWindow(visible)
		for index := start; index < end; index++ {
			node := s.storeNodes[index]
			prefix := "  "
			if index == s.storeSelected {
				prefix = "> "
			}
			shared.PrintAt(screen, r.X+2, r.Y+5+index-start, shared.Truncate(prefix+nodeLabel(node, frame.NerdFont), r.W-4), nodeColor(node, colors))
		}
	}
	help := storeFormHelp(r.W - 4)
	shared.PrintAt(screen, r.X+2, r.Y+r.H-2, help, colors.Muted)
}

func storeFormHelp(width int) string {
	return shared.FitHints(width, "↑/↓ select  •  <tab> folder", "↑/↓ select  •  <tab>", "<tab> folder")
}

func (s *StoreForm) storeFormRect(viewport shared.Rect) (shared.Rect, int) {
	w := min(52, viewport.W-4)
	maxHeight := viewport.H - 4
	if s.choosingType {
		h := min(4+len(s.createActions()), maxHeight)
		return shared.Rect{X: viewport.X + (viewport.W-w)/2, Y: viewport.Y + (viewport.H-h)/2, W: w, H: h}, 0
	}
	if s.editingValue {
		h := min(11, maxHeight)
		return shared.Rect{X: viewport.X + (viewport.W-w)/2, Y: viewport.Y + (viewport.H-h)/2, W: w, H: h}, 0
	}
	visible := min(5, min(max(0, maxHeight-8), len(s.storeNodes)))
	return shared.Rect{X: viewport.X + (viewport.W-w)/2, Y: viewport.Y + max(2, (viewport.H-(8+visible))/2), W: w, H: 8 + visible}, visible
}
