package shared

import (
	"strings"

	"github.com/rivo/uniseg"
)

// WrapText preserves graphemes and splits long tokens when they exceed a row.
func WrapText(text string, width int) []string {
	if width <= 0 {
		return nil
	}
	var lines []string
	var line strings.Builder
	used := 0
	for _, word := range strings.Fields(text) {
		wordWidth := uniseg.StringWidth(word)
		if used > 0 && used+1+wordWidth <= width {
			line.WriteByte(' ')
			line.WriteString(word)
			used += 1 + wordWidth
			continue
		}
		if used > 0 {
			lines = append(lines, line.String())
			line.Reset()
			used = 0
		}
		g := uniseg.NewGraphemes(word)
		for g.Next() {
			if used+g.Width() > width && used > 0 {
				lines = append(lines, line.String())
				line.Reset()
				used = 0
			}
			if g.Width() > width {
				continue
			}
			line.WriteString(g.Str())
			used += g.Width()
		}
	}
	if used > 0 {
		lines = append(lines, line.String())
	}
	return lines
}
