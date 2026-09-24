// Package remote wraps go-mega with path-oriented helpers.
package remote

import (
	"errors"
	"fmt"
	"sort"

	mega "github.com/t3rm1n4l/go-mega"
)

// ErrMFARequired is returned when the account needs a 2FA code.
var ErrMFARequired = mega.EMFAREQUIRED

// ErrSharedRoot is returned when an operation targets the virtual
// "shared:" root, which is not a real folder.
var ErrSharedRoot = errors.New("shared: is a virtual folder")

// ErrNotDir is returned when a folder operation targets a file.
var ErrNotDir = errors.New("not a folder")

// NotFoundError reports a missing remote path.
type NotFoundError struct {
	// Path is the path that could not be resolved.
	Path Path
}

// Error implements the error interface.
func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s: no such file or folder", e.Path)
}

// Options tunes the underlying MEGA client.
type Options struct {
	// Debugf receives go-mega debug output when non-nil.
	Debugf func(format string, v ...any)
}

// Client is a logged-in MEGA client.
type Client struct {
	// m is the underlying go-mega client.
	m *mega.Mega
	// sizes caches folder sizes by node handle; mutations reset it.
	sizes map[string]int64
}

// newMega builds a configured go-mega client.
func newMega(o Options) *mega.Mega {
	return mega.New().SetLogger(nil).SetDebugger(o.Debugf)
}

// Login authenticates with email and password, and an optional 2FA code.
func Login(email, password, mfa string, o Options) (*Client, error) {
	m := newMega(o)
	if err := m.MultiFactorLogin(email, password, mfa); err != nil {
		return nil, err
	}
	return &Client{m: m}, nil
}

// Resume restores a session previously obtained through Login.
func Resume(sessionID string, masterKey []byte, o Options) (*Client, error) {
	m := newMega(o)
	if err := m.LoginWithKeys(sessionID, masterKey); err != nil {
		return nil, err
	}
	return &Client{m: m}, nil
}

// SessionID returns the opaque session identifier.
func (c *Client) SessionID() string {
	return c.m.GetSessionID()
}

// MasterKey returns the account master key.
func (c *Client) MasterKey() []byte {
	return c.m.GetMasterKey()
}

// SetWorkers sets the number of parallel transfer workers.
func (c *Client) SetWorkers(n int) error {
	if n <= 0 {
		return nil
	}
	if err := c.m.SetDownloadWorkers(n); err != nil {
		return err
	}
	return c.m.SetUploadWorkers(n)
}

// User returns account details.
func (c *Client) User() (mega.UserResp, error) {
	return c.m.GetUser()
}

// Quota returns storage usage.
func (c *Client) Quota() (mega.QuotaResp, error) {
	return c.m.GetQuota()
}

// SharedRoots returns the folders other users shared with the account.
func (c *Client) SharedRoots() []*mega.Node {
	return sortNodes(c.m.FS.GetSharedRoots())
}

// rootChildren returns the top-level nodes of a namespace.
func (c *Client) rootChildren(ns Namespace) ([]*mega.Node, error) {
	switch ns {
	case Shared:
		return c.m.FS.GetSharedRoots(), nil
	case Trash:
		return c.m.FS.GetChildren(c.m.FS.GetTrash())
	default:
		return c.m.FS.GetChildren(c.m.FS.GetRoot())
	}
}

// rootNode returns the node backing a namespace root, or ErrSharedRoot.
func (c *Client) rootNode(ns Namespace) (*mega.Node, error) {
	switch ns {
	case Shared:
		return nil, ErrSharedRoot
	case Trash:
		return c.m.FS.GetTrash(), nil
	default:
		return c.m.FS.GetRoot(), nil
	}
}

// Lookup resolves a path to its node.
func (c *Client) Lookup(p Path) (*mega.Node, error) {
	if p.IsRoot() {
		return c.rootNode(p.NS)
	}
	children, err := c.rootChildren(p.NS)
	if err != nil {
		return nil, err
	}
	var n *mega.Node
	for _, name := range p.Parts {
		n = findChild(children, name)
		if n == nil {
			return nil, &NotFoundError{Path: p}
		}
		if children, err = c.m.FS.GetChildren(n); err != nil {
			return nil, err
		}
	}
	return n, nil
}

// findChild returns the first node in nodes with the given name.
func findChild(nodes []*mega.Node, name string) *mega.Node {
	for _, n := range nodes {
		if n.GetName() == name {
			return n
		}
	}
	return nil
}

// Child returns the named child of a folder, or nil.
func (c *Client) Child(parent *mega.Node, name string) (*mega.Node, error) {
	children, err := c.m.FS.GetChildren(parent)
	if err != nil {
		return nil, err
	}
	return findChild(children, name), nil
}

// Children lists a folder's children sorted by name.
func (c *Client) Children(p Path) ([]*mega.Node, error) {
	if p.IsRoot() {
		nodes, err := c.rootChildren(p.NS)
		return sortNodes(nodes), err
	}
	n, err := c.Lookup(p)
	if err != nil {
		return nil, err
	}
	if !IsDir(n) {
		return nil, fmt.Errorf("%s: %w", p, ErrNotDir)
	}
	nodes, err := c.m.FS.GetChildren(n)
	return sortNodes(nodes), err
}

// sortNodes sorts nodes by name in place and returns them.
func sortNodes(nodes []*mega.Node) []*mega.Node {
	sort.SliceStable(nodes, func(i, j int) bool {
		return nodes[i].GetName() < nodes[j].GetName()
	})
	return nodes
}

// WalkFunc is called for every node visited by Walk.
type WalkFunc func(p Path, n *mega.Node) error

// Walk visits every node below p, depth first, excluding p itself.
func (c *Client) Walk(p Path, fn WalkFunc) error {
	children, err := c.Children(p)
	if err != nil {
		return err
	}
	for _, n := range children {
		child := p.Join(n.GetName())
		if err := fn(child, n); err != nil {
			return err
		}
		if IsDir(n) {
			if err := c.Walk(child, fn); err != nil {
				return err
			}
		}
	}
	return nil
}

// Mkdir creates a folder. With parents set, missing ancestors are created
// and an existing folder is not an error.
func (c *Client) Mkdir(p Path, parents bool) (*mega.Node, error) {
	if p.IsRoot() {
		if parents {
			return c.rootNode(p.NS)
		}
		return nil, fmt.Errorf("%s: already exists", p)
	}
	parent, err := c.Lookup(p.Dir())
	var nf *NotFoundError
	if errors.As(err, &nf) && parents {
		parent, err = c.Mkdir(p.Dir(), true)
	}
	if err != nil {
		return nil, err
	}
	if !IsDir(parent) {
		return nil, fmt.Errorf("%s: %w", p.Dir(), ErrNotDir)
	}
	existing, err := c.Child(parent, p.Base())
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if parents && IsDir(existing) {
			return existing, nil
		}
		return nil, fmt.Errorf("%s: already exists", p)
	}
	c.invalidate()
	return c.m.CreateDir(p.Base(), parent)
}

// Move moves src into the folder parent.
func (c *Client) Move(src, parent *mega.Node) error {
	c.invalidate()
	return c.m.Move(src, parent)
}

// Rename renames a node in place.
func (c *Client) Rename(n *mega.Node, name string) error {
	return c.m.Rename(n, name)
}

// Delete moves a node to the trash, or destroys it when permanent is set.
func (c *Client) Delete(n *mega.Node, permanent bool) error {
	c.invalidate()
	return c.m.Delete(n, permanent)
}

// Link exports a public link for a node.
func (c *Client) Link(n *mega.Node, includeKey bool) (string, error) {
	return c.m.Link(n, includeKey)
}

// Size returns a file's size, or the total size of every file below a
// folder. It is computed from the in-memory tree, so it makes no API calls,
// and folder totals are cached until the next mutation.
func (c *Client) Size(n *mega.Node) int64 {
	if !IsDir(n) {
		return n.GetSize()
	}
	h := n.GetHash()
	if s, ok := c.sizes[h]; ok {
		return s
	}
	children, _ := c.m.FS.GetChildren(n)
	var total int64
	for _, child := range children {
		total += c.Size(child)
	}
	if c.sizes == nil {
		c.sizes = make(map[string]int64)
	}
	c.sizes[h] = total
	return total
}

// invalidate drops cached folder sizes after the tree changes.
func (c *Client) invalidate() {
	c.sizes = nil
}

// IsDir reports whether a node can contain children.
func IsDir(n *mega.Node) bool {
	switch n.GetType() {
	case mega.FOLDER, mega.ROOT, mega.INBOX, mega.TRASH:
		return true
	default:
		return false
	}
}
