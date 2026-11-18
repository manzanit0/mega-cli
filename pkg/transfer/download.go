package transfer

import (
	"fmt"
	"os"
	"path/filepath"

	mega "github.com/t3rm1n4l/go-mega"

	"github.com/manzanit0/mega-cli/pkg/remote"
)

// downloader downloads remote nodes to the local filesystem.
type downloader struct {
	// c is the MEGA client.
	c *remote.Client
	// o holds the transfer options.
	o Options
	// stats accumulates results.
	stats Stats
}

// Download copies the files and folders at srcs to the local path dst.
// Folders are downloaded recursively and files are written atomically, then
// verified against MEGA's MAC. If dst is an existing directory each source
// is placed inside it, otherwise dst is the target name of a single source.
// Stats cover what was transferred even when an error is returned.
func Download(c *remote.Client, srcs []remote.Path, dst string, o Options) (Stats, error) {
	d := &downloader{c: c, o: o}
	for _, s := range srcs {
		if err := d.run(s, dst); err != nil {
			return d.stats, err
		}
	}
	return d.stats, nil
}

// isLocalDir reports whether p exists locally and is a directory.
func isLocalDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// sameSize reports whether a local file exists with the given size.
func sameSize(p string, size int64) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular() && fi.Size() == size
}

// file downloads a single remote file to dst.
func (d *downloader) file(p remote.Path, n *mega.Node, dst string) error {
	if d.o.SkipExisting && sameSize(dst, n.GetSize()) {
		d.stats.Skipped++
		return nil
	}
	pf, done := d.o.Progress.Start(p.String(), n.GetSize())
	err := d.c.Download(n, dst, pf)
	done()
	if err != nil {
		return fmt.Errorf("%s: %w", p, err)
	}
	d.o.notify(p.String(), dst)
	d.stats.Files++
	d.stats.Bytes += n.GetSize()
	return nil
}

// tree downloads the folder at p into the local directory dst.
func (d *downloader) tree(p remote.Path, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	return d.c.Walk(p, func(cp remote.Path, n *mega.Node) error {
		target := dst
		for _, part := range cp.Parts[len(p.Parts):] {
			target = filepath.Join(target, remote.LocalName(part))
		}
		if remote.IsDir(n) {
			return os.MkdirAll(target, 0o755)
		}
		return d.file(cp, n, target)
	})
}

// run resolves the destination and downloads p.
func (d *downloader) run(p remote.Path, local string) error {
	p, n, err := d.c.Resolve(p)
	if err != nil {
		return err
	}
	name := remote.LocalName(n.GetName())
	if p.IsRoot() {
		name = "MEGA"
	}
	dst := local
	if isLocalDir(local) {
		dst = filepath.Join(local, name)
	}
	if remote.IsDir(n) {
		return d.tree(p, dst)
	}
	return d.file(p, n, dst)
}
