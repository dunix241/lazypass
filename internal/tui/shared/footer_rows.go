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
	text     string
	width    int
	priority int
}

func (f Footer) Rows(width int, options FooterOptions) []string {
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
			actions = append(actions, footerLabel{text: label, width: labelWidth, priority: action.Priority})
		}
	}
	if len(f.Actions) == 0 {
		rows := WrapText(f.Text, width)
		return rows[:min(len(rows), options.MaxRows)]
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

func packFooter(actions []footerLabel, width int) []string {
	var rows []string
	var line strings.Builder
	used := 0
	for _, action := range actions {
		separator := 0
		if used > 0 {
			separator = 5
		}
		if used+separator+action.width > width {
			rows = append(rows, line.String())
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
		rows = append(rows, line.String())
	}
	return rows
}
