package remote

import (
	"reflect"
	"testing"
)

// fakeNode is a minimal tree node for resolver tests.
type fakeNode struct {
	// name is the node name, which may contain "/".
	name string
	// kids are the node's children; nil marks a file.
	kids []*fakeNode
}

// dir builds a folder node.
func dir(name string, kids ...*fakeNode) *fakeNode {
	return &fakeNode{name: name, kids: append([]*fakeNode{}, kids...)}
}

// file builds a file node.
func file(name string) *fakeNode {
	return &fakeNode{name: name}
}

// resolveFake runs resolveParts over a fake tree.
func resolveFake(root []*fakeNode, p string) ([]string, *fakeNode, bool) {
	return resolveParts(root, Parse(p).Parts,
		func(n *fakeNode) string { return n.name },
		func(n *fakeNode) ([]*fakeNode, bool) { return n.kids, n.kids != nil })
}

// TestResolveSlashNames checks names containing "/" are found.
func TestResolveSlashNames(t *testing.T) {
	target := file("notes.txt")
	slash := dir("UC3M 2014/2015", target)
	root := []*fakeNode{dir("docs", file("a.txt")), slash, dir("a/b/c", file("x"))}

	cases := []struct {
		path  string
		names []string
		node  *fakeNode
	}{
		{"/UC3M 2014/2015", []string{"UC3M 2014/2015"}, slash},
		{"/UC3M 2014/2015/notes.txt", []string{"UC3M 2014/2015", "notes.txt"}, target},
		{"/docs/a.txt", []string{"docs", "a.txt"}, root[0].kids[0]},
		{"/a/b/c/x", []string{"a/b/c", "x"}, root[2].kids[0]},
	}
	for _, c := range cases {
		names, n, ok := resolveFake(root, c.path)
		if !ok || n != c.node || !reflect.DeepEqual(names, c.names) {
			t.Errorf("%s: got %v %v %v, want %v", c.path, names, n, ok, c.names)
		}
	}
	for _, missing := range []string{"/UC3M 2014", "/docs/b.txt", "/docs/a.txt/x", "/a/b"} {
		if _, _, ok := resolveFake(root, missing); ok {
			t.Errorf("%s: unexpectedly found", missing)
		}
	}
}

// TestResolvePrefersSplit checks that plain folders win over slash names,
// and that the resolver backtracks when the plain route is a dead end.
func TestResolvePrefersSplit(t *testing.T) {
	plain := file("2015")
	joined := dir("a/b", file("only-here"))
	root := []*fakeNode{dir("a", dir("b"), plain), joined}

	if names, n, _ := resolveFake(root, "/a/2015"); n != plain || names[0] != "a" {
		t.Errorf("plain route not preferred: %v", names)
	}
	names, n, ok := resolveFake(root, "/a/b/only-here")
	if !ok || n != joined.kids[0] || names[0] != "a/b" {
		t.Errorf("backtracking failed: %v %v", names, ok)
	}
}
