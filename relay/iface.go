package relay

import "path/filepath"

// matchInterface reports whether name is included by any include pattern
// and not excluded by any exclude pattern. Patterns use filepath.Match glob
// syntax ("*", "?", character classes). Invalid patterns are treated as
// non-matching rather than erroring.
func matchInterface(name string, include, exclude []string) bool {
	included := false
	for _, p := range include {
		ok, err := filepath.Match(p, name)
		if err == nil && ok {
			included = true
			break
		}
	}
	if !included {
		return false
	}
	for _, p := range exclude {
		ok, err := filepath.Match(p, name)
		if err == nil && ok {
			return false
		}
	}
	return true
}
