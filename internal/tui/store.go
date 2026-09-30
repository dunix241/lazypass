package tui

import (
	"context"
	"errors"
	"strings"

	"lazypass/internal/debug"
	"lazypass/internal/vault"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/uniseg"
)

func (s *screen) drawStoreForm(screen tcell.Screen, x, y, width, height int) {
	r, visible := s.storeFormRect(x, y, width, height)
	drawBox(screen, r, s.colors.focus, s.colors.surface)
	printAt(screen, r.x+2, r.y, " Store generated password ", s.colors.accent)
	printAt(screen, r.x+2, r.y+2, "Destination (folder/name):", s.colors.muted)
	printAt(screen, r.x+2, r.y+3, truncate(s.storePath+"_", r.w-4), s.colors.text)
	if s.storeMessage != "" {
		printAt(screen, r.x+2, r.y+5, truncate(s.storeMessage, r.w-4), s.colors.warning)
	} else if visible > 0 {
		printAt(screen, r.x+2, r.y+5, "Folders (Tab inserts selected):", s.colors.muted)
		for i := 0; i < visible; i++ {
			prefix, color := "  ", s.colors.accent
			if i == s.storeSelected {
				prefix = "> "
			}
			rowY := r.y + 6 + i
			printAt(screen, r.x+2, rowY, truncate(prefix+s.vaultNodeLabel(s.storeNodes[i]), r.w-4), color)
		}
	}
	printAt(screen, r.x+2, r.y+r.h-2, storeFormHelp(r.w-4), s.colors.muted)
}

func storeFormHelp(width int) string {
	for _, hint := range []string{
		"↑/↓ select  •  Tab folder  •  Enter save  •  Esc cancel",
		"↑/↓ select  •  Tab  •  Enter save  •  Esc cancel",
		"Tab pick  •  Enter save  •  Esc cancel",
		"Enter save  •  Esc cancel",
	} {
		if uniseg.StringWidth(hint) <= width {
			return hint
		}
	}
	return "Esc cancel"
}

func (s *screen) storeFormRect(x, y, width, height int) (rect, int) {
	w := min(52, width-4)
	maxHeight := height - 4
	visible := min(4, min(max(0, maxHeight-9), len(s.storeNodes)))
	return rect{x + (width-w)/2, y + max(2, (height-(9+visible))/2), w, 9 + visible}, visible
}

func (s *screen) openStoreForm() {
	if s.vault.Provider == nil {
		s.notify(notificationWarning, "Vault unavailable")
		return
	}
	s.storeForm, s.storePath, s.storeSelected, s.storeMessage = true, "", 0, ""
	s.refreshStoreSuggestions()
}

func (s *screen) handleStoreInput(event *tcell.EventKey) {
	switch event.Key() {
	case tcell.KeyEscape:
		s.storeForm, s.storePath, s.storeNodes, s.storeSelected, s.storeMessage = false, "", nil, 0, ""
	case tcell.KeyCtrlU:
		s.storePath = ""
		s.refreshStoreSuggestions()
	case tcell.KeyCtrlW:
		s.storePath = deletePreviousWord(s.storePath)
		s.refreshStoreSuggestions()
	case tcell.KeyUp:
		s.storeSelected = max(0, s.storeSelected-1)
	case tcell.KeyDown:
		s.storeSelected = min(max(0, len(s.storeNodes)-1), s.storeSelected+1)
	case tcell.KeyTAB:
		s.insertStoreSuggestion()
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if len(s.storePath) > 0 {
			runes := []rune(s.storePath)
			s.storePath = string(runes[:len(runes)-1])
			s.refreshStoreSuggestions()
		}
	case tcell.KeyEnter:
		path, err := vaultPath(s.storePath)
		if err != nil || len(path) == 0 {
			s.notify(notificationWarning, "Enter a valid vault destination")
			return
		}
		if err := s.vault.Store(context.Background(), vault.Entry{Path: path, Password: s.password}); err != nil {
			var diagnostic vault.Diagnostic
			if !errors.As(err, &diagnostic) {
				debug.Failure("vault store", vaultFailureCause(err))
			}
			s.notify(notificationError, storeErrorMessage(err))
			return
		}
		s.storeForm, s.storePath, s.storeNodes, s.storeSelected, s.storeMessage = false, "", nil, 0, ""
		s.notify(notificationSuccess, "Stored in vault")
	case tcell.KeyRune:
		if event.Rune() >= ' ' {
			s.storePath += string(event.Rune())
			s.refreshStoreSuggestions()
		}
	}
}

func (s *screen) refreshStoreSuggestions() {
	s.storeNodes, s.storeSelected, s.storeMessage = nil, 0, ""
	path, prefix, err := storeSuggestionQuery(s.storePath)
	if err != nil {
		return
	}
	nodes, err := s.vault.List(context.Background(), path)
	if err != nil {
		if !errors.Is(err, vault.ErrNotFound) {
			debug.Failure("vault suggestions", vaultFailureCause(err))
			s.storeMessage = "Folders cannot be loaded. Type a destination instead."
		}
		return
	}
	for _, node := range nodes {
		if node.Kind == vault.FolderNode && strings.HasPrefix(strings.ToLower(node.Name), strings.ToLower(prefix)) {
			s.storeNodes = append(s.storeNodes, node)
		}
	}
}

func storeSuggestionQuery(destination string) (vault.Path, string, error) {
	if destination == "" {
		return nil, "", nil
	}
	separator := strings.LastIndex(destination, "/")
	if separator < 0 {
		return nil, destination, nil
	}
	parent, prefix := destination[:separator], destination[separator+1:]
	if parent == "" {
		return nil, prefix, nil
	}
	path, err := vaultPath(parent)
	return path, prefix, err
}

func (s *screen) insertStoreSuggestion() {
	if len(s.storeNodes) == 0 {
		return
	}
	s.storePath = s.storeNodes[s.storeSelected].Path.String() + "/"
	s.refreshStoreSuggestions()
}

func vaultPath(text string) (vault.Path, error) {
	path := vault.Path(strings.Split(text, "/"))
	if err := path.Validate(); err != nil {
		return nil, err
	}
	return path, nil
}
