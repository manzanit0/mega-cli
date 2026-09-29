// Package mirror makes one folder tree match another, between MEGA and the
// local filesystem.
package mirror

import (
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	mega "github.com/t3rm1n4l/go-mega"
)

// Entry is a file or folder in a Tree.
type Entry struct {
	// Dir marks folders.
	Dir bool
	// Size is the file size in bytes.
	Size int64
	// MTime is the local modification time, or the remote node timestamp.
	MTime time.Time
	// Node is the remote node, for remote trees.
	Node *mega.Node
	// Path is the absolute local path, for local trees.
	Path string
}

// Tree maps slash-separated paths, relative to the tree root, to entries.
type Tree map[string]Entry

// Op is a kind of sync action.
type Op int

const (
	// Mkdir creates a folder.
	Mkdir Op = iota
	// Copy transfers a file missing from the destination.
	Copy
	// Update replaces a destination file that differs from the source.
	Update
	// Delete removes a destination item missing from the source.
	Delete
)

// String returns the op's name.
func (o Op) String() string {
	return [...]string{"mkdir", "copy", "update", "delete"}[o]
}

// MarshalText encodes the op by name in JSON output.
func (o Op) MarshalText() ([]byte, error) {
	return []byte(o.String()), nil
}

// Action is one step needed to make the destination match the source.
type Action struct {
	// Op is what to do.
	Op Op `json:"op"`
	// Path is the item's path relative to the sync roots.
	Path string `json:"path"`
	// Size is the number of bytes transferred, for copies and updates.
	Size int64 `json:"size,omitempty"`
}

// ChangedFunc reports whether a destination file differs from its source.
type ChangedFunc func(src, dst Entry) bool

// Plan computes the actions that make dst match src. Without del, items
// only present in dst are kept, and a file/folder type clash is an error.
func Plan(src, dst Tree, changed ChangedFunc, del bool) ([]Action, error) {
	var clashes, creates, removals []Action
	for _, k := range sortedKeys(src) {
		s := src[k]
		d, exists := dst[k]
		if exists && d.Dir != s.Dir {
			if !del {
				return nil, fmt.Errorf("%s: file/folder mismatch (use --delete to replace)", k)
			}
			clashes = append(clashes, Action{Op: Delete, Path: k})
			exists = false
		}
		if a, ok := createAction(k, s, d, exists, changed); ok {
			creates = append(creates, a)
		}
	}
	if del {
		removals = extras(src, dst)
	}
	return append(append(clashes, creates...), removals...), nil
}

// createAction returns the action needed for source entry s at key k.
func createAction(k string, s, d Entry, exists bool, changed ChangedFunc) (Action, bool) {
	switch {
	case s.Dir && !exists:
		return Action{Op: Mkdir, Path: k}, true
	case s.Dir:
		return Action{}, false
	case !exists:
		return Action{Op: Copy, Path: k, Size: s.Size}, true
	case changed(s, d):
		return Action{Op: Update, Path: k, Size: s.Size}, true
	}
	return Action{}, false
}

// extras returns deletes for dst items missing from src, skipping items
// whose parent folder is already being deleted.
func extras(src, dst Tree) []Action {
	var out []Action
	deleted := map[string]bool{}
	for _, k := range sortedKeys(dst) {
		if _, ok := src[k]; ok || underDeleted(k, deleted) {
			continue
		}
		deleted[k] = true
		out = append(out, Action{Op: Delete, Path: k})
	}
	return out
}

// underDeleted reports whether any ancestor of k is in deleted.
func underDeleted(k string, deleted map[string]bool) bool {
	for d := path.Dir(k); d != "." && d != "/"; d = path.Dir(d) {
		if deleted[d] {
			return true
		}
	}
	return false
}

// sortedKeys returns the tree's keys with parents before children.
func sortedKeys(t Tree) []string {
	keys := make([]string, 0, len(t))
	for k := range t {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return lessPath(keys[i], keys[j])
	})
	return keys
}

// lessPath orders paths component by component so that "a" and everything
// under it sort before "a b".
func lessPath(a, b string) bool {
	pa, pb := strings.Split(a, "/"), strings.Split(b, "/")
	for i := 0; i < len(pa) && i < len(pb); i++ {
		if pa[i] != pb[i] {
			return pa[i] < pb[i]
		}
	}
	return len(pa) < len(pb)
}

// ChangedDown detects changes when syncing MEGA to disk. Downloads stamp
// the local file with the node timestamp, so any difference means change.
func ChangedDown(src, dst Entry) bool {
	return src.Size != dst.Size || src.MTime.Unix() != dst.MTime.Unix()
}

// ChangedUp detects changes when syncing disk to MEGA. A node's timestamp
// is its upload time, so a local file modified after it has changed.
func ChangedUp(src, dst Entry) bool {
	return src.Size != dst.Size || src.MTime.Unix() > dst.MTime.Unix()
}
