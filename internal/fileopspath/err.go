package fileopspath

import "errors"

// ErrPathTraversal is returned when a path cannot be constrained under the configured
// filesystem root (lexical/OS containment).
var ErrPathTraversal = errors.New("path would escape root")

// ErrFileOpsRootAlreadyBound is returned when Root is invoked with a different non-empty
// root after filesystem operations were configured; FileOps implementations bind once per instance.
var ErrFileOpsRootAlreadyBound = errors.New("fileops root already bound")
