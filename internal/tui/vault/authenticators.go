package vault

import (
	"context"
	"errors"
	"time"

	"lazypass/internal/tui/shared"
	"lazypass/internal/vault"
	"lazypass/internal/vault/service"
)

const authenticatorCapacity = 256

type authenticatorRow struct {
	value  service.Authenticator
	retry  time.Time
	loaded bool
}

type authenticatorState struct {
	rows           map[string]authenticatorRow
	cancel         context.CancelFunc
	generation     uint64
	promptDeclined bool
	card           shared.Rect
}

// ClearAuthenticators also rejects results already queued for UI dispatch.
func (s *View) ClearAuthenticators() {
	if s.authenticators.cancel != nil {
		s.authenticators.cancel()
	}
	s.authenticators = authenticatorState{
		generation: s.authenticators.generation + 1,
		card:       s.authenticators.card,
	}
}

// TickAuthenticators runs on UI dispatch; provider work runs in a single worker.
func (s *View) TickAuthenticators(ctx context.Context) {
	state := &s.authenticators
	if state.cancel != nil || s.awaitingUnlock || s.loading || s.syncing || s.deleting || s.deps.Dispatch == nil || ctx.Err() != nil {
		return
	}
	if state.rows == nil {
		state.rows = make(map[string]authenticatorRow)
	}
	nodes := s.vaultDisplayNodes()
	_, limit, start := s.vaultListWindow(state.card)
	if limit <= 0 || len(nodes) == 0 {
		return
	}
	start = min(start, len(nodes))
	// Visible entries lead the queue; continue through the current result set.
	end := min(len(nodes), start+limit)
	now := time.Now()
	for _, candidates := range [][]vault.Node{nodes[start:end], nodes[:start], nodes[end:]} {
		for _, node := range candidates {
			if node.Kind != vault.EntryNode || (node.TypeKnown && node.EntryType != vault.LoginEntry) {
				continue
			}
			id := node.Path.String()
			row, found := state.rows[id]
			code, _ := row.value.Current(now)
			if found && (now.Before(row.retry) || code != "") {
				continue
			}
			if !found && len(state.rows) >= authenticatorCapacity {
				if !authenticatorVisible(id, nodes[start:end]) {
					continue
				}
				for cached := range state.rows {
					if !authenticatorVisible(cached, nodes[start:end]) {
						delete(state.rows, cached)
						break
					}
				}
			}
			work, cancel := context.WithCancel(vault.WithoutPrompt(ctx))
			state.cancel = cancel
			generation := state.generation
			provider := s.vault
			path := append(vault.Path(nil), node.Path...)
			go func() {
				load := func(request context.Context) error {
					if row.loaded {
						return provider.RefreshAuthenticator(request, path, &row.value)
					}
					var err error
					row.value, err = provider.LoadAuthenticator(request, path)
					return err
				}
				complete := func(err error) {
					defer cancel()
					if generation != s.authenticators.generation || work.Err() != nil {
						return
					}
					s.authenticators.cancel = nil
					if err == nil {
						row.loaded = true
						row.retry = time.Time{}
					} else {
						row.value = service.Authenticator{}
						row.loaded = false
						row.retry = time.Now().Add(authenticatorRetryDelay(err))
					}
					s.authenticators.rows[id] = row
					// Drain the queue without waiting a second between entries.
					s.TickAuthenticators(ctx)
				}
				err := load(work)
				s.deps.Dispatch(ctx, func() {
					if generation != s.authenticators.generation || work.Err() != nil {
						cancel()
						return
					}
					if errors.Is(err, vault.ErrPromptRequired) && !s.authenticators.promptDeclined && s.deps.RunOperation != nil {
						s.deps.RunOperation.Execute(s.deps.RunPrompted, func(request context.Context) error {
							request, stop := context.WithCancel(request)
							defer stop()
							unregister := context.AfterFunc(work, stop)
							defer unregister()
							if work.Err() != nil {
								return context.Canceled
							}
							return load(request)
						}, func(err error) {
							if generation == s.authenticators.generation && err != nil {
								s.authenticators.promptDeclined = true
							}
							complete(err)
						})
						return
					}
					complete(err)
				})
			}()
			return
		}
	}
}

func authenticatorRetryDelay(err error) time.Duration {
	switch {
	case errors.Is(err, vault.ErrPromptRequired), errors.Is(err, vault.ErrLocked):
		return 2 * time.Second
	case errors.Is(err, service.ErrEmptyCopy), errors.Is(err, vault.ErrUnsupported):
		return 24 * time.Hour
	default:
		return time.Minute
	}
}

func authenticatorVisible(path string, nodes []vault.Node) bool {
	for _, node := range nodes {
		if node.Path.String() == path {
			return true
		}
	}
	return false
}

func (s *View) authenticatorChip(node vault.Node, card shared.Rect, now time.Time) (shared.Rect, string, int) {
	row := s.authenticators.rows[node.Path.String()]
	code, remaining := row.value.Current(now)
	if code == "" || card.W < len(code)+14 {
		return shared.Rect{}, "", 0
	}
	if len(code) == 6 {
		code = code[:3] + " " + code[3:]
	} else if len(code) == 8 {
		code = code[:4] + " " + code[4:]
	}
	validityWidth := 7
	if card.W < len(code)+24 {
		validityWidth = 0
	}
	width := len(code) + 2
	return shared.Rect{X: card.X + card.W - 2 - validityWidth - width, W: width, H: 1}, code, remaining
}
