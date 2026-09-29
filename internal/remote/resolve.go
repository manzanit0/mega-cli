package remote

import (
	"strings"

	mega "github.com/t3rm1n4l/go-mega"
)

// resolveParts matches parts against a tree whose node names may contain
// "/". At each level it tries the shortest name first and, if that does not
// lead to a full match, joins it with the following parts and retries. It
// returns the real names along the matched route and the final node.
func resolveParts[N any](level []N, parts []string, name func(N) string,
	children func(N) ([]N, bool)) ([]string, N, bool) {
	var zero N
	for k := 1; k <= len(parts); k++ {
		want := strings.Join(parts[:k], "/")
		for _, n := range level {
			if name(n) != want {
				continue
			}
			if k == len(parts) {
				return []string{want}, n, true
			}
			next, ok := children(n)
			if !ok {
				continue
			}
			if rest, found, ok := resolveParts(next, parts[k:], name, children); ok {
				return append([]string{want}, rest...), found, true
			}
		}
	}
	return nil, zero, false
}

// Resolve finds the node at p and returns p rewritten so that each part is
// a real node name. This lets "a/b/c" address a folder literally named
// "a/b" when no folder "a" leads to a match.
func (c *Client) Resolve(p Path) (Path, *mega.Node, error) {
	if p.IsRoot() {
		n, err := c.rootNode(p.NS)
		return p, n, err
	}
	level, err := c.rootChildren(p.NS)
	if err != nil {
		return p, nil, err
	}
	names, n, ok := resolveParts(level, p.Parts, (*mega.Node).GetName, c.dirChildren)
	if !ok {
		return p, nil, &NotFoundError{Path: p}
	}
	return Path{NS: p.NS, Parts: names}, n, nil
}

// dirChildren returns a folder's children, or false for files.
func (c *Client) dirChildren(n *mega.Node) ([]*mega.Node, bool) {
	if !IsDir(n) {
		return nil, false
	}
	nodes, err := c.m.FS.GetChildren(n)
	return nodes, err == nil
}
