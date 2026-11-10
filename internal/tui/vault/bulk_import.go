package vault

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"

	"lazypass/internal/clipboard"
	"lazypass/internal/otp"
	"lazypass/internal/tui/shared"
	"lazypass/internal/vault"
	"lazypass/internal/vault/service"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/uniseg"
)

type bulkRow struct {
	attemptedPath vault.Path
	created       bool
	uncertain     bool
	record        otp.MigrationRecord
	name          string
	selected      bool
	status        string
}

func (row bulkRow) importable() bool {
	return row.selected && !row.created && !row.uncertain && row.record.Unsupported == ""
}

func (row bulkRow) destinationPath(parent vault.Path) (vault.Path, error) {
	path := append(append(vault.Path(nil), parent...), row.name)
	if row.name == "" || strings.ContainsAny(row.name, "/\\") || path.Validate() != nil {
		return nil, vault.ErrInvalidPath
	}
	return path, nil
}

type bulkImport struct {
	destination vault.Path
	folders     []vault.Node
	folderIndex int
	message     string
	completed   atomic.Int32
	total       int
	cancel      context.CancelFunc
	batch       otp.MigrationBatch
	rows        []bulkRow
	phase       string
	text        string
	index       int
	changed     bool
}

func (s *StoreForm) handleBulkKey(event *tcell.EventKey) {
	b := s.bulk
	if s.saving {
		if event.Key() == tcell.KeyEscape && b.cancel != nil {
			b.cancel()
		}
		return
	}
	if event.Key() == tcell.KeyEscape {
		if b.phase == "destination" {
			b.phase = "preview"
			return
		}
		if b.phase == "rename" {
			b.phase = "preview"
			b.text = ""
			return
		}
		if b.changed {
			s.saved(s.createParent, "Import finished")
		} else {
			s.Close()
		}
		return
	}
	switch b.phase {
	case "images":
		switch event.Key() {
		case tcell.KeyEnter:
			s.loadBulkImage(false)
		case tcell.KeyCtrlV:
			s.loadBulkImage(true)
		default:
			s.editInput(event)
		}
	case "preview":
		switch event.Key() {
		case tcell.KeyUp:
			b.index = max(0, b.index-1)
		case tcell.KeyDown:
			b.index = min(len(b.rows)-1, b.index+1)
		case tcell.KeyEnter:
			b.phase = "destination"
			if b.destination == nil {
				b.destination = append(vault.Path(nil), s.createParent...)
			}
			s.loadBulkDestination(b.destination)
		case tcell.KeyRune:
			row := &b.rows[b.index]
			switch event.Rune() {
			case 'j':
				b.index = min(len(b.rows)-1, b.index+1)
			case 'k':
				b.index = max(0, b.index-1)
			case ' ':
				if row.record.Unsupported == "" && !row.created && !row.uncertain {
					row.selected = !row.selected
				}
			case 'r':
				if row.created || row.uncertain {
					return
				}
				b.phase = "rename"
				b.text = row.name
			}
		}
	case "rename":
		if event.Key() == tcell.KeyEnter {
			b.rows[b.index].name = b.text
			b.phase = "preview"
		} else {
			s.editInput(event)
		}
	case "destination":
		s.handleBulkDestination(event)
	case "results":
		if event.Key() == tcell.KeyRune {
			if event.Rune() == 'c' {
				s.reconcileBulk()
				return
			}
			if event.Rune() == 'r' {
				b.phase = "preview"
				return
			}
			if event.Rune() == 'j' {
				b.index = min(len(b.rows)-1, b.index+1)
			}
			if event.Rune() == 'k' {
				b.index = max(0, b.index-1)
			}
		}
		if event.Key() == tcell.KeyUp {
			b.index = max(0, b.index-1)
		}
		if event.Key() == tcell.KeyDown {
			b.index = min(len(b.rows)-1, b.index+1)
		}
		if event.Key() == tcell.KeyEnter {
			if b.changed {
				s.saved(s.createParent, "Authenticators imported")
			} else {
				s.Close()
			}
		}
	}
}

func (s *StoreForm) loadBulkImage(fromClipboard bool) {
	b := s.bulk
	path, err := qrImagePath(b.text)
	if err != nil && !fromClipboard {
		s.notify(shared.Error, err.Error())
		return
	}
	var pages []otp.MigrationPage
	s.run(func(ctx context.Context) error {
		var values []string
		var err error
		if fromClipboard {
			data, readErr := clipboard.ReadImage(ctx)
			defer clear(data)
			if readErr != nil {
				return readErr
			}
			values, err = otp.DecodeQRValues(ctx, data)
		} else {
			values, err = otp.ReadQRImage(ctx, path)
		}
		if err != nil {
			return err
		}
		for _, value := range values {
			page, err := otp.ParseMigration(value)
			if err != nil {
				return err
			}
			pages = append(pages, page)
		}
		return nil
	}, func(err error) {
		if err != nil {
			message := qrImportError(err)
			if errors.Is(err, otp.ErrMigration) {
				message = otp.ErrMigration.Error()
			}
			s.notify(shared.Error, message)
			return
		}
		before, _ := b.batch.Progress()
		if err := b.batch.AddPages(pages); err != nil {
			s.notify(shared.Error, err.Error())
			return
		}
		b.text = ""
		loaded, _ := b.batch.Progress()
		b.message = "Page loaded. Add the next export image."
		if loaded == before {
			b.message = "This page is already loaded. Add a missing page."
		}
		_, total := b.batch.Progress()
		if loaded != total {
			return
		}
		records, err := b.batch.Records()
		if err != nil {
			s.notify(shared.Error, err.Error())
			return
		}
		seen := map[string]bool{}
		for _, record := range records {
			name := record.Name
			if record.Issuer != "" {
				name = record.Issuer + " - " + name
			}
			name = strings.Map(func(r rune) rune {
				if r == '/' || r == '\\' {
					return '-'
				}
				return r
			}, name)
			row := bulkRow{record: record, name: name, selected: record.Unsupported == ""}
			if record.URI != "" && seen[record.URI] {
				row.selected = false
				row.status = "Duplicate in export"
			}
			seen[record.URI] = true
			b.rows = append(b.rows, row)
		}
		b.phase = "preview"
	})
}

func (s *StoreForm) executeBulk() {
	b := s.bulk
	parent, err := vault.ParsePath(b.text)
	if err != nil {
		s.notify(shared.Warning, "Choose a valid destination folder")
		return
	}
	entries := make([]vault.Entry, 0, len(b.rows))
	indexes := make([]int, 0, len(b.rows))
	for i, row := range b.rows {
		if !row.importable() {
			continue
		}
		path, err := row.destinationPath(parent)
		if err != nil {
			s.notify(shared.Warning, "Rename invalid entry names before importing")
			return
		}
		entries = append(entries, vault.Entry{Path: path, Authenticator: row.record.URI})
		b.rows[i].attemptedPath = append(vault.Path(nil), path...)
		indexes = append(indexes, i)
	}
	if len(entries) == 0 {
		s.notify(shared.Warning, "Select at least one supported account")
		return
	}
	provider := s.deps.Service
	var results []service.ImportResult
	b.total = len(entries)
	b.completed.Store(0)
	request, cancel := context.WithCancel(context.Background())
	b.cancel = cancel
	s.run(func(ctx context.Context) error {
		ctx, stop := context.WithCancel(ctx)
		defer stop()
		unregister := context.AfterFunc(request, stop)
		defer unregister()
		if request.Err() != nil {
			stop()
		}
		results = provider.ImportEntries(ctx, entries, func(completed int) { b.completed.Store(int32(completed)) })
		return nil
	}, func(error) {
		cancel()
		b.cancel = nil
		for i, index := range indexes {
			b.rows[index].status = results[i].Status
			b.rows[index].created = results[i].Created
			b.rows[index].uncertain = results[i].Uncertain
			if results[i].Created || results[i].Uncertain {
				b.rows[index].selected = false
			}
			b.changed = b.changed || results[i].Created
		}
		b.phase = "results"
	})
}

func (s *StoreForm) drawBulk(screen tcell.Screen, frame shared.Frame) {
	b := s.bulk
	r := shared.CenteredPanel(frame.Viewport, min(76, frame.Viewport.W-4), min(20, frame.Viewport.H-4))
	shared.Panel{Bounds: r, Title: "Import authenticators", TitleColor: frame.Palette.Accent, Border: frame.Palette.Focus, Background: frame.Palette.Surface}.Draw(screen, nil)
	step, title := bulkStep(b.phase)
	y := r.Y + 2
	y += drawBulkSteps(screen, r, frame.Palette, step, title)
	y++
	if s.saving {
		message := "Reading export image…"
		if b.phase == "destination" {
			message = fmt.Sprintf("Importing %d of %d", min(int(b.completed.Load())+1, b.total), b.total)
		}
		shared.PrintAt(screen, r.X+2, y, shared.Truncate(message, r.W-4), frame.Palette.Muted)
		shared.PrintPanelFooter(screen, r, "<esc> cancel", frame.Palette.Muted)
		return
	}
	if b.phase == "destination" {
		label := "/" + b.destination.String()
		if s.deps.LocationLabel != nil {
			label = s.deps.LocationLabel(b.destination)
		}
		shared.PrintAt(screen, r.X+2, y, shared.Truncate(label, r.W-4), frame.Palette.Muted)
		y += 2
		limit := max(1, r.Y+r.H-2-y)
		start := max(0, b.folderIndex-limit+1)
		for i := start; i <= len(b.folders) && i < start+limit; i++ {
			label := "Use this location"
			if i > 0 {
				label = b.folders[i-1].Name + "/"
			}
			color := frame.Palette.Text
			if i == b.folderIndex {
				color = frame.Palette.Focus
			}
			shared.PrintAt(screen, r.X+2, y+i-start, shared.Truncate(label, r.W-4), color)
		}
		shared.PrintPanelFooter(screen, r, "<CR> next  •  <esc> back", frame.Palette.Muted)
		return
	}
	switch b.phase {
	case "images", "rename":
		label := "Google Authenticator export PNG/JPEG:"
		hint := "<ctrl-v> clipboard  •  <CR> next  •  <esc> cancel"
		if b.phase == "rename" {
			label = "Entry name:"
			hint = "<CR> next  •  <esc> back"
		}
		if b.phase == "images" {
			loaded, total := b.batch.Progress()
			if total > 0 {
				shared.PrintAt(screen, r.X+2, y, fmt.Sprintf("%d of %d export pages loaded", loaded, total), frame.Palette.Text)
				y++
				available := max(0, r.Y+r.H-7-y)
				if total <= available {
					for page := 0; page < total; page++ {
						text := fmt.Sprintf("○ Export page %d needed", page+1)
						if b.batch.HasPage(page) {
							text = fmt.Sprintf("✓ Export page %d loaded", page+1)
						}
						shared.PrintAt(screen, r.X+2, y, shared.Truncate(text, r.W-4), frame.Palette.Muted)
						y++
					}
				} else {
					missing := b.batch.Missing()
					shared.PrintAt(screen, r.X+2, y, fmt.Sprintf("Next missing: export page %d", missing[0]+1), frame.Palette.Muted)
					y++
				}
				for _, line := range shared.WrapText(b.message, r.W-4) {
					if y >= r.Y+r.H-5 {
						break
					}
					shared.PrintAt(screen, r.X+2, y, line, frame.Palette.Muted)
					y++
				}
			}
		}
		for _, line := range shared.WrapText(label, r.W-4) {
			if y >= r.Y+r.H-3 {
				break
			}
			shared.PrintAt(screen, r.X+2, y, line, frame.Palette.Text)
			y++
		}
		_, editor, options := s.activeInput()
		editor.Draw(screen, r.X+2, min(y+1, r.Y+r.H-3), r.W-4, 1, tcell.StyleDefault.Foreground(frame.Palette.Text).Background(frame.Palette.Surface), true, options)
		shared.PrintPanelFooter(screen, r, shared.FitHints(r.W-4, hint, "<CR> next  •  <esc> cancel"), frame.Palette.Muted)
	default:
		limit := max(1, r.Y+r.H-2-y)
		start := max(0, b.index-limit+1)
		for i := start; i < len(b.rows) && i < start+limit; i++ {
			row := b.rows[i]
			prefix := "[ ] "
			if row.selected {
				prefix = "[x] "
			}
			text := prefix + row.name
			if row.record.Unsupported != "" {
				text += " · " + row.record.Unsupported
			} else if row.status != "" {
				text += " · " + row.status
			}
			color := frame.Palette.Text
			if i == b.index {
				color = frame.Palette.Focus
			}
			shared.PrintAt(screen, r.X+2, y+i-start, shared.Truncate(text, r.W-4), color)
		}
		hint := "<space> select  •  r rename  •  <CR> next  •  <esc> cancel"
		if b.phase == "results" {
			hint = "r review  •  c reconcile  •  <CR> finish  •  <esc> cancel"
		}
		fallback := "<CR> next  •  <esc> cancel"
		if b.phase == "results" {
			fallback = "<CR> finish  •  <esc> cancel"
		}
		shared.PrintPanelFooter(screen, r, shared.FitHints(r.W-4, hint, fallback), frame.Palette.Muted)
	}
}

func drawBulkSteps(screen tcell.Screen, panel shared.Rect, colors shared.Palette, step int, title string) int {
	labels := []string{"Collect export", "Review accounts", "Destination", "Results"}
	active := tcell.StyleDefault.Foreground(colors.Surface).Background(colors.Focus)
	if panel.W < 72 {
		lines := shared.WrapText(fmt.Sprintf("Step %d of 4 · %s", step, title), panel.W-4)
		for row, line := range lines {
			shared.PrintStyled(screen, panel.X+2, panel.Y+2+row, line, active)
		}
		return len(lines)
	}
	x := panel.X + 2
	for i, label := range labels {
		style := tcell.StyleDefault.Foreground(colors.Muted).Background(colors.Surface)
		if i+1 < step {
			label = "✓ " + label
		}
		if i+1 == step {
			label = " " + label + " "
			style = active
		}
		shared.PrintStyled(screen, x, panel.Y+2, label, style)
		x += uniseg.StringWidth(label)
		if i < len(labels)-1 {
			shared.PrintAt(screen, x, panel.Y+2, " → ", colors.Muted)
			x += 3
		}
	}
	return 1
}

func bulkStep(phase string) (int, string) {
	switch phase {
	case "preview", "rename":
		return 2, "Review accounts"
	case "destination":
		return 3, "Destination"
	case "results":
		return 4, "Results"
	default:
		return 1, "Collect export"
	}
}
