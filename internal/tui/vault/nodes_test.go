package vault

import (
	"strings"
	"testing"
	"unicode"

	"lazypass/internal/vault"
)

func TestKnownEntryTypesHaveDistinctIconsAndTextFallbacks(t *testing.T) {
	icons := map[string]bool{}
	for _, kind := range []vault.EntryType{vault.LoginEntry, vault.CardEntry, vault.IdentityEntry, vault.NoteEntry, vault.SSHKeyEntry} {
		node := vault.Node{Name: "fixture", Kind: vault.EntryNode, EntryType: kind, TypeKnown: true}
		label := nodeLabel(node, true)
		if !strings.HasSuffix(label, " fixture") || icons[label] {
			t.Fatalf("missing or duplicated type icon: %q", label)
		}
		icons[label] = true
		fallback := nodeLabel(node, false)
		if fallback != "["+kind.String()+"] fixture" {
			t.Fatalf("fallback=%q", fallback)
		}
		for _, r := range fallback {
			if unicode.Is(unicode.Co, r) {
				t.Fatal("Nerd Font glyph in text fallback")
			}
		}
	}
}
