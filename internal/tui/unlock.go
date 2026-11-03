package tui

import (
	"context"
	"strings"
	"unicode"
	"unicode/utf8"

	"lazypass/internal/tui/shared"
	"lazypass/internal/vault"

	"github.com/gdamore/tcell/v2"
)

const maxPassphraseBytes = 4096

type unlockReply struct {
	value string
	err   error
}
type unlockPrompt struct {
	ctx   context.Context
	value string
	reply chan unlockReply
}

func (s *screen) requestVaultPassphrase(ctx context.Context, provider vault.Provider) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	request := &unlockPrompt{ctx: ctx, reply: make(chan unlockReply, 1)}
	s.dispatch(ctx, func() {
		if ctx.Err() != nil {
			return
		}
		if provider != nil && !s.isActiveProvider(provider) {
			request.reply <- unlockReply{err: context.Canceled}
			return
		}
		if s.unlock != nil {
			request.reply <- unlockReply{err: vault.ErrLocked}
			return
		}
		s.unlock = request
	})
	select {
	case reply := <-request.reply:
		return reply.value, reply.err
	case <-ctx.Done():
		cleanupCtx := s.operationContext
		if cleanupCtx == nil {
			cleanupCtx = context.Background()
		}
		s.dispatch(cleanupCtx, func() {
			if s.unlock == request {
				s.finishUnlock(unlockReply{err: ctx.Err()})
			}
		})
		return "", ctx.Err()
	}
}

func (s *screen) finishUnlock(reply unlockReply) {
	p := s.unlock
	if p == nil {
		return
	}
	s.unlock = nil
	p.value = ""
	select {
	case p.reply <- reply:
	default:
	}
}

func (p *unlockPrompt) append(text string) {
	for _, r := range text {
		if unicode.IsControl(r) {
			continue
		}
		value := string(r)
		if len(p.value)+len(value) > maxPassphraseBytes {
			break
		}
		p.value += value
	}
}

func (s *screen) handleUnlock(event *tcell.EventKey) {
	p := s.unlock
	if p == nil {
		return
	}
	if p.ctx != nil && p.ctx.Err() != nil {
		s.finishUnlock(unlockReply{err: p.ctx.Err()})
		return
	}
	switch event.Key() {
	case tcell.KeyEscape:
		s.finishUnlock(unlockReply{err: vault.ErrLocked})
	case tcell.KeyEnter:
		s.finishUnlock(unlockReply{value: p.value})
		s.notify(shared.Info, "Unlocking…")
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		p.value = shared.DeleteLastRune(p.value)
	case tcell.KeyCtrlU:
		p.value = ""
	case tcell.KeyCtrlW:
		p.value = shared.DeletePreviousWord(p.value)
	case tcell.KeyRune:
		p.append(string(event.Rune()))
	}
}

func (s *screen) drawUnlock(screen tcell.Screen, frame shared.Frame) {
	v := frame.Viewport
	w := min(52, v.W-2)
	r := shared.Rect{X: v.X + (v.W-w)/2, Y: v.Y + (v.H-7)/2, W: w, H: 7}
	colors := frame.Palette
	shared.DrawBox(screen, r, colors.Focus, colors.Surface)
	shared.PrintAt(screen, r.X+2, r.Y, " Unlock vault ", colors.Accent)
	shared.PrintAt(screen, r.X+2, r.Y+2, "Passphrase:", colors.Text)
	shared.PrintAt(screen, r.X+2, r.Y+3, strings.Repeat("*", min(utf8.RuneCountInString(s.unlock.value), max(0, r.W-5)))+"_", colors.Focus)
}
