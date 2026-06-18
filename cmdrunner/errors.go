// Vision: cmdrunner-wide nil-dependency sentinels shared by runner, capture, and resolver constructors.
package cmdrunner

import "errors"

// Nil dependency sentinels — returned when required dependencies are nil.
var (
	// ErrNilDependency when CommandRunner, ToolResolver, or other required dependency is nil.
	ErrNilDependency = errors.New("nil dependency")
)
