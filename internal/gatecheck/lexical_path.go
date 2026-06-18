// Vision: lexical absolute-path detection for CRAP / gocyclo path alignment after fsnorm.Canonical.
package gatecheck

import (
	"path/filepath"
	"strings"
)

// minWindowsDrivePathLen is the shortest `C:/` form after Canonical (letter, colon, slash).
const minWindowsDrivePathLen = 3

func hasWindowsDriveLetterPrefix(canonPath string) bool {
	if len(canonPath) < minWindowsDrivePathLen {
		return false
	}
	c0 := canonPath[0]
	letter := (c0 >= 'A' && c0 <= 'Z') || (c0 >= 'a' && c0 <= 'z')
	return letter && canonPath[1] == ':' && canonPath[2] == '/'
}

// IsWindowsDriveLexicalCanon reports whether canonicalPath is the letter-colon-slash
// form produced by fsnorm.Canonical for a Windows drive path. It is not
// filepath.IsAbs on Unix, but should still bypass root joining (same as CRAP path logic).
func IsWindowsDriveLexicalCanon(canonicalPath string) bool {
	return hasWindowsDriveLetterPrefix(canonicalPath)
}

// isLexicallyAbsolute reports whether a path should be treated as absolute for
// cross-platform matching: OS-absolute originalPath, Unix-root canonicalPath, or
// Windows drive form in canonicalPath (after fsnorm.Canonical).
func isLexicallyAbsolute(originalPath, canonicalPath string) bool {
	if filepath.IsAbs(originalPath) {
		return true
	}
	if strings.HasPrefix(canonicalPath, "/") {
		return true
	}
	return hasWindowsDriveLetterPrefix(canonicalPath)
}
