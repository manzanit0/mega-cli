package cmd

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	mega "github.com/t3rm1n4l/go-mega"

	"github.com/manzanit0/mega-cli/internal/remote"
)

// putter uploads local files and directories.
type putter struct {
	// c is the MEGA client.
	c *remote.Client
	// force replaces existing remote files.
	force bool
	// skipExisting skips remote files with the same name and size.
	skipExisting bool
	// stats accumulates results.
	stats transferStats
}

// file uploads src as name into parent, located at remote path p.
func (u *putter) file(src string, parent *mega.Node, p remote.Path) error {
	fi, err := os.Stat(src)
	if err != nil {
		return err
	}
	if u.skipExisting {
		existing, err := u.c.Child(parent, p.Base())
		if err != nil {
			return err
		}
		if existing != nil && existing.GetSize() == fi.Size() {
			u.stats.Skipped++
			return nil
		}
	}
	replace := u.force || u.skipExisting
	pb := progress(p.String(), fi.Size())
	_, err = u.c.Upload(src, parent, p.Base(), replace, pb.Add)
	pb.Done()
	if err != nil {
		return fmt.Errorf("%s: %w", src, err)
	}
	logf("%s -> %s", src, p)
	u.stats.Files++
	u.stats.Bytes += fi.Size()
	return nil
}

// tree uploads the local directory src as the remote folder p.
func (u *putter) tree(src string, p remote.Path) error {
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
			logf("skipping non-regular file %s", lp)
			return nil
		}
		parent, err := u.c.Lookup(target.Dir())
		if err != nil {
			return err
		}
		return u.file(lp, parent, target)
	})
}

// newPutCmd builds `mega put`.
func newPutCmd() *cobra.Command {
	var force, skip bool
	cmd := &cobra.Command{
		Use:     "put <local>... <remote>",
		Aliases: []string{"upload"},
		Short:   "Upload files or folders",
		Long: `Upload files or folders. Directories are uploaded recursively and
missing remote parent folders are created.

If remote is an existing folder, or ends in "/", items go inside it;
otherwise a single source is uploaded under that name. Use "-" as the
local source to upload stdin. Existing files are not overwritten unless
--force is given; the replaced file is moved to the trash.`,
		Example: `  mega put report.pdf /docs/
  mega put -j 8 ./photos /backup/photos
  pg_dump db | gzip | mega put - /backup/db.sql.gz`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := connect()
			if err != nil {
				return err
			}
			u := &putter{c: c, force: force, skipExisting: skip}
			srcs, dst := args[:len(args)-1], args[len(args)-1]
			if err := u.run(srcs, dst); err != nil {
				return err
			}
			return u.stats.report("Uploaded")
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false,
		"replace existing files (old versions go to the trash)")
	cmd.Flags().BoolVar(&skip, "skip-existing", false,
		"skip files that exist remotely with the same size")
	return cmd
}

// run resolves the destination and uploads every source.
func (u *putter) run(srcs []string, dst string) error {
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
func (u *putter) isInto(p remote.Path, raw string, n int) (bool, error) {
	node, err := u.c.Lookup(p)
	var nf *remote.NotFoundError
	switch {
	case errors.As(err, &nf):
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
	if src == "-" {
		return "stdin"
	}
	return filepath.Base(filepath.Clean(src))
}

// one uploads a single source argument to target.
func (u *putter) one(src string, target remote.Path) error {
	if target.IsRoot() {
		return fmt.Errorf("cannot upload over the root folder")
	}
	if src == "-" {
		return u.stdin(target)
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

// stdin spools standard input to a temp file and uploads it, since MEGA
// needs the size before an upload starts.
func (u *putter) stdin(target remote.Path) error {
	tmp, err := os.CreateTemp("", "mega-stdin-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	_, err = io.Copy(tmp, os.Stdin)
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
