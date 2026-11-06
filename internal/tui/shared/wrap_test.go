package shared

import (
	"strings"
	"testing"

	"github.com/rivo/uniseg"
)

func TestWrapTextKeepsLongCommand(t *testing.T) {
	text := `Run BITWARDENCLI_APPDATA_DIR="/home/user/.config/lazypass/bitwarden/work" bw login`
	lines := WrapText(text, 24)
	for _, line := range lines {
		if uniseg.StringWidth(line) > 24 {
			t.Fatal("line exceeds width")
		}
	}
	if strings.ReplaceAll(strings.Join(lines, ""), " ", "") != strings.ReplaceAll(text, " ", "") {
		t.Fatal("wrapped command lost characters")
	}
	if len(WrapText("", 24)) != 0 || len(WrapText("test", 0)) != 0 {
		t.Fatal("invalid dimensions")
	}
}
