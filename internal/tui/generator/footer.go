package generator

import "lazypass/internal/tui/shared"

func (s *View) Footer(width int) string {
	contextHint := ""
	switch s.selected {
	case focusLength:
		contextHint = "←/→ length"
	case focusUpper, focusLower, focusNumbers, focusSymbols, focusAmbiguous:
		contextHint = "<space> toggle"
	}
	bases := []string{
		"<tab> focus  •  a add  •  r regenerate  •  c copy  •  V vault  •  t themes  •  v view  •  q quit",
		"a add  •  c copy  •  V vault  •  t themes  •  v view  •  q quit",
		"a add  •  v view  •  q quit", "q quit",
	}
	if contextHint == "" {
		return shared.FitFooter(width, bases...)
	}
	candidates := make([]string, 0, len(bases)+1)
	for _, base := range bases {
		candidates = append(candidates, contextHint+"  •  "+base)
	}
	candidates = append(candidates, contextHint)
	return shared.FitFooter(width, candidates...)
}
