package gatecheck

import (
	"testing"

	"github.com/hotchkj/mage-gate/internal/fsnorm"
)

func TestHasWindowsDriveLetterPrefix(t *testing.T) {
	t.Parallel()
	if !hasWindowsDriveLetterPrefix("C:/x") {
		t.Fatal("expected true for C:/x")
	}
	if hasWindowsDriveLetterPrefix("Cx") {
		t.Fatal("expected false for short string")
	}
	if hasWindowsDriveLetterPrefix("rel/path") {
		t.Fatal("expected false for relative")
	}
	if hasWindowsDriveLetterPrefix("C:") {
		t.Fatal("expected false when length < 3")
	}
	if hasWindowsDriveLetterPrefix("C:0") {
		t.Fatal("expected false when third rune is not '/'")
	}
	if !hasWindowsDriveLetterPrefix("C:/") {
		t.Fatal(`expected true for minimal letter-colon-slash prefix "C:/"`)
	}
	if hasWindowsDriveLetterPrefix("1:/x") {
		t.Fatal("expected false when first rune is not a letter")
	}
	for _, drive := range []string{"A:/", "Z:/", "a:/", "z:/"} {
		if !hasWindowsDriveLetterPrefix(drive + "x") {
			t.Fatalf("expected true for drive prefix %q", drive)
		}
	}
}

func TestIsWindowsDriveLexicalCanon(t *testing.T) {
	t.Parallel()
	c := fsnorm.Canonical(`C:\x\y`)
	if !IsWindowsDriveLexicalCanon(c) {
		t.Fatalf("expected drive canonical, got %q", c)
	}
	if IsWindowsDriveLexicalCanon("/a") {
		t.Fatal("unix-root canonical must not match drive-only predicate")
	}
	if IsWindowsDriveLexicalCanon("rel") {
		t.Fatal("expected relative false")
	}
}

func TestIsLexicallyAbsolute(t *testing.T) {
	t.Parallel()
	if !isLexicallyAbsolute("/a", "/a") {
		t.Fatal("expected unix abs")
	}
	if !isLexicallyAbsolute("rel", "/abs") {
		t.Fatal("expected canonical unix root to win over relative original")
	}
	c := fsnorm.Canonical(`C:\x\y`)
	if !isLexicallyAbsolute(`C:\x\y`, c) {
		t.Fatalf("expected windows drive path, canonical=%q", c)
	}
	if isLexicallyAbsolute("rel", "rel") {
		t.Fatal("expected relative false")
	}
}
