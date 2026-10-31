package shared

import "github.com/rivo/uniseg"

func FitFooter(width int, options ...string) string {
	return FitHints(width-2, options...)
}

func FitHints(width int, options ...string) string {
	for _, option := range options {
		if uniseg.StringWidth(option) <= width {
			return option
		}
	}
	if len(options) == 0 {
		return ""
	}
	return Truncate(options[len(options)-1], width)
}
