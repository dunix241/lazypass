package vault

import (
	"lazypass/internal/tui/shared"
	"lazypass/internal/vault"
)

func (s *View) Footer(width int) string {
	if s.IsConfirmingDelete() {
		if s.deleteTarget.Kind == vault.FolderNode {
			return shared.FitFooter(width, "Enter confirm  •  Esc cancel", "Esc cancel")
		}
		return shared.FitFooter(width, "y yes  •  n no", "y/n")
	}
	return shared.FitFooter(width,
		"a add  •  e edit  •  o copy code  •  d remove  •  D recursive delete  •  r refresh  •  h back  •  l open/copy  •  V vault  •  v view  •  q quit",
		"a add  •  e edit  •  d remove  •  D recursive delete  •  r refresh  •  h back  •  l open/copy  •  V vault  •  v view  •  q quit",
		"a add  •  e edit  •  d delete  •  r refresh  •  h back  •  l open/copy  •  / search  •  V vault  •  t themes  •  v view  •  q quit",
		"a add  •  e edit  •  d delete  •  r refresh  •  h back  •  l open/copy  •  V vault  •  t themes  •  v view  •  q quit",
		"a add  •  e edit  •  d delete  •  h back  •  l open/copy  •  / search  •  V vault  •  t themes  •  v view  •  q quit",
		"a add  •  e edit  •  d delete  •  h back  •  l open/copy  •  V vault  •  t themes  •  v view  •  q quit",
		"a add  •  e edit  •  d delete  •  h back  •  l open/copy  •  / filter  •  t themes  •  v view  •  q quit",
		"a add  •  e edit  •  d delete  •  h back  •  l open/copy  •  v view  •  q quit",
		"h back  •  l open/copy  •  o code  •  / filter  •  v view  •  q quit",
		"h back  •  l open/copy  •  o code  •  v view  •  q quit",
		"h back • l open/copy • o code • q quit",
		"h back • l open/copy • q quit")
}
