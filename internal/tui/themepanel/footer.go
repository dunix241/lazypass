package themepanel

import "lazypass/internal/tui/shared"

func themePickerHelp(width int) string {
	return themeHelp(width, "a add  e edit  d delete  / filter", "a/e/d actions  / filter", "/ filter")
}
func themeHelp(width int, hints ...string) string { return shared.FitHints(width, hints...) }
