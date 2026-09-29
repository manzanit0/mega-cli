package mirror

import (
	"io/fs"
	"path"
	"path/filepath"
	"strings"

	mega "github.com/t3rm1n4l/go-mega"

	"github.com/manzanit0/mega-cli/internal/remote"
)

// excluded reports whether any component of rel matches a glob pattern.
func excluded(rel string, patterns []string) bool {
	for _, part := range strings.Split(rel, "/") {
		for _, p := range patterns {
			if ok, _ := path.Match(p, part); ok {
				return true
			}
		}
	}
	return false
}

// RemoteTree lists everything below root. Keys use local-safe names, so
// they line up with a LocalTree of a downloaded copy.
func RemoteTree(c *remote.Client, root remote.Path, exclude []string) (Tree, error) {
	t := Tree{}
	err := c.Walk(root, func(p remote.Path, n *mega.Node) error {
		parts := p.Parts[len(root.Parts):]
		names := make([]string, len(parts))
		for i, part := range parts {
			names[i] = remote.LocalName(part)
		}
		rel := strings.Join(names, "/")
		if excluded(rel, exclude) {
			return nil
		}
		t[rel] = Entry{
			Dir:   remote.IsDir(n),
			Size:  n.GetSize(),
			MTime: n.GetTimeStamp(),
			Node:  n,
		}
		return nil
	})
	return t, err
}

// LocalTree lists everything below root. Symlinks and other non-regular
// files are reported through skip and left out.
func LocalTree(root string, exclude []string, skip func(string)) (Tree, error) {
	t := Tree{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil || rel == "." {
			return err
		}
		rel = filepath.ToSlash(rel)
		if excluded(rel, exclude) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		return addLocal(t, rel, p, d, skip)
	})
	return t, err
}

// addLocal records one local directory entry in t.
func addLocal(t Tree, rel, p string, d fs.DirEntry, skip func(string)) error {
	if d.IsDir() {
		t[rel] = Entry{Dir: true, Path: p}
		return nil
	}
	if !d.Type().IsRegular() {
		if skip != nil {
			skip(p)
		}
		return nil
	}
	info, err := d.Info()
	if err != nil {
		return err
	}
	t[rel] = Entry{Size: info.Size(), MTime: info.ModTime(), Path: p}
	return nil
}
