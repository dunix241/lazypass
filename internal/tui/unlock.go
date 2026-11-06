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
	value []byte
	err   error
}
type unlockPrompt struct {
	ctx   context.Context
	value []byte
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
		defer clear(reply.value)
		if err := ctx.Err(); err != nil {
			return "", err
		}
		return string(reply.value), reply.err
	case <-ctx.Done():
		request.discardReply()
		cleanupCtx := s.operationContext
		if cleanupCtx == nil {
			cleanupCtx = context.Background()
		}
		s.dispatch(cleanupCtx, func() {
			if s.unlock == request {
				s.finishUnlock(unlockReply{err: ctx.Err()})
			}
			request.discardReply()
		})
		return "", ctx.Err()
	}
}

func (s *screen) finishUnlock(reply unlockReply) {
	p := s.unlock
	if p == nil {
		clear(reply.value)
		return
	}
	s.unlock = nil
	if err := p.contextError(); err != nil {
		clear(reply.value)
		reply = unlockReply{err: err}
	}
	if reply.value == nil {
		clear(p.value)
	}
	p.value = nil
	select {
	case p.reply <- reply:
	default:
		clear(reply.value)
	}
}

func (p *unlockPrompt) append(text string) {
	for _, r := range text {
		if unicode.IsControl(r) {
			continue
		}
		if len(p.value)+utf8.RuneLen(r) > maxPassphraseBytes {
			break
		}
		if p.value == nil {
			p.value = make([]byte, 0, maxPassphraseBytes)
		}
		p.value = utf8.AppendRune(p.value, r)
	}
}

func (s *screen) handleUnlock(event *tcell.EventKey) {
	p := s.unlock
	if p == nil {
		return
	}
	if err := p.contextError(); err != nil {
		s.finishUnlock(unlockReply{err: err})
		return
	}
	switch event.Key() {
	case tcell.KeyEscape:
		s.finishUnlock(unlockReply{err: vault.ErrLocked})
	case tcell.KeyEnter:
		s.finishUnlock(unlockReply{value: p.value})
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		_, size := utf8.DecodeLastRune(p.value)
		p.truncate(len(p.value) - size)
	case tcell.KeyCtrlU:
		p.truncate(0)
	case tcell.KeyCtrlW:
		p.deleteWord()
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
	shared.PrintAt(screen, r.X+2, r.Y+3, strings.Repeat("*", min(utf8.RuneCount(s.unlock.value), max(0, r.W-5)))+"_", colors.Focus)
}

func (p *unlockPrompt) truncate(end int) {
	clear(p.value[end:])
	p.value = p.value[:end]
}

func (p *unlockPrompt) contextError() error {
	if p.ctx == nil {
		return nil
	}
	return p.ctx.Err()
}

func (p *unlockPrompt) discardReply() {
	select {
	case reply := <-p.reply:
		clear(reply.value)
	default:
	}
}

func (p *unlockPrompt) deleteWord() {
	end := len(p.value)
	word := false
	for end > 0 {
		r, size := utf8.DecodeLastRune(p.value[:end])
		space := unicode.IsSpace(r)
		if word && space || !space && r == '/' {
			break
		}
		word = word || !space
		end -= size
	}
	p.truncate(end)
}
