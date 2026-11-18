package remote

import (
	"fmt"

	mega "github.com/t3rm1n4l/go-mega"
)

// WalkFunc is called for every entry visited by Walk. p is the entry's
// path, whose parts are the real node names.
type WalkFunc func(p Path, e Entry) error

// Resolve finds the entry at p and returns p rewritten so that each part is
// a real node name. This lets "a/b/c" address a folder literally named
// "a/b" when no folder "a" leads to a match.
func (c *Client) Resolve(p Path) (Path, Entry, error) {
	p, n, err := c.resolve(p)
	if err != nil {
		return p, Entry{}, err
	}
	return p, c.entry(p, n), nil
}

// Walk visits every entry below p, depth first, excluding p itself.
func (c *Client) Walk(p Path, fn WalkFunc) error {
	return c.walk(p, func(cp Path, n *mega.Node) error {
		return fn(cp, c.entry(cp, n))
	})
}

// Mkdir creates a folder. With parents set, missing ancestors are created
// and an existing folder is not an error.
func (c *Client) Mkdir(p Path, parents bool) (Entry, error) {
	n, err := c.mkdir(p, parents)
	if err != nil {
		return Entry{}, err
	}
	return c.entry(p, n), nil
}

// Upload stores the local file src at dst. The parent folder must exist.
// When replace is set and a file already exists at dst, it is moved to the
// trash after the new upload succeeds; otherwise ErrExists is returned.
func (c *Client) Upload(src string, dst Path, replace bool, fn ProgressFunc) (Entry, error) {
	if dst.IsRoot() {
		return Entry{}, fmt.Errorf("%s: %w", dst, ErrIsDir)
	}
	parentPath, parent, err := c.resolve(dst.Dir())
	if err != nil {
		return Entry{}, err
	}
	if !isDir(parent) {
		return Entry{}, fmt.Errorf("%s: %w", dst.Dir(), ErrNotDir)
	}
	n, err := c.uploadNode(src, parent, dst.Base(), replace, fn)
	if n == nil {
		return Entry{}, err
	}
	return c.entry(parentPath.Join(dst.Base()), n), err
}

// Download writes the file at src to the local path dst atomically, via a
// temp file in the destination folder that is renamed once MEGA's MAC has
// been verified. Folders are not downloaded; see the transfer package.
func (c *Client) Download(src Path, dst string, fn ProgressFunc) error {
	p, n, err := c.resolve(src)
	if err != nil {
		return err
	}
	if isDir(n) {
		return fmt.Errorf("%s: %w", p, ErrIsDir)
	}
	return c.downloadNode(n, dst, fn)
}
