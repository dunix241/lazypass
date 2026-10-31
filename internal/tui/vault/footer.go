package vault

import "lazypass/internal/tui/shared"

func (s *View) Footer(width int) string {
	return shared.FitFooter(width,
		"h back  •  l open/copy  •  / filter  •  t themes  •  v view  •  q quit",
		"h back  •  l open/copy  •  / filter  •  v view  •  q quit",
		"h back  •  l open/copy  •  v view  •  q quit",
		"h back • l open/copy • q quit")
}
