// Vision: Canonical artifact projections—logical paths for FileOps and argv vs native host seams.
package harness

import (
	"fmt"
	"strings"

	"github.com/hotchkj/mage-gate/internal/fileopspath"
	"github.com/hotchkj/mage-gate/internal/fsnorm"
)

// artifactPaths projects the gate artifact directory under a host root into logical FileOps/command
// paths and native host paths for documented OS seams.
type artifactPaths struct {
	root       string
	logicalDir string
}

func newArtifactPaths(root, artifactSubdir string) (artifactPaths, error) {
	if strings.TrimSpace(root) == "" {
		return artifactPaths{}, ErrRootRequired
	}
	logical, err := fileopspath.LogicalContainedRelative(root, artifactSubdir)
	if err != nil {
		return artifactPaths{}, err
	}
	logical = fsnorm.Canonical(logical)
	if logical == "" {
		logical = "."
	}
	return artifactPaths{
		root:       root,
		logicalDir: logical,
	}, nil
}

// Dir returns the canonical logical artifact directory for FileOps, with no trailing slash.
func (ap artifactPaths) Dir() string {
	return ap.logicalDir
}

func (ap artifactPaths) FileOpsPath(parts ...string) (string, error) {
	return ap.joinLexicalContained(parts)
}

func (ap artifactPaths) CommandPath(parts ...string) (string, error) {
	return ap.FileOpsPath(parts...)
}

func (ap artifactPaths) HostPath(parts ...string) (string, error) {
	logicalRel, err := ap.joinLexicalContained(parts)
	if err != nil {
		return "", err
	}
	return ResolveWithinRoot(ap.root, logicalSegmentsForResolve(logicalRel)...)
}

func (ap artifactPaths) Validate(parts ...string) error {
	_, err := ap.joinLexicalContained(parts)
	return err
}

func logicalSegmentsForResolve(logicalRelative string) []string {
	logicalRelative = fsnorm.Canonical(logicalRelative)
	if logicalRelative == "" || logicalRelative == "." {
		return nil
	}
	return strings.Split(logicalRelative, "/")
}

func (ap artifactPaths) joinLexicalContained(parts []string) (string, error) {
	var candidate string
	if len(parts) == 0 {
		candidate = ap.logicalDir
	} else {
		joined := make([]string, 0, len(parts)+1)
		joined = append(joined, ap.logicalDir)
		joined = append(joined, parts...)
		candidate = fsnorm.Join(joined...)
	}
	return candidate, lexicalContainmentRelativeToArtifact(ap.logicalDir, candidate)
}

func lexicalContainmentRelativeToArtifact(artifactDir, resolved string) error {
	artifactDir = fsnorm.Canonical(artifactDir)
	resolved = fsnorm.Canonical(resolved)
	rel, err := fsnorm.Rel(artifactDir, resolved)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrPathTraversal, err)
	}
	if relContainsDotDotSegment(rel) {
		return fmt.Errorf("%w: %q escapes artifact directory %q",
			ErrPathTraversal, resolved, artifactDir)
	}
	return nil
}

func relContainsDotDotSegment(rel string) bool {
	if rel == "" || rel == "." {
		return false
	}
	for _, segment := range strings.Split(rel, "/") {
		if segment == ".." {
			return true
		}
	}
	return false
}
