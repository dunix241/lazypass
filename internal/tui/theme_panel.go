package tui

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"lazypass/internal/debug"
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
	filtering  bool
	query      string
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
		debug.Failure("theme list", debug.IO)
		s.notify(notificationError, "Unable to list themes")
		return
	}
	p := themePanel{open: true, names: names, original: s.colors, originalID: s.cfg.Theme}
	p.selected = max(0, slices.Index(names, s.cfg.Theme))
	if len(names) > 0 {
		s.previewTheme(&p, names[p.selected])
	}
	s.themePanel = p
}

func (s *screen) previewTheme(panel *themePanel, name string) bool {
	p, err := theme.Load(name)
	if err != nil {
		debug.Failure("theme preview load", debug.Invalid)
		panel.error = fmt.Sprintf("Cannot load %s: %v", name, err)
		return false
	}
	colors, err := newTUIPalette(p)
	if err != nil {
		debug.Failure("theme preview palette", debug.Invalid)
		panel.error = err.Error()
		return false
	}
	panel.pending, panel.error, s.colors = p, "", colors
	return true
}

func (s *screen) handleThemeInput(event *tcell.EventKey) {
	p := &s.themePanel
	if p.mode == themePicker && !p.filtering && event.Key() == tcell.KeyRune && event.Rune() == 'q' {
		s.colors = p.original
		p.open = false
		return
	}
	if event.Key() == tcell.KeyEscape {
		if p.mode == themePicker && p.filtering {
			p.filtering, p.query = false, ""
			return
		}
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
			p.selected = max(0, slices.Index(p.names, p.originalID))
			return
		}
		p.open = false
	}
}

func (s *screen) handleThemePicker(event *tcell.EventKey, p *themePanel) {
	if p.filtering {
		s.handleThemeFilter(event, p)
		return
	}
	switch event.Key() {
	case tcell.KeyUp, tcell.KeyBacktab:
		s.moveThemeSelection(p, -1)
		return
	case tcell.KeyDown, tcell.KeyTAB:
		s.moveThemeSelection(p, 1)
		return
	case tcell.KeyEnter:
		s.applyThemeSelection(p)
		return
	}
	if event.Key() != tcell.KeyRune {
		return
	}
	switch event.Rune() {
	case '/':
		p.filtering, p.query = true, ""
	case 'j':
		s.moveThemeSelection(p, 1)
	case 'k':
		s.moveThemeSelection(p, -1)
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
			p.colorText = *paletteRole(&p.pending, 0)
		}
	case 'd':
		if len(p.names) > 0 && !theme.IsBuiltin(p.names[p.selected]) {
			p.mode, p.error = themeDeleteConfirm, ""
		}
	}
}

func (p *themePanel) matchingThemes() []int {
	matches := make([]int, 0, len(p.names))
	query := strings.ToLower(p.query)
	for i, name := range p.names {
		if !p.filtering || strings.Contains(strings.ToLower(name), query) {
			matches = append(matches, i)
		}
	}
	return matches
}

func (s *screen) updateThemeFilter(p *themePanel, query string) {
	p.query = query
	matches := p.matchingThemes()
	if len(matches) == 0 {
		return
	}
	for _, i := range matches {
		if i == p.selected {
			return
		}
	}
	p.selected = matches[0]
	if s.previewTheme(p, p.names[p.selected]) {
		p.previewed = true
	}
}

func (s *screen) handleThemeFilter(event *tcell.EventKey, p *themePanel) {
	switch event.Key() {
	case tcell.KeyUp, tcell.KeyDown, tcell.KeyTAB, tcell.KeyBacktab:
		delta := 1
		if event.Key() == tcell.KeyUp || event.Key() == tcell.KeyBacktab {
			delta = -1
		}
		s.moveThemeSelection(p, delta)
	case tcell.KeyEnter:
		s.applyThemeSelection(p)
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if p.query != "" {
			runes := []rune(p.query)
			s.updateThemeFilter(p, string(runes[:len(runes)-1]))
		}
	case tcell.KeyCtrlU:
		s.updateThemeFilter(p, "")
	case tcell.KeyCtrlW:
		s.updateThemeFilter(p, deletePreviousWord(p.query))
	case tcell.KeyRune:
		if event.Rune() >= ' ' {
			s.updateThemeFilter(p, p.query+string(event.Rune()))
		}
	}
}

func (s *screen) applyThemeSelection(p *themePanel) {
	if len(p.matchingThemes()) == 0 || !s.previewTheme(p, p.names[p.selected]) {
		return
	}
	if err := s.saveTheme(p.names[p.selected]); err != nil {
		debug.Failure("theme apply", debug.IO)
		p.error = "Saving config: " + err.Error()
		return
	}
	p.open = false
}

func (s *screen) moveThemeSelection(p *themePanel, delta int) {
	matches := p.matchingThemes()
	if len(matches) == 0 {
		return
	}
	position := 0
	for i, index := range matches {
		if index == p.selected {
			position = i
			break
		}
	}
	p.selected = matches[(position+delta+len(matches))%len(matches)]
	if s.previewTheme(p, p.names[p.selected]) {
		p.previewed = true
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
			runes := []rune(p.name)
			p.name = string(runes[:len(runes)-1])
		}
	case tcell.KeyEnter:
		if err := theme.ValidateName(p.name); err != nil {
			debug.Failure("theme name", debug.Invalid)
			p.error = err.Error()
			return
		}
		if err := theme.Save(p.name, p.pending); err != nil {
			debug.Failure("theme create", debug.IO)
			p.error = err.Error()
			return
		}
		names, err := theme.List()
		if err != nil {
			debug.Failure("theme list", debug.IO)
			p.error = "Listing themes: " + err.Error()
			return
		}
		p.names = names
		p.selected = slices.Index(p.names, p.name)
		if p.selected < 0 {
			debug.Failure("theme create", debug.NotFound)
			p.selected, p.error = 0, "Created theme is missing from the list"
			return
		}
		p.error = ""
		p.mode, p.role, p.colorEdits, p.colorText = themeEditor, 0, false, *paletteRole(&p.pending, 0)
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
			debug.Failure("theme save", debug.IO)
			p.error = err.Error()
		} else {
			p.error = "Saved"
		}
		return
	}
	switch event.Key() {
	case tcell.KeyUp:
		p.role = (p.role - 1 + len(themeRoles)) % len(themeRoles)
		p.colorText, p.colorEdits = *paletteRole(&p.pending, p.role), false
	case tcell.KeyDown:
		p.role = (p.role + 1) % len(themeRoles)
		p.colorText, p.colorEdits = *paletteRole(&p.pending, p.role), false
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
		*paletteRole(&candidate, p.role) = p.colorText
		if err := candidate.Validate(); err != nil {
			debug.Failure("theme color", debug.Invalid)
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
		debug.Failure("theme delete", debug.IO)
		p.error, p.mode = err.Error(), themePicker
		return
	}
	names, err := theme.List()
	if err != nil {
		debug.Failure("theme list", debug.IO)
		p.names = append(p.names[:p.selected], p.names[p.selected+1:]...)
		p.error = "Listing themes: " + err.Error()
	} else {
		p.names, p.error = names, ""
	}
	p.mode, p.previewed = themePicker, false
	if s.cfg.Theme == name {
		s.cfg.Theme = theme.DefaultName()
		s.colors = defaultTUIPalette()
		if err := s.saveTheme(s.cfg.Theme); err != nil {
			debug.Failure("theme fallback save", debug.IO)
			p.error = "Saving config: " + err.Error()
		}
		p.original, p.originalID = s.colors, s.cfg.Theme
	} else {
		s.colors = p.original
	}
	p.selected = max(0, slices.Index(p.names, p.originalID))
	if pending, err := theme.Load(p.originalID); err == nil {
		p.pending = pending
	} else {
		debug.Failure("theme fallback load", debug.Invalid)
		p.error = "Loading theme: " + err.Error()
	}
}

func paletteRole(p *theme.Palette, role int) *string {
	switch role {
	case 0:
		return &p.Base
	case 1:
		return &p.Surface
	case 2:
		return &p.Border
	case 3:
		return &p.Text
	case 4:
		return &p.Muted
	case 5:
		return &p.Accent
	case 6:
		return &p.Focus
	case 7:
		return &p.Warning
	}
	return nil
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
	footer := ""
	switch p.mode {
	case themeName:
		printAt(screen, r.x+2, r.y+2, "Custom theme name:", s.colors.text)
		printAt(screen, r.x+2, r.y+4, truncate("["+p.name+"]", w-4), s.colors.focus)
		footer = "<enter> create"
	case themeEditor:
		printAt(screen, r.x+2, r.y+2, truncate("Edit "+p.names[p.selected], w-4), s.colors.text)
		rows := min(len(themeRoles), h-5)
		for i := 0; i < rows; i++ {
			value := *paletteRole(&p.pending, i)
			color := s.colors.muted
			if i == p.role {
				value, color = "["+p.colorText+"]", s.colors.focus
			}
			printAt(screen, r.x+2, r.y+3+i, truncate(fmt.Sprintf("%-8s %s", themeRoles[i], value), w-4), color)
		}
		footer = themeHelp(w-4, "<enter> validate  s save", "s save")
	case themeDeleteConfirm:
		printAt(screen, r.x+2, r.y+3, truncate("Delete "+p.names[p.selected]+"? (y/n)", w-4), s.colors.warning)
	default:
		if p.filtering {
			printAt(screen, r.x+2, r.y+2, truncate("Filter: "+p.query+"_", w-4), s.colors.focus)
		} else {
			footer = themePickerHelp(w - 4)
		}
		matches := p.matchingThemes()
		listY, limit := r.y+2, h-5
		if p.filtering {
			listY, limit = r.y+4, h-7
		}
		rows := min(len(matches), limit)
		position := 0
		for i, index := range matches {
			if index == p.selected {
				position = i
				break
			}
		}
		start := max(0, position-rows+1)
		for i := 0; i < rows; i++ {
			index := matches[start+i]
			prefix, color := "  ", s.colors.text
			if index == p.selected {
				prefix, color = "> ", s.colors.focus
			}
			kind := "custom"
			if theme.IsBuiltin(p.names[index]) {
				kind = "built-in"
			}
			printAt(screen, r.x+2, listY+i, truncate(prefix+p.names[index]+" ("+kind+")", w-4), color)
		}
		if p.filtering {
			if len(matches) == 0 {
				printAt(screen, r.x+2, listY, "No matching themes", s.colors.muted)
			}
		}
	}
	if p.error != "" {
		printAt(screen, r.x+2, r.y+h-2, truncate(strings.ReplaceAll(p.error, "\n", " "), w-4), s.colors.warning)
	} else if footer != "" {
		printAt(screen, r.x+2, r.y+h-2, truncate(footer, w-4), s.colors.muted)
	}
}

func themePickerHelp(width int) string {
	return themeHelp(width,
		"n new  e edit  d delete  / filter",
		"n/e/d actions  / filter",
		"/ filter")
}

func themeHelp(width int, hints ...string) string {
	for _, hint := range hints {
		if utf8.RuneCountInString(hint) <= width {
			return hint
		}
	}
	return hints[len(hints)-1]
}
