//go:build special

package taggedrun

// NewDB is the one provider of *DB, and only a build that sets the special tag
// holds it. Google Wire and Yama must read the package under the same tag.
func NewDB(l *Logger) (*DB, func()) { return &DB{}, func() {} }
