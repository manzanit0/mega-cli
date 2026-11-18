package transfer

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	mega "github.com/t3rm1n4l/go-mega"

	"github.com/manzanit0/mega-cli/pkg/remote"
)

// uploader uploads local files and directories.
type uploader struct {
	// c is the MEGA client.
	c *remote.Client
	// o holds the transfer options.
	o Options
	// stats accumulates results.
	stats Stats
}

// Upload copies local files and folders to MEGA. Folders are uploaded
// recursively and missing remote parents are created. If dst is an existing
// folder, or ends in "/", sources go inside it; otherwise a single source
// is uploaded under that name. A source of StdinName reads Options.Stdin.
// Stats cover what was transferred even when an error is returned.
func Upload(c *remote.Client, srcs []string, dst string, o Options) (Stats, error) {
	u := &uploader{c: c, o: o}
	err := u.run(srcs, dst)
	return u.stats, err
}

// UploadReader uploads the contents of r as the remote file dst, creating
// missing parents. The data is spooled to a temporary file first, since
// MEGA needs the size before an upload starts.
func UploadReader(c *remote.Client, r io.Reader, dst remote.Path, o Options) (Stats, error) {
	u := &uploader{c: c, o: o}
	err := u.reader(r, dst)
	return u.stats, err
}

// file uploads src as name into parent, located at remote path p.
func (u *uploader) file(src string, parent *mega.Node, p remote.Path) error {
	fi, err := os.Stat(src)
	if err != nil {
		return err
	}
	if u.o.SkipExisting {
		existing, err := u.c.Child(parent, p.Base())
		if err != nil {
			return err
		}
		if existing != nil && existing.GetSize() == fi.Size() {
			u.stats.Skipped++
			return nil
		}
	}
	replace := u.o.Force || u.o.SkipExisting
	pf, done := u.o.Progress.Start(p.String(), fi.Size())
	_, err = u.c.Upload(src, parent, p.Base(), replace, pf)
	done()
	if err != nil {
		return fmt.Errorf("%s: %w", src, err)
	}
	u.o.notify(src, p.String())
	u.stats.Files++
	u.stats.Bytes += fi.Size()
	return nil
}

// tree uploads the local directory src as the remote folder p.
func (u *uploader) tree(src string, p remote.Path) error {
	return filepath.WalkDir(src, func(lp string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, lp)
		if err != nil {
			return err
		}
		target := p
		if rel != "." {
			for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
				target = target.Join(part)
			}
		}
		if d.IsDir() {
			_, err := u.c.Mkdir(target, true)
			return err
		}
		if !d.Type().IsRegular() {
			u.o.skipped(lp)
			return nil
		}
		parent, err := u.c.Lookup(target.Dir())
		if err != nil {
			return err
		}
		return u.file(lp, parent, target)
	})
}

// run resolves the destination and uploads every source.
func (u *uploader) run(srcs []string, dst string) error {
	p := remote.Parse(dst)
	into, err := u.isInto(p, dst, len(srcs))
	if err != nil {
		return err
	}
	for _, src := range srcs {
		target := p
		if into {
			target = p.Join(sourceName(src))
		}
		if err := u.one(src, target); err != nil {
			return err
		}
	}
	return nil
}

// isInto decides whether sources are placed inside dst or become dst.
func (u *uploader) isInto(p remote.Path, raw string, n int) (bool, error) {
	node, err := u.c.Lookup(p)
	switch {
	case remote.IsNotFound(err):
		return n > 1 || strings.HasSuffix(raw, "/"), nil
	case err != nil:
		return false, err
	case remote.IsDir(node):
		return true, nil
	case n > 1:
		return false, fmt.Errorf("%s: %w", p, remote.ErrNotDir)
	}
	return false, nil
}

// sourceName returns the remote name for a local source.
func sourceName(src string) string {
	if src == StdinName {
		return "stdin"
	}
	return filepath.Base(filepath.Clean(src))
}

// one uploads a single source argument to target.
func (u *uploader) one(src string, target remote.Path) error {
	if target.IsRoot() {
		return errors.New("cannot upload over the root folder")
	}
	if src == StdinName {
		if u.o.Stdin == nil {
			return errors.New("no stdin available to upload")
		}
		return u.reader(u.o.Stdin, target)
	}
	fi, err := os.Stat(src)
	if err != nil {
		return err
	}
	if fi.IsDir() {
		return u.tree(src, target)
	}
	parent, err := u.c.Mkdir(target.Dir(), true)
	if err != nil {
		return err
	}
	return u.file(src, parent, target)
}

// reader spools r to a temp file and uploads it as target.
func (u *uploader) reader(r io.Reader, target remote.Path) error {
	if target.IsRoot() {
		return errors.New("cannot upload over the root folder")
	}
	tmp, err := os.CreateTemp("", "mega-stdin-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	_, err = io.Copy(tmp, r)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	parent, err := u.c.Mkdir(target.Dir(), true)
	if err != nil {
		return err
	}
	return u.file(tmp.Name(), parent, target)
}
