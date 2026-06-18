// Vision: Coverage profile path parsing helpers for robust filtering across OS formats.
package gatecheck

import "strings"

func coverProfileFilePath(token string) string {
	return trimLineColSuffix(strings.TrimSpace(token))
}

// trimLineColSuffix removes a trailing ":<line>:<col>" from a coverage token when both
// line and column components are numeric. This preserves Windows/UNC paths that
// legitimately contain colons before the line/column suffix.
func trimLineColSuffix(token string) string {
	lastColon := strings.LastIndex(token, ":")
	if lastColon <= 0 {
		return token
	}
	// Coverage rows use "file:line.start,line.end" where line/column ranges include a dot.
	if isCoverageRange(token[lastColon+1:]) {
		return token[:lastColon]
	}

	firstColon := strings.LastIndex(token[:lastColon], ":")
	if firstColon <= 0 {
		return token
	}
	linePart := token[firstColon+1 : lastColon]
	colPart := token[lastColon+1:]
	if !isAllDigits(linePart) || !isAllDigits(colPart) {
		return token
	}
	return token[:firstColon]
}

func isCoverageRange(value string) bool {
	comma := strings.Index(value, ",")
	if comma <= 0 || comma == len(value)-1 {
		return false
	}
	return isLineOrCol(value[:comma]) && isLineOrCol(value[comma+1:])
}

func isLineOrCol(value string) bool {
	if value == "" {
		return false
	}
	const maxLineColParts = 2
	parts := strings.Split(value, ".")
	if len(parts) > maxLineColParts {
		return false
	}
	if len(parts) == 1 {
		return isAllDigits(parts[0])
	}
	return isAllDigits(parts[0]) && isAllDigits(parts[1])
}

func isAllDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
