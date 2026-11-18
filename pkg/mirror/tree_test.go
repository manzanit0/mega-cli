package mirror

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

// write creates a file with content below root, making parent dirs.
func write(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// keys returns the sorted keys of a tree.
func keys(tr Tree) []string {
	var ks []string
	for k := range tr {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// TestLocalTree checks scanning, excludes and symlink skipping.
func TestLocalTree(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a/one.txt", "1")
	write(t, root, "a/.DS_Store", "x")
	write(t, root, "cache/big.bin", "zz")
	write(t, root, "two.txt", "22")
	if err := os.Symlink("two.txt", filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	var skipped []string
	tr, err := LocalTree(root, []string{".DS_Store", "cache"}, func(p string) {
		skipped = append(skipped, filepath.Base(p))
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a", "a/one.txt", "two.txt"}
	if !reflect.DeepEqual(keys(tr), want) {
		t.Errorf("keys = %v, want %v", keys(tr), want)
	}
	if !tr["a"].Dir || tr["two.txt"].Size != 2 || tr["two.txt"].Path == "" {
		t.Errorf("bad entries: %+v", tr)
	}
	if !reflect.DeepEqual(skipped, []string{"link"}) {
		t.Errorf("skipped = %v", skipped)
	}
}

// TestExcluded checks patterns match any path component.
func TestExcluded(t *testing.T) {
	pats := []string{"*.tmp", "node_modules"}
	for rel, want := range map[string]bool{
		"a/b.tmp":             true,
		"x/node_modules/y.js": true,
		"src/main.go":         false,
		"tmp":                 false,
	} {
		if got := excluded(rel, pats); got != want {
			t.Errorf("excluded(%q) = %v, want %v", rel, got, want)
		}
	}
}
