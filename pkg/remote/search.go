package remote

import (
	"fmt"
	"path"
	"strings"

	mega "github.com/t3rm1n4l/go-mega"
)

// FindOptions filters the entries returned by Find.
type FindOptions struct {
	// Name is a glob matched against entry names; empty matches all.
	Name string
	// Type restricts results to TypeFile or TypeFolder; empty matches all.
	Type string
	// IgnoreCase makes Name case-insensitive.
	IgnoreCase bool
}

// pattern returns the glob to match, lower-cased when ignoring case.
func (o FindOptions) pattern() string {
	if o.IgnoreCase {
		return strings.ToLower(o.Name)
	}
	return o.Name
}

// Validate checks the type and glob.
func (o FindOptions) Validate() error {
	if o.Type != "" && o.Type != TypeFile && o.Type != TypeFolder {
		return fmt.Errorf("type must be %q or %q", TypeFile, TypeFolder)
	}
	if _, err := path.Match(o.pattern(), ""); err != nil {
		return fmt.Errorf("bad name pattern: %w", err)
	}
	return nil
}

// Matches reports whether e satisfies the filters.
func (o FindOptions) Matches(e Entry) bool {
	if o.Type != "" && e.Type != o.Type {
		return false
	}
	pattern := o.pattern()
	if pattern == "" {
		return true
	}
	name := e.Name
	if o.IgnoreCase {
		name = strings.ToLower(name)
	}
	ok, _ := path.Match(pattern, name)
	return ok
}

// Find returns the entries below root that match o.
func (c *Client) Find(root Path, o FindOptions) ([]Entry, error) {
	if err := o.Validate(); err != nil {
		return nil, err
	}
	var found []Entry
	err := c.walk(root, func(p Path, n *mega.Node) error {
		if e := c.entry(p, n); o.Matches(e) {
			found = append(found, e)
		}
		return nil
	})
	return found, err
}

// Usage totals the contents of a path.
type Usage struct {
	// Path is the measured path.
	Path string `json:"path"`
	// Bytes is the total size of all files below the path.
	Bytes int64 `json:"bytes"`
	// Files is the number of files below the path.
	Files int `json:"files"`
	// Folders is the number of folders below the path.
	Folders int `json:"folders"`
}

// DiskUsage totals the files and folders below p.
func (c *Client) DiskUsage(p Path) (Usage, error) {
	u := Usage{Path: p.String()}
	if !p.IsRoot() {
		n, err := c.lookup(p)
		if err != nil {
			return u, err
		}
		if !isDir(n) {
			u.Bytes, u.Files = n.GetSize(), 1
			return u, nil
		}
	}
	err := c.walk(p, func(_ Path, n *mega.Node) error {
		if isDir(n) {
			u.Folders++
		} else {
			u.Files++
			u.Bytes += n.GetSize()
		}
		return nil
	})
	return u, err
}
