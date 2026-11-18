package remote

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	mega "github.com/t3rm1n4l/go-mega"
)

const (
	// TypeFile is the Entry type of regular files.
	TypeFile = "file"
	// TypeFolder is the Entry type of folders.
	TypeFolder = "folder"
)

const (
	// SortName orders entries by name.
	SortName = "name"
	// SortSize orders entries by size, largest first.
	SortSize = "size"
	// SortTime orders entries by modification time, newest first.
	SortTime = "time"
)

// SortKeys lists the keys accepted by Order.
var SortKeys = []string{SortName, SortSize, SortTime}

// Entry describes a remote node without exposing go-mega types.
type Entry struct {
	// Name is the node's file name.
	Name string `json:"name"`
	// Path is the node's full remote path.
	Path string `json:"path"`
	// Type is TypeFile or TypeFolder.
	Type string `json:"type"`
	// Size is the size in bytes; for folders, the total of all files below.
	Size int64 `json:"size"`
	// Modified is the node's timestamp.
	Modified time.Time `json:"modified"`
	// Handle is MEGA's node handle.
	Handle string `json:"handle"`
}

// IsDir reports whether the entry is a folder.
func (e Entry) IsDir() bool {
	return e.Type == TypeFolder
}

// Entry builds the Entry for node n located at path p.
func (c *Client) Entry(p Path, n *mega.Node) Entry {
	e := Entry{
		Name:     n.GetName(),
		Path:     p.String(),
		Type:     TypeFile,
		Size:     c.Size(n),
		Modified: n.GetTimeStamp(),
		Handle:   n.GetHash(),
	}
	if IsDir(n) {
		e.Type = TypeFolder
	}
	return e
}

// Stat returns the Entry for the node at p.
func (c *Client) Stat(p Path) (Entry, error) {
	n, err := c.Lookup(p)
	if err != nil {
		return Entry{}, err
	}
	return c.Entry(p, n), nil
}

// List returns the entries at p. For a folder these are its children, or
// every descendant when recursive is set; for a file it is the file itself.
func (c *Client) List(p Path, recursive bool) ([]Entry, error) {
	if recursive {
		return c.listAll(p)
	}
	if !p.IsRoot() {
		n, err := c.Lookup(p)
		if err != nil && !errors.Is(err, ErrSharedRoot) {
			return nil, err
		}
		if n != nil && !IsDir(n) {
			return []Entry{c.Entry(p, n)}, nil
		}
	}
	nodes, err := c.Children(p)
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, len(nodes))
	for i, n := range nodes {
		entries[i] = c.Entry(p.Join(n.GetName()), n)
	}
	return entries, nil
}

// listAll returns every descendant of p.
func (c *Client) listAll(p Path) ([]Entry, error) {
	var entries []Entry
	err := c.Walk(p, func(cp Path, n *mega.Node) error {
		entries = append(entries, c.Entry(cp, n))
		return nil
	})
	return entries, err
}

// Order describes how entries are sorted.
type Order struct {
	// Key is one of SortKeys; the empty key means SortName.
	Key string
	// Reverse flips the order.
	Reverse bool
}

// Validate checks the sort key.
func (o Order) Validate() error {
	if o.Key == "" || slices.Contains(SortKeys, o.Key) {
		return nil
	}
	return fmt.Errorf("sort key must be one of %s", strings.Join(SortKeys, ", "))
}

// Compare orders a against b. Ties fall back to name order.
func (o Order) Compare(a, b Entry) int {
	c := 0
	switch o.Key {
	case SortSize:
		c = cmp.Compare(b.Size, a.Size)
	case SortTime:
		c = b.Modified.Compare(a.Modified)
	}
	if c == 0 {
		c = strings.Compare(a.Name, b.Name)
	}
	if o.Reverse {
		return -c
	}
	return c
}

// Sort orders entries in place.
func (o Order) Sort(entries []Entry) {
	slices.SortStableFunc(entries, o.Compare)
}
