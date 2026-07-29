// Package deps: sysml.lock lockfile read/write.
package deps

import (
	"sort"
	"strings"
)

// Lock pins dependency names to their resolved commit SHAs.
type Lock struct {
	SHA map[string]string
}

// NewLock returns an empty lock.
func NewLock() *Lock {
	return &Lock{SHA: map[string]string{}}
}

// ReadLock parses a sysml.lock file (minimal `name = "sha"` line list).
func ReadLock(content []byte) (*Lock, error) {
	lock := NewLock()
	for _, raw := range strings.Split(string(content), "\n") {
		line := strings.TrimSpace(stripComment(raw))
		if line == "" {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		name := strings.TrimSpace(key)
		sha, err := parseString(strings.TrimSpace(val))
		if err != nil {
			return nil, err
		}
		if name != "" {
			lock.SHA[name] = sha
		}
	}
	return lock, nil
}

// Bytes serializes the lock back to sysml.lock form, sorted by name for
// deterministic output.
func (l *Lock) Bytes() []byte {
	names := make([]string, 0, len(l.SHA))
	for name := range l.SHA {
		names = append(names, name)
	}
	sort.Strings(names)
	var b strings.Builder
	for _, name := range names {
		b.WriteString(name)
		b.WriteString(" = ")
		b.WriteString(quote(l.SHA[name]))
		b.WriteByte('\n')
	}
	return []byte(b.String())
}

// quote wraps s in double quotes for lock/manifest string serialization.
func quote(s string) string {
	return "\"" + s + "\""
}
