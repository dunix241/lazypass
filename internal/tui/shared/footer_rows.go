package shared

import (
	"strings"

	"github.com/rivo/uniseg"
)

const (
	Optional = iota
	Secondary
	Primary
	Essential
)

type FooterOptions struct{ MaxRows int }

type footerLabel struct {
	action   Action
	text     string
	width    int
	priority int
}

type footerRow struct {
	text   string
	width  int
	labels []footerLabel
}

func (f Footer) Rows(width int, options FooterOptions) []string {
	if len(f.Actions) == 0 {
		if width <= 0 || options.MaxRows <= 0 {
			return nil
		}
		rows := WrapText(f.Text, width)
		return rows[:min(len(rows), options.MaxRows)]
	}
	layout := f.layoutRows(width, options)
	rows := make([]string, len(layout))
	for i, row := range layout {
		rows[i] = row.text
	}
	return rows
}

func (f Footer) layoutRows(width int, options FooterOptions) []footerRow {
	if width <= 0 || options.MaxRows <= 0 {
		return nil
	}
	actions := make([]footerLabel, 0, len(f.Actions))
	for _, action := range f.Actions {
		if action.Enabled != nil && !action.Enabled() {
			continue
		}
		label := strings.TrimSpace(action.Hint + " " + action.Label)
		labelWidth := uniseg.StringWidth(label)
		if label != "" && labelWidth <= width {
			actions = append(actions, footerLabel{action: action, text: label, width: labelWidth, priority: action.Priority})
		}
	}
	for len(actions) > 0 {
		rows := packFooter(actions, width)
		if len(rows) <= options.MaxRows {
			return rows
		}
		remove := len(actions) - 1
		for i := len(actions) - 1; i >= 0; i-- {
			if actions[i].priority < actions[remove].priority {
				remove = i
			}
		}
		actions = append(actions[:remove], actions[remove+1:]...)
	}
	return nil
}

// HitAction uses the same packed rows and centered placement as drawing.
func (f Footer) HitAction(bounds Rect, options FooterOptions, x, y int) (Action, bool) {
	if !bounds.Contains(x, y) {
		return Action{}, false
	}
	rows := f.layoutRows(bounds.W, options)
	row := y - bounds.Y
	if row < 0 || row >= len(rows) {
		return Action{}, false
	}
	left := bounds.X + max(0, (bounds.W-rows[row].width)/2)
	offset := 0
	for _, label := range rows[row].labels {
		if x >= left+offset && x < left+offset+label.width {
			return label.action, true
		}
		offset += label.width + 5
	}
	return Action{}, false
}

func packFooter(actions []footerLabel, width int) []footerRow {
	var rows []footerRow
	start := 0
	var line strings.Builder
	used := 0
	for i, action := range actions {
		separator := 0
		if used > 0 {
			separator = 5
		}
		if used+separator+action.width > width {
			rows = append(rows, footerRow{text: line.String(), width: used, labels: actions[start:i]})
			start = i
			line.Reset()
			used, separator = 0, 0
		}
		if separator > 0 {
			line.WriteString("  •  ")
		}
		line.WriteString(action.text)
		used += separator + action.width
	}
	if used > 0 {
		rows = append(rows, footerRow{text: line.String(), width: used, labels: actions[start:]})
	}
	return rows
}
