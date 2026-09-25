package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"lazypass/internal/theme"

	"github.com/gdamore/tcell/v2"
)

type themePanelMode uint8

const (
	themePicker themePanelMode = iota
	themeName
	themeEditor
	themeDeleteConfirm
)

type themePanel struct {
	open       bool
	mode       themePanelMode
	names      []string
	selected   int
	original   tuiPalette
	originalID string
	pending    theme.Palette
	name       string
	nameEdits  bool
	role       int
	colorText  string
	colorEdits bool
	previewed  bool
	error      string
}

var themeRoles = []string{"base", "surface", "border", "text", "muted", "accent", "focus", "warning"}

func (s *screen) openThemePanel() {
	names, err := theme.List()
	if err != nil {
		s.notify(notificationError, "Unable to list themes")
		return
	}
	p := themePanel{open: true, names: names, original: s.colors, originalID: s.cfg.Theme}
	for i, name := range names {
		if name == s.cfg.Theme {
			p.selected = i
			break
		}
	}
	if len(names) > 0 {
		s.previewTheme(&p, names[p.selected])
	}
	s.themePanel = p
}

func (s *screen) previewTheme(panel *themePanel, name string) bool {
	p, err := theme.Load(name)
	if err != nil {
		panel.error = fmt.Sprintf("Cannot load %s: %v", name, err)
		return false
	}
	colors, err := newTUIPalette(p)
	if err != nil {
		panel.error = err.Error()
		return false
	}
	panel.pending, panel.error, s.colors = p, "", colors
	return true
}

func (s *screen) handleThemeInput(event *tcell.EventKey) {
	p := &s.themePanel
	if event.Key() == tcell.KeyEscape {
		s.escapeThemePanel(p)
		return
	}
	if p.mode == themeName {
		s.handleThemeName(event, p)
		return
	}
	if p.mode == themeEditor {
		s.handleThemeEditor(event, p)
		return
	}
	if p.mode == themeDeleteConfirm {
		if event.Key() == tcell.KeyRune && (event.Rune() == 'y' || event.Rune() == 'Y') {
			s.deleteTheme(p)
		} else if event.Key() == tcell.KeyRune && (event.Rune() == 'n' || event.Rune() == 'N') {
			p.mode = themePicker
		}
		return
	}
	s.handleThemePicker(event, p)
}

func (s *screen) escapeThemePanel(p *themePanel) {
	switch p.mode {
	case themeName, themeEditor, themeDeleteConfirm:
		p.mode, p.error, p.colorEdits = themePicker, "", false
	case themePicker:
		if p.previewed {
			s.colors = p.original
			p.previewed = false
			for i, name := range p.names {
				if name == p.originalID {
					p.selected = i
					break
				}
			}
			return
		}
		p.open = false
	}
}

func (s *screen) handleThemePicker(event *tcell.EventKey, p *themePanel) {
	switch event.Key() {
	case tcell.KeyUp, tcell.KeyBacktab:
		if len(p.names) > 0 {
			p.selected = (p.selected - 1 + len(p.names)) % len(p.names)
			if s.previewTheme(p, p.names[p.selected]) {
				p.previewed = true
			}
		}
		return
	case tcell.KeyDown, tcell.KeyTAB:
		if len(p.names) > 0 {
			p.selected = (p.selected + 1) % len(p.names)
			if s.previewTheme(p, p.names[p.selected]) {
				p.previewed = true
			}
		}
		return
	case tcell.KeyEnter:
		if len(p.names) == 0 || !s.previewTheme(p, p.names[p.selected]) {
			return
		}
		if err := s.saveTheme(p.names[p.selected]); err != nil {
			p.error = "Saving config: " + err.Error()
			return
		}
		p.open = false
		return
	}
	if event.Key() != tcell.KeyRune {
		return
	}
	switch event.Rune() {
	case 'n':
		p.mode, p.name, p.nameEdits, p.error = themeName, "", false, ""
	case 'e':
		if len(p.names) == 0 {
			return
		}
		if theme.IsBuiltin(p.names[p.selected]) {
			p.mode, p.name, p.nameEdits, p.error = themeName, "", false, "Name the editable copy"
			return
		}
		if s.previewTheme(p, p.names[p.selected]) {
			p.mode, p.role, p.colorEdits = themeEditor, 0, false
			p.colorText = paletteColor(p.pending, 0)
		}
	case 'd':
		if len(p.names) > 0 && !theme.IsBuiltin(p.names[p.selected]) {
			p.mode, p.error = themeDeleteConfirm, ""
		}
	}
}

func (s *screen) handleThemeName(event *tcell.EventKey, p *themePanel) {
	switch event.Key() {
	case tcell.KeyCtrlU:
		p.name, p.nameEdits = "", true
	case tcell.KeyCtrlW:
		p.name, p.nameEdits = deletePreviousWord(p.name), true
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if len(p.name) > 0 {
			p.name = p.name[:len(p.name)-1]
		}
	case tcell.KeyEnter:
		if err := theme.ValidateName(p.name); err != nil {
			p.error = err.Error()
			return
		}
		if err := theme.Save(p.name, p.pending); err != nil {
			p.error = err.Error()
			return
		}
		p.names, _ = theme.List()
		for i, name := range p.names {
			if name == p.name {
				p.selected = i
				break
			}
		}
		p.error = ""
		p.mode, p.role, p.colorEdits, p.colorText = themeEditor, 0, false, paletteColor(p.pending, 0)
	default:
		if event.Key() == tcell.KeyRune && utf8.RuneCountInString(p.name) < 48 {
			if !p.nameEdits {
				p.name, p.nameEdits = "", true
			}
			p.name += string(event.Rune())
		}
	}
}

func (s *screen) handleThemeEditor(event *tcell.EventKey, p *themePanel) {
	if event.Key() == tcell.KeyRune && event.Rune() == 's' && !p.colorEdits {
		if err := theme.Save(p.names[p.selected], p.pending); err != nil {
			p.error = err.Error()
		} else {
			p.error = "Saved"
		}
		return
	}
	switch event.Key() {
	case tcell.KeyUp:
		p.role = (p.role - 1 + len(themeRoles)) % len(themeRoles)
		p.colorText, p.colorEdits = paletteColor(p.pending, p.role), false
	case tcell.KeyDown:
		p.role = (p.role + 1) % len(themeRoles)
		p.colorText, p.colorEdits = paletteColor(p.pending, p.role), false
	case tcell.KeyCtrlU:
		p.colorText, p.colorEdits = "", true
	case tcell.KeyCtrlW:
		p.colorText, p.colorEdits = deletePreviousWord(p.colorText), true
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if len(p.colorText) > 0 {
			p.colorText = p.colorText[:len(p.colorText)-1]
			p.colorEdits = true
		}
	case tcell.KeyEnter:
		candidate := p.pending
		setPaletteColor(&candidate, p.role, p.colorText)
		if err := candidate.Validate(); err != nil {
			p.error = err.Error()
			return
		}
		p.pending, p.error, p.colorEdits = candidate, "", false
		colors, _ := newTUIPalette(candidate)
		s.colors = colors
		p.previewed = true
	default:
		if event.Key() == tcell.KeyRune && (!p.colorEdits || utf8.RuneCountInString(p.colorText) < 7) {
			if !p.colorEdits {
				p.colorText, p.colorEdits = "", true
			}
			p.colorText += string(event.Rune())
		}
	}
}

func (s *screen) deleteTheme(p *themePanel) {
	name := p.names[p.selected]
	if err := theme.Delete(name); err != nil {
		p.error, p.mode = err.Error(), themePicker
		return
	}
	p.names, _ = theme.List()
	p.selected, p.mode = 0, themePicker
	if s.cfg.Theme == name {
		s.cfg.Theme = theme.DefaultName()
		s.colors = defaultTUIPalette()
		s.save()
	}
}

func paletteColor(p theme.Palette, role int) string {
	return []string{p.Base, p.Surface, p.Border, p.Text, p.Muted, p.Accent, p.Focus, p.Warning}[role]
}
func setPaletteColor(p *theme.Palette, role int, color string) {
	switch role {
	case 0:
		p.Base = color
	case 1:
		p.Surface = color
	case 2:
		p.Border = color
	case 3:
		p.Text = color
	case 4:
		p.Muted = color
	case 5:
		p.Accent = color
	case 6:
		p.Focus = color
	case 7:
		p.Warning = color
	}
}

func (s *screen) drawThemePanel(screen tcell.Screen, ox, oy, width, height int) {
	p := &s.themePanel
	w, h := min(64, width-2), min(15, height-2)
	if w < 28 || h < 8 {
		return
	}
	r := rect{ox + (width-w)/2, oy + (height-h)/2, w, h}
	drawBox(screen, r, s.colors.border, s.colors.surface)
	printAt(screen, r.x+2, r.y, " Theme ", s.colors.accent)
	if p.mode == themeName {
		printAt(screen, r.x+2, r.y+2, "Custom theme name:", s.colors.text)
		printAt(screen, r.x+2, r.y+4, "["+p.name+"]", s.colors.focus)
		printAt(screen, r.x+2, r.y+h-2, "Enter create  Esc cancel", s.colors.muted)
	} else if p.mode == themeEditor {
		printAt(screen, r.x+2, r.y+2, "Edit "+p.names[p.selected], s.colors.text)
		rows := min(len(themeRoles), h-5)
		for i := 0; i < rows; i++ {
			value := paletteColor(p.pending, i)
			color := s.colors.muted
			if i == p.role {
				value, color = "["+p.colorText+"]", s.colors.focus
			}
			printAt(screen, r.x+2, r.y+3+i, fmt.Sprintf("%-8s %s", themeRoles[i], value), color)
		}
		printAt(screen, r.x+2, r.y+h-2, "Enter validate  s save  Esc back", s.colors.muted)
	} else if p.mode == themeDeleteConfirm {
		printAt(screen, r.x+2, r.y+3, "Delete "+p.names[p.selected]+"? (y/n)", s.colors.warning)
	} else {
		printAt(screen, r.x+2, r.y+2, "Up/Down preview  Enter apply  n new  e edit  d delete", s.colors.muted)
		rows := min(len(p.names), h-6)
		for i := 0; i < rows; i++ {
			prefix, color := "  ", s.colors.text
			if i == p.selected {
				prefix, color = "> ", s.colors.focus
			}
			kind := "custom"
			if theme.IsBuiltin(p.names[i]) {
				kind = "built-in"
			}
			printAt(screen, r.x+2, r.y+3+i, prefix+p.names[i]+" ("+kind+")", color)
		}
		printAt(screen, r.x+2, r.y+h-2, "Esc discard preview, then close", s.colors.muted)
	}
	if p.error != "" {
		printAt(screen, r.x+2, r.y+h-1, truncate(strings.ReplaceAll(p.error, "\n", " "), w-4), s.colors.warning)
	}
}
