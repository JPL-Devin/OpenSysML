// Package deps parses the sysml.toml project manifest and resolves declared
// dependencies into the shared symbol index.
package deps

import (
	"fmt"
	"strings"
)

// Dep is a single declared dependency: either a local path or a git source
// pinned by exactly one of Rev/Tag/Branch.
type Dep struct {
	Path   string
	Git    string
	Rev    string
	Tag    string
	Branch string
}

// Manifest is the parsed sysml.toml at a workspace or dependency root.
type Manifest struct {
	LibraryPaths []string
	Dependencies map[string]Dep
}

// ParseManifest parses a minimal TOML subset: top-level `library-paths`
// string array, and `[dependencies]` / `[dependencies.<name>]` tables with
// string keys path|git|rev|tag|branch. Inline tables
// (`name = { key = "v", ... }`) under `[dependencies]` are also accepted.
func ParseManifest(content []byte) (*Manifest, error) {
	m := &Manifest{Dependencies: map[string]Dep{}}
	// section: "" (top-level), "dependencies", or a specific dep name under
	// [dependencies.<name>].
	section := ""
	depName := ""
	lines := strings.Split(string(content), "\n")
	for i, raw := range lines {
		line := stripComment(raw)
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") {
			header, ok := strings.CutSuffix(line, "]")
			if !ok {
				return nil, fmt.Errorf("sysml.toml line %d: unterminated section header %q", i+1, raw)
			}
			header = strings.TrimSpace(strings.TrimPrefix(header, "["))
			switch {
			case header == "dependencies":
				section, depName = "dependencies", ""
			case strings.HasPrefix(header, "dependencies."):
				name := strings.TrimSpace(strings.TrimPrefix(header, "dependencies."))
				if name == "" {
					return nil, fmt.Errorf("sysml.toml line %d: empty dependency name", i+1)
				}
				section, depName = "dep", name
				if _, ok := m.Dependencies[name]; !ok {
					m.Dependencies[name] = Dep{}
				}
			default:
				section, depName = header, ""
			}
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("sysml.toml line %d: expected key = value, got %q", i+1, raw)
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		switch section {
		case "":
			if key == "library-paths" {
				items, err := parseStringArray(val)
				if err != nil {
					return nil, fmt.Errorf("sysml.toml line %d: %w", i+1, err)
				}
				m.LibraryPaths = items
			}
			// unknown top-level keys ignored for forward-compat
		case "dependencies":
			// name = { inline table } OR name = "path"
			dep, err := parseInlineDep(val)
			if err != nil {
				return nil, fmt.Errorf("sysml.toml line %d: %w", i+1, err)
			}
			m.Dependencies[key] = dep
		case "dep":
			s, err := parseString(val)
			if err != nil {
				return nil, fmt.Errorf("sysml.toml line %d: %w", i+1, err)
			}
			d := m.Dependencies[depName]
			if err := setDepField(&d, key, s); err != nil {
				return nil, fmt.Errorf("sysml.toml line %d: %w", i+1, err)
			}
			m.Dependencies[depName] = d
		}
	}
	return m, nil
}

func stripComment(s string) string {
	inStr := false
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			inStr = !inStr
		case '#':
			if !inStr {
				return s[:i]
			}
		}
	}
	return s
}

func parseString(v string) (string, error) {
	v = strings.TrimSpace(v)
	if len(v) < 2 || v[0] != '"' || v[len(v)-1] != '"' {
		return "", fmt.Errorf("expected quoted string, got %q", v)
	}
	return v[1 : len(v)-1], nil
}

func parseStringArray(v string) ([]string, error) {
	v = strings.TrimSpace(v)
	inner, ok := strings.CutPrefix(v, "[")
	if !ok {
		return nil, fmt.Errorf("expected array, got %q", v)
	}
	inner, ok = strings.CutSuffix(strings.TrimSpace(inner), "]")
	if !ok {
		return nil, fmt.Errorf("unterminated array %q", v)
	}
	inner = strings.TrimSpace(inner)
	if inner == "" {
		return nil, nil
	}
	var out []string
	for _, part := range strings.Split(inner, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		s, err := parseString(part)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

func parseInlineDep(v string) (Dep, error) {
	v = strings.TrimSpace(v)
	if inner, ok := strings.CutPrefix(v, "{"); ok {
		inner, ok = strings.CutSuffix(strings.TrimSpace(inner), "}")
		if !ok {
			return Dep{}, fmt.Errorf("unterminated inline table %q", v)
		}
		var d Dep
		inner = strings.TrimSpace(inner)
		if inner == "" {
			return d, nil
		}
		for _, part := range strings.Split(inner, ",") {
			k, val, ok := strings.Cut(part, "=")
			if !ok {
				return Dep{}, fmt.Errorf("expected key = value in inline table, got %q", part)
			}
			s, err := parseString(strings.TrimSpace(val))
			if err != nil {
				return Dep{}, err
			}
			if err := setDepField(&d, strings.TrimSpace(k), s); err != nil {
				return Dep{}, err
			}
		}
		return d, nil
	}
	// bare string shorthand => local path
	s, err := parseString(v)
	if err != nil {
		return Dep{}, err
	}
	return Dep{Path: s}, nil
}

func setDepField(d *Dep, key, val string) error {
	switch key {
	case "path":
		d.Path = val
	case "git":
		d.Git = val
	case "rev":
		d.Rev = val
	case "tag":
		d.Tag = val
	case "branch":
		d.Branch = val
	default:
		// ignore unknown keys for forward-compat
	}
	return nil
}
