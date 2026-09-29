package mirror

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"

	mega "github.com/t3rm1n4l/go-mega"

	"github.com/manzanit0/mega-cli/internal/remote"
)

// Options controls a sync run.
type Options struct {
	// Delete removes destination items missing from the source.
	Delete bool
	// Permanent destroys remote items instead of trashing them (Up only).
	Permanent bool
	// DryRun plans without changing anything.
	DryRun bool
	// Exclude holds glob patterns matched against every path component.
	Exclude []string
	// OnAction is called before each action is applied, or for each
	// planned action in a dry run.
	OnAction func(Action)
	// OnSkip is called for local items that cannot be synced.
	OnSkip func(string)
	// Progress returns a byte counter and a finish callback for a transfer.
	Progress func(label string, total int64) (remote.ProgressFunc, func())
	// ConfirmLocalDelete is called by Down with the planned local deletes,
	// which are permanent, before anything changes. An error aborts.
	ConfirmLocalDelete func(deletes []Action) error
}

// Deletes returns the delete actions in plan.
func Deletes(plan []Action) []Action {
	var out []Action
	for _, a := range plan {
		if a.Op == Delete {
			out = append(out, a)
		}
	}
	return out
}

// Result summarises a sync run.
type Result struct {
	// Actions are the planned actions.
	Actions []Action `json:"actions"`
	// DryRun reports whether actions were only planned.
	DryRun bool `json:"dry_run"`
	// Files is the number of files copied or updated.
	Files int `json:"files"`
	// Bytes is the number of bytes transferred.
	Bytes int64 `json:"bytes"`
	// Folders is the number of folders created.
	Folders int `json:"folders"`
	// Deleted is the number of items deleted.
	Deleted int `json:"deleted"`
}

// record tallies an applied action.
func (r *Result) record(a Action) {
	switch a.Op {
	case Mkdir:
		r.Folders++
	case Copy, Update:
		r.Files++
		r.Bytes += a.Size
	case Delete:
		r.Deleted++
	}
}

// progress returns the configured progress callbacks, or no-ops.
func (o Options) progress(label string, total int64) (remote.ProgressFunc, func()) {
	if o.Progress == nil {
		return nil, func() {}
	}
	return o.Progress(label, total)
}

// apply runs plan through do, or only reports it in a dry run.
func apply(plan []Action, o Options, do func(Action) error) (Result, error) {
	r := Result{Actions: plan, DryRun: o.DryRun}
	for _, a := range plan {
		if o.OnAction != nil {
			o.OnAction(a)
		}
		if o.DryRun {
			continue
		}
		if err := do(a); err != nil {
			return r, fmt.Errorf("%s %s: %w", a.Op, a.Path, err)
		}
		r.record(a)
	}
	return r, nil
}

// Down makes the local directory dst match the MEGA folder src.
func Down(c *remote.Client, src remote.Path, dst string, o Options) (Result, error) {
	src, root, err := c.Resolve(src)
	if err != nil {
		return Result{}, err
	}
	if !remote.IsDir(root) {
		return Result{}, fmt.Errorf("%s: %w", src, remote.ErrNotDir)
	}
	srcTree, err := RemoteTree(c, src, o.Exclude)
	if err != nil {
		return Result{}, err
	}
	dstTree, err := localTreeOrEmpty(dst, o)
	if err != nil {
		return Result{}, err
	}
	plan, err := Plan(srcTree, dstTree, ChangedDown, o.Delete)
	if err != nil {
		return Result{}, err
	}
	if !o.DryRun {
		if err := confirmDeletes(plan, o); err != nil {
			return Result{Actions: plan}, err
		}
		if err := os.MkdirAll(dst, 0o755); err != nil {
			return Result{}, err
		}
	}
	return apply(plan, o, func(a Action) error {
		return downOne(c, a, srcTree[a.Path], dst, o)
	})
}

// confirmDeletes asks the caller to approve planned local deletes.
func confirmDeletes(plan []Action, o Options) error {
	dels := Deletes(plan)
	if len(dels) == 0 || o.ConfirmLocalDelete == nil {
		return nil
	}
	return o.ConfirmLocalDelete(dels)
}

// localTreeOrEmpty scans dst, treating a missing directory as empty.
func localTreeOrEmpty(dst string, o Options) (Tree, error) {
	fi, err := os.Stat(dst)
	if errors.Is(err, os.ErrNotExist) {
		return Tree{}, nil
	}
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("%s: not a directory", dst)
	}
	return LocalTree(dst, o.Exclude, o.OnSkip)
}

// downOne applies a single action to the local tree.
func downOne(c *remote.Client, a Action, s Entry, dst string, o Options) error {
	target := filepath.Join(dst, filepath.FromSlash(a.Path))
	switch a.Op {
	case Mkdir:
		return os.MkdirAll(target, 0o755)
	case Delete:
		return os.RemoveAll(target)
	}
	add, done := o.progress(a.Path, s.Size)
	err := c.Download(s.Node, target, add)
	done()
	if err != nil {
		return err
	}
	return os.Chtimes(target, s.MTime, s.MTime)
}

// Up makes the MEGA folder dst match the local directory src.
func Up(c *remote.Client, src string, dst remote.Path, o Options) (Result, error) {
	fi, err := os.Stat(src)
	if err != nil {
		return Result{}, err
	}
	if !fi.IsDir() {
		return Result{}, fmt.Errorf("%s: not a directory", src)
	}
	srcTree, err := LocalTree(src, o.Exclude, o.OnSkip)
	if err != nil {
		return Result{}, err
	}
	u, err := newUploader(c, dst, o)
	if err != nil {
		return Result{}, err
	}
	plan, err := Plan(srcTree, u.dst, ChangedUp, o.Delete)
	if err != nil {
		return Result{}, err
	}
	if !o.DryRun {
		if err := u.ensureRoot(); err != nil {
			return Result{}, err
		}
	}
	return apply(plan, o, func(a Action) error {
		return u.apply(a, srcTree[a.Path])
	})
}

// uploader applies actions to a MEGA folder.
type uploader struct {
	// c is the MEGA client.
	c *remote.Client
	// root is the destination folder path.
	root remote.Path
	// dst is the current destination tree.
	dst Tree
	// folders maps relative folder paths to their nodes; "" is the root.
	folders map[string]*mega.Node
	// o holds the sync options.
	o Options
}

// newUploader scans the destination, which may not exist yet.
func newUploader(c *remote.Client, dst remote.Path, o Options) (*uploader, error) {
	u := &uploader{c: c, root: dst, dst: Tree{}, folders: map[string]*mega.Node{}, o: o}
	p, n, err := c.Resolve(dst)
	var nf *remote.NotFoundError
	if errors.As(err, &nf) {
		return u, nil
	}
	if err != nil {
		return nil, err
	}
	if !remote.IsDir(n) {
		return nil, fmt.Errorf("%s: %w", dst, remote.ErrNotDir)
	}
	u.root, u.folders[""] = p, n
	if u.dst, err = RemoteTree(c, p, o.Exclude); err != nil {
		return nil, err
	}
	for rel, e := range u.dst {
		if e.Dir {
			u.folders[rel] = e.Node
		}
	}
	return u, nil
}

// ensureRoot creates the destination folder if it does not exist.
func (u *uploader) ensureRoot() error {
	if u.folders[""] != nil {
		return nil
	}
	n, err := u.c.Mkdir(u.root, true)
	u.folders[""] = n
	return err
}

// parent returns the destination folder that will contain rel.
func (u *uploader) parent(rel string) (*mega.Node, error) {
	dir := path.Dir(rel)
	if dir == "." {
		dir = ""
	}
	n := u.folders[dir]
	if n == nil {
		return nil, fmt.Errorf("parent folder %q missing", dir)
	}
	return n, nil
}

// apply performs one action against MEGA.
func (u *uploader) apply(a Action, s Entry) error {
	if a.Op == Delete {
		return u.c.Delete(u.dst[a.Path].Node, u.o.Permanent)
	}
	parent, err := u.parent(a.Path)
	if err != nil {
		return err
	}
	name := path.Base(a.Path)
	if a.Op == Mkdir {
		n, err := u.c.MkdirIn(parent, name)
		u.folders[a.Path] = n
		return err
	}
	if old, ok := u.dst[a.Path]; ok && a.Op == Update {
		name = old.Node.GetName()
	}
	add, done := u.o.progress(a.Path, s.Size)
	_, err = u.c.Upload(s.Path, parent, name, a.Op == Update, add)
	done()
	return err
}
