package shared

import "unicode/utf8"

func FitFooter(width int, options ...string) string {
	return FitHints(width-2, options...)
}

func FitHints(width int, options ...string) string {
	for _, option := range options {
		if utf8.RuneCountInString(option) <= width {
			return option
		}
	}
	if len(options) == 0 {
		return ""
	}
	return options[len(options)-1]
}
