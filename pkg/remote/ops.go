package remote

import (
	"errors"
	"fmt"
	"io"
	"io/fs"

	mega "github.com/t3rm1n4l/go-mega"
)

// ErrIsDir is returned when a file operation targets a folder.
var ErrIsDir = errors.New("is a folder")

// ErrExists is returned when an operation would overwrite a node.
var ErrExists = errors.New("already exists")

// IsNotFound reports whether err is a NotFoundError.
func IsNotFound(err error) bool {
	var nf *NotFoundError
	return errors.As(err, &nf)
}

// RemoveOptions tunes Remove.
type RemoveOptions struct {
	// Recursive allows removing folders.
	Recursive bool
	// Permanent destroys the node instead of moving it to the trash.
	// Nodes already in the trash are always destroyed.
	Permanent bool
	// IgnoreMissing turns a missing path into a no-op.
	IgnoreMissing bool
}

// RemoveResult reports what Remove did.
type RemoveResult struct {
	// Removed is false when the path was missing and ignored.
	Removed bool
	// Destroyed is true when the node was deleted rather than trashed.
	Destroyed bool
}

// Remove trashes or destroys the node at p.
func (c *Client) Remove(p Path, o RemoveOptions) (RemoveResult, error) {
	if p.IsRoot() {
		return RemoveResult{}, fmt.Errorf("refusing to remove %s", p)
	}
	n, err := c.lookup(p)
	if o.IgnoreMissing && IsNotFound(err) {
		return RemoveResult{}, nil
	}
	if err != nil {
		return RemoveResult{}, err
	}
	if isDir(n) && !o.Recursive {
		return RemoveResult{}, fmt.Errorf("%s: %w", p, ErrIsDir)
	}
	destroy := o.Permanent || p.NS == Trash
	if err := c.deleteNode(n, destroy); err != nil {
		return RemoveResult{}, fmt.Errorf("%s: %w", p, err)
	}
	return RemoveResult{Removed: true, Destroyed: destroy}, nil
}

// EmptyTrash permanently deletes everything in the trash and returns the
// number of top-level items removed.
func (c *Client) EmptyTrash() (int, error) {
	nodes, err := c.children(Path{NS: Trash})
	if err != nil {
		return 0, err
	}
	for i, n := range nodes {
		if err := c.deleteNode(n, true); err != nil {
			return i, fmt.Errorf("%s: %w", n.GetName(), err)
		}
	}
	return len(nodes), nil
}

// MoveFunc is called after each successful MovePaths step.
type MoveFunc func(src, dst Path)

// MovePaths moves each source into dst when dst is an existing folder, or
// moves and renames a single source to dst otherwise. Paths are resolved
// first so names containing "/" keep their real name. fn may be nil.
func (c *Client) MovePaths(srcs []Path, dst Path, fn MoveFunc) error {
	resolved := make([]Path, len(srcs))
	for i, s := range srcs {
		p, _, err := c.resolve(s)
		if err != nil {
			return err
		}
		resolved[i] = p
	}
	dst, d, err := c.resolve(dst)
	if err != nil && !IsNotFound(err) {
		return err
	}
	switch {
	case d != nil && isDir(d):
		return c.moveInto(resolved, dst, d, fn)
	case d != nil:
		return fmt.Errorf("%s: %w", dst, ErrExists)
	case len(resolved) > 1:
		return &NotFoundError{Path: dst}
	}
	parent, err := c.lookup(dst.Dir())
	if err != nil {
		return err
	}
	return c.movePath(resolved[0], parent, dst, fn)
}

// moveInto moves every source into the folder d, located at dst.
func (c *Client) moveInto(srcs []Path, dst Path, d *mega.Node, fn MoveFunc) error {
	for _, s := range srcs {
		if err := c.movePath(s, d, dst.Join(s.Base()), fn); err != nil {
			return err
		}
	}
	return nil
}

// movePath moves src into the folder parent under target's name.
func (c *Client) movePath(src Path, parent *mega.Node, target Path, fn MoveFunc) error {
	if src.IsRoot() {
		return errors.New("cannot move a root folder")
	}
	n, err := c.lookup(src)
	if err != nil {
		return err
	}
	if !isDir(parent) {
		return fmt.Errorf("%s: %w", target.Dir(), ErrNotDir)
	}
	clash, err := c.child(parent, target.Base())
	if err != nil {
		return err
	}
	if clash != nil && clash != n {
		return fmt.Errorf("%s: %w", target, ErrExists)
	}
	if src.Dir().String() != target.Dir().String() {
		if err := c.move(n, parent); err != nil {
			return err
		}
	}
	if src.Base() != target.Base() {
		if err := c.rename(n, target.Base()); err != nil {
			return err
		}
	}
	if fn != nil {
		fn(src, target)
	}
	return nil
}

// PublicLink exports a public link for the node at p.
func (c *Client) PublicLink(p Path, includeKey bool) (string, error) {
	n, err := c.lookup(p)
	if err != nil {
		return "", err
	}
	u, err := c.link(n, includeKey)
	if err != nil {
		return "", fmt.Errorf("%s: %w", p, err)
	}
	return u, nil
}

// Cat streams the file at p to w. The MAC is verified at the end, so a
// corrupted stream is reported only after data was written.
func (c *Client) Cat(p Path, w io.Writer, fn ProgressFunc) error {
	n, err := c.lookup(p)
	if err != nil {
		return err
	}
	if isDir(n) {
		return &fs.PathError{Op: "cat", Path: p.String(), Err: ErrIsDir}
	}
	if err := c.stream(n, w, fn); err != nil {
		return fmt.Errorf("%s: %w", p, err)
	}
	return nil
}
