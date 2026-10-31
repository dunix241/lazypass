package themepanel

import "lazypass/internal/tui/shared"

func themePickerHelp(width int) string {
	return themeHelp(width, "n new  e edit  d delete  / filter", "n/e/d actions  / filter", "/ filter")
}
func themeHelp(width int, hints ...string) string { return shared.FitHints(width, hints...) }
