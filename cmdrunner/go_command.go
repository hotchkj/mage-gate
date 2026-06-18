// Vision: GoCommand views argv with go/go test semantics—structural flag asserts, not substring luck.
package cmdrunner

import "strings"

// GoCommand wraps Command with Go-toolchain-specific accessors. External
// consumers can define their own wrapper types by embedding Command and
// reading through getters (e.g. BashCommand for shell-specific inspection).
type GoCommand struct{ Command }

// Go returns a GoCommand view over c for Go-toolchain flag inspection.
func (c Command) Go() GoCommand { return GoCommand{c} }

// Flags parses single-dash flags from the args, stopping at the first "--".
// Only the -flag=value form populates a non-empty value; bare -flag tokens
// produce an entry with an empty-string value. Two-token form (-flag value) is
// inherently ambiguous without a flag schema, so use FlagValue for that case.
// Repeated flags are preserved as []string slices; consumers decide semantics.
func (g GoCommand) Flags() map[string][]string {
	flags := make(map[string][]string)
	for _, token := range g.args {
		if token == "--" {
			break
		}
		if !strings.HasPrefix(token, "-") || token == "-" {
			continue
		}
		key, val, _ := parseFlag(token)
		flags[key] = append(flags[key], val)
	}
	return flags
}

// HasFlag reports whether any arg before "--" is -name or -name=....
func (g GoCommand) HasFlag(name string) bool {
	target := "-" + name
	for _, a := range g.args {
		if a == "--" {
			return false
		}
		if a == target || strings.HasPrefix(a, target+"=") {
			return true
		}
	}
	return false
}

// FlagValue returns the first value associated with -name. It recognizes
// both -name=value and -name value (two-token) forms. Scanning stops at "--".
func (g GoCommand) FlagValue(
	name string,
) (string, bool) {
	target := "-" + name
	for idx := 0; idx < len(g.args); idx++ {
		token := g.args[idx]
		if token == "--" {
			return "", false
		}
		if strings.HasPrefix(token, target+"=") {
			return token[len(target)+1:], true
		}
		if token != target {
			continue
		}
		next, ok := g.peekNonFlag(idx + 1)
		if ok {
			return next, true
		}
		return "", true
	}
	return "", false
}

// peekNonFlag returns args[idx] when it exists and is not a flag or passthrough marker.
func (g GoCommand) peekNonFlag(idx int) (string, bool) {
	if idx >= len(g.args) {
		return "", false
	}
	val := g.args[idx]
	if val == "--" || strings.HasPrefix(val, "-") {
		return "", false
	}
	return val, true
}

// parseFlag splits a single-dash flag token into key, value, hasEquals.
// "-coverprofile=out" -> ("coverprofile", "out", true)
// "-json"             -> ("json", "", false)
func parseFlag(token string) (key, val string, hasEquals bool) {
	s := strings.TrimPrefix(token, "-")
	idx := strings.IndexByte(s, '=')
	if idx < 0 {
		return s, "", false
	}
	return s[:idx], s[idx+1:], true
}
