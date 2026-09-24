package remote

import (
	"path"
	"strings"
)

// Namespace identifies which top-level tree a remote path lives in.
type Namespace int

const (
	// Cloud is the account's Cloud Drive.
	Cloud Namespace = iota
	// Trash is the account's Rubbish Bin.
	Trash
	// Shared is the set of folders other users shared with the account.
	Shared
)

// String returns the path prefix used for the namespace.
func (n Namespace) String() string {
	switch n {
	case Trash:
		return "trash:"
	case Shared:
		return "shared:"
	default:
		return ""
	}
}

// Path is a parsed remote path.
type Path struct {
	// NS is the namespace the path belongs to.
	NS Namespace
	// Parts are the cleaned path components below the namespace root.
	Parts []string
}

// Parse turns user input such as "docs/a.txt", "/docs" or "trash:/x" into
// a Path. Relative paths are resolved from the namespace root.
func Parse(s string) Path {
	p := Path{NS: Cloud}
	switch {
	case strings.HasPrefix(s, "trash:"):
		p.NS, s = Trash, strings.TrimPrefix(s, "trash:")
	case strings.HasPrefix(s, "shared:"):
		p.NS, s = Shared, strings.TrimPrefix(s, "shared:")
	}
	clean := path.Clean("/" + s)
	if clean == "/" {
		return p
	}
	p.Parts = strings.Split(strings.TrimPrefix(clean, "/"), "/")
	return p
}

// IsRoot reports whether the path points at the namespace root.
func (p Path) IsRoot() bool {
	return len(p.Parts) == 0
}

// Base returns the final path component, or "" for the root.
func (p Path) Base() string {
	if p.IsRoot() {
		return ""
	}
	return p.Parts[len(p.Parts)-1]
}

// Dir returns the parent path. The parent of the root is the root.
func (p Path) Dir() Path {
	if p.IsRoot() {
		return p
	}
	return Path{NS: p.NS, Parts: p.Parts[:len(p.Parts)-1]}
}

// Join returns a child path.
func (p Path) Join(name string) Path {
	parts := make([]string, len(p.Parts), len(p.Parts)+1)
	copy(parts, p.Parts)
	return Path{NS: p.NS, Parts: append(parts, name)}
}

// String renders the path in the same form Parse accepts.
func (p Path) String() string {
	return p.NS.String() + "/" + strings.Join(p.Parts, "/")
}
