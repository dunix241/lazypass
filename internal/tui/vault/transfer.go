package vault

import (
	"context"
	"errors"
	"fmt"
	"github.com/gdamore/tcell/v2"
	"lazypass/internal/tui/input"
	"lazypass/internal/tui/shared"
	"lazypass/internal/vault"
	"slices"
	"strings"
)

type transferState struct {
	nodes       []vault.Node
	move        bool
	busy        bool
	conflict    bool
	rename      bool
	editor      input.Editor
	destination vault.Path
	index       int
	names       []string
	skipped     map[int]bool
	cancel      context.CancelFunc
	blocked     bool
}

func (s *View) HasStagedTransfers() bool { return len(s.transfer.nodes) > 0 }

func (s *View) ClearStagedTransfers() { s.transfer.nodes = nil }

func (s *View) CancelTransfer() {
	s.renameTarget = nil
	if s.transfer.cancel != nil {
		s.transfer.cancel()
	}
	s.transfer.conflict = false
}

func (s *View) PasteTransferName(text string) {
	if (s.transfer.conflict && s.transfer.rename) || s.renameTarget != nil {
		_ = s.transfer.editor.Insert(text, input.Options{MaxBytes: 255})
	}
}

func (s *View) stageTransfer(move bool) {
	if s.deleting || s.transfer.busy {
		return
	}
	if _, ok := s.vault.Provider.(vault.EntryTransfer); !ok {
		s.notify(shared.Warning, "Entry transfers unavailable")
		return
	}
	var nodes []vault.Node
	seen := map[string]bool{}
	for _, node := range s.vaultDisplayNodes() {
		id := markID(node)
		if s.markable(node) && s.marks[id] && !seen[id] {
			node.Path = slices.Clone(node.Path)
			nodes = append(nodes, node)
			seen[id] = true
		}
	}
	if len(nodes) == 0 {
		if node, ok := s.selectedVaultNode(); ok && s.markable(node) {
			node.Path = slices.Clone(node.Path)
			nodes = append(nodes, node)
		}
	}
	if len(nodes) == 0 {
		return
	}
	if s.transfer.move == move && !s.transfer.blocked {
		remaining := slices.Clone(s.transfer.nodes)
		for _, node := range nodes {
			id := markID(node)
			index := slices.IndexFunc(remaining, func(staged vault.Node) bool { return markID(staged) == id })
			if index >= 0 {
				remaining = slices.Delete(remaining, index, index+1)
			} else {
				remaining = append(remaining, node)
			}
		}
		nodes = remaining
	}
	// A staged folder owns its descendants, even when they were staged earlier
	// from search results or another location.
	staged := nodes
	nodes = slices.DeleteFunc(slices.Clone(nodes), func(node vault.Node) bool {
		for _, parent := range staged {
			if parent.Kind == vault.FolderNode && strings.HasPrefix(node.Path.String(), parent.Path.String()+"/") {
				return true
			}
		}
		return false
	})
	s.transfer = transferState{nodes: nodes, move: move}
}

func (s *View) pasteEntries() {
	t := &s.transfer
	if len(t.nodes) == 0 || t.busy || s.deleting {
		return
	}
	if t.blocked {
		s.notify(shared.Warning, "Previous paste has an uncertain result; refresh and stage entries again before retrying")
		return
	}
	if s.vaultFiltering {
		s.notify(shared.Warning, "Leave search before pasting into a folder")
		return
	}
	t.destination = slices.Clone(s.vaultPath)
	t.names = make([]string, len(t.nodes))
	for i, node := range t.nodes {
		t.names[i] = node.Name
	}
	t.index = 0
	t.skipped = map[int]bool{}
	s.checkPaste()
}

func (s *View) checkPaste() {
	t := &s.transfer
	t.busy = true
	request, cancel := context.WithCancel(context.Background())
	t.cancel = cancel
	// Complete preflight for the entire batch before publishing any entry.
	s.deps.RunOperation.Execute(s.deps.RunPrompted, func(ctx context.Context) error {
		ctx, stop := operationContext(ctx, request)
		defer stop()
		for ; t.index < len(t.nodes); t.index++ {
			if err := request.Err(); err != nil {
				return err
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if t.skipped[t.index] {
				continue
			}
			for j := 0; j < t.index; j++ {
				if !t.skipped[j] && t.names[j] == t.names[t.index] {
					return vault.ErrConflict
				}
			}
			if err := s.transferNode(ctx, t.nodes[t.index], t.destination, t.names[t.index], t.move, true); err != nil {
				return err
			}
		}
		return nil
	}, func(err error) {
		canceled := request.Err() != nil
		cancel()
		t.cancel = nil
		t.busy = false
		if canceled {
			return
		}
		if errors.Is(err, vault.ErrConflict) {
			t.conflict = true
			t.rename = false
			t.editor.Sync(t.names[t.index])
			return
		}
		if err != nil {
			s.notify(shared.Error, storeErrorMessage(err))
			return
		}
		s.executePaste()
	})
}

func (s *View) executePaste() {
	t := &s.transfer
	t.busy = true
	request, cancel := context.WithCancel(context.Background())
	t.cancel = cancel
	completed := map[int]bool{}
	var warning error
	index := 0
	s.progress("Pasting entries…")
	s.deps.RunOperation.Execute(s.deps.RunPrompted, func(ctx context.Context) error {
		ctx, stop := operationContext(ctx, request)
		defer stop()
		for ; index < len(t.nodes); index++ {
			if err := request.Err(); err != nil {
				return err
			}
			if t.skipped[index] {
				continue
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			err := s.transferNode(ctx, t.nodes[index], t.destination, t.names[index], t.move, false)
			var saved vault.SavedWarning
			if errors.As(err, &saved) {
				completed[index] = true
				warning = err
				continue
			}
			if err != nil {
				return err
			}
			completed[index] = true
		}
		return nil
	}, func(err error) {
		canceled := request.Err() != nil
		cancel()
		t.cancel = nil
		t.busy = false
		if t.move {
			var remaining []vault.Node
			for i, node := range t.nodes {
				if !completed[i] {
					remaining = append(remaining, node)
				} else {
					delete(s.marks, markID(node))
				}
			}
			t.nodes = remaining
		}
		if canceled {
			t.blocked = true
			return
		}
		if err != nil {
			// A remote mutation may have succeeded before its response was lost.
			// Keep unfinished references, but require explicit restaging after review.
			t.blocked = !errors.Is(err, vault.ErrPromptRequired) && !errors.Is(err, vault.ErrConflict)
			s.Refresh()
			s.notify(shared.Error, pasteSummary(len(completed))+"; "+storeErrorMessage(err))
			return
		}
		s.Reconcile(pasteSummary(len(completed)))
		if warning != nil {
			s.notify(shared.Warning, storeErrorMessage(warning))
		}
	})
}

func pasteSummary(count int) string {
	if count == 1 {
		return "Pasted 1 item"
	}
	return fmt.Sprintf("Pasted %d items", count)
}

func (s *View) handleTransferKey(event *tcell.EventKey) {
	if s.renameTarget != nil {
		s.handleRenameKey(event)
		return
	}
	t := &s.transfer
	if event.Key() == tcell.KeyEscape {
		if t.cancel != nil {
			t.cancel()
		}
		t.conflict = false
		return
	}
	if !t.conflict {
		return
	}
	if t.rename {
		if event.Key() == tcell.KeyEnter {
			name := t.editor.Value()
			if name == "" || (vault.Path{name}).Validate() != nil {
				return
			}
			t.names[t.index] = name
			t.conflict = false
			s.checkPaste()
			return
		}
		_, _ = t.editor.HandleKey(event, input.Options{MaxBytes: 255})
		return
	}
	if event.Key() == tcell.KeyRune {
		switch event.Rune() {
		case 'r':
			t.rename = true
		case 's':
			t.skipped[t.index] = true
			t.index++
			t.conflict = false
			s.checkPaste()
		}
	}
}

func (s *View) drawTransfer(screen tcell.Screen, frame shared.Frame) {
	if s.renameTarget != nil {
		s.drawRename(screen, frame)
		return
	}
	t := &s.transfer
	if !t.conflict {
		return
	}
	screen.HideCursor()
	r := shared.CenteredPanel(frame.Viewport, min(64, max(0, frame.Viewport.W-2)), 8)
	shared.Panel{Bounds: r, Title: "Paste conflict", Border: frame.Palette.Focus, Background: frame.Palette.Surface}.Draw(screen, func(content shared.Rect) {
		shared.PrintAt(screen, content.X, content.Y, shared.Truncate(t.names[t.index]+" already exists at destination", content.W), frame.Palette.Text)
		if t.rename {
			t.editor.Draw(screen, content.X, content.Y+2, content.W, 1, tcell.StyleDefault.Foreground(frame.Palette.Text).Background(frame.Palette.Surface), true, input.Options{MaxBytes: 255})
			shared.PrintAt(screen, content.X, content.Y+4, "<CR> continue  •  <esc> cancel", frame.Palette.Muted)
		} else {
			shared.PrintAt(screen, content.X, content.Y+3, "r rename  •  s skip  •  <esc> cancel", frame.Palette.Muted)
		}
	})
}

func (s *View) stagedCut(node vault.Node) bool {
	return s.transfer.move && s.stagedEntry(node)
}

func (s *View) stagedEntry(node vault.Node) bool {
	for _, staged := range s.transfer.nodes {
		if markID(staged) == markID(node) {
			return true
		}
	}
	return false
}
