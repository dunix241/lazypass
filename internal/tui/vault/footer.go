package vault

import "lazypass/internal/tui/shared"

func (s *View) Footer(width int) string {
	if s.IsConfirmingDelete() {
		return shared.FitFooter(width, "y yes  •  n no", "y/n")
	}
	return shared.FitFooter(width,
		"a add  •  e edit  •  d delete  •  h back  •  l open/copy  •  / filter  •  t themes  •  v view  •  q quit",
		"a add  •  e edit  •  d delete  •  h back  •  l open/copy  •  v view  •  q quit",
		"h back  •  l open/copy  •  / filter  •  v view  •  q quit",
		"h back  •  l open/copy  •  v view  •  q quit",
		"h back • l open/copy • q quit")
}
