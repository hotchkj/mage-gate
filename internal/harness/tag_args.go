// Vision: Build tag argv is merged once so coverage, CRAP, and mutation observe the same quality scope.
package harness

import (
	"strings"
)

func buildTagsFlag(tags string) []string {
	tags = normalizeBuildTags(tags)
	if tags == "" {
		return nil
	}
	return []string{"-tags=" + tags}
}

func mergeBuildTags(scopeTags string, consumerArgs []string) (tags string, filtered []string) {
	values := buildTagSet{}
	values.add(scopeTags)

	filtered = make([]string, 0, len(consumerArgs))
	for argIndex := 0; argIndex < len(consumerArgs); argIndex++ {
		arg := consumerArgs[argIndex]
		switch {
		case arg == "-tags" || arg == "--tags":
			if argIndex+1 < len(consumerArgs) {
				values.add(consumerArgs[argIndex+1])
				argIndex++
			} else {
				filtered = append(filtered, arg)
			}
		case strings.HasPrefix(arg, "-tags="):
			values.add(strings.TrimPrefix(arg, "-tags="))
		case strings.HasPrefix(arg, "--tags="):
			values.add(strings.TrimPrefix(arg, "--tags="))
		default:
			filtered = append(filtered, arg)
		}
	}
	return values.join(), filtered
}

func normalizeBuildTags(tags string) string {
	values := buildTagSet{}
	values.add(tags)
	return values.join()
}

type buildTagSet struct {
	seen map[string]struct{}
	tags []string
}

func (set *buildTagSet) add(value string) {
	for _, tag := range splitBuildTags(value) {
		if set.seen == nil {
			set.seen = make(map[string]struct{})
		}
		if _, ok := set.seen[tag]; ok {
			continue
		}
		set.seen[tag] = struct{}{}
		set.tags = append(set.tags, tag)
	}
}

func (set *buildTagSet) join() string {
	return strings.Join(set.tags, ",")
}

func splitBuildTags(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n'
	})
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field != "" {
			out = append(out, field)
		}
	}
	return out
}
