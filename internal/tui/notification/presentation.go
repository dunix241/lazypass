package notification

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
)

// Format resolves presentation without changing lifecycle or queue state.
func Format(message Message, fallback tcell.Style) Presentation {
	if message.Style != nil {
		fallback = *message.Style
	}
	if message.Present != nil {
		return message.Present(message, fallback)
	}
	var text strings.Builder
	if message.Icon != "" {
		text.WriteString(message.Icon)
		text.WriteByte(' ')
	}
	if message.Title != "" {
		text.WriteString(message.Title)
		text.WriteString(": ")
	}
	text.WriteString(message.Text)
	for i, action := range message.Actions {
		if i >= 11 {
			break
		}
		fmt.Fprintf(&text, " · F%d %s", i+2, action.Label)
	}
	return Presentation{Text: text.String(), Style: fallback}
}
