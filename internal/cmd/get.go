package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	mega "github.com/t3rm1n4l/go-mega"

	"github.com/manzanit0/mega-cli/internal/remote"
	"github.com/manzanit0/mega-cli/internal/ui"
)

// transferStats tallies a batch of transfers.
type transferStats struct {
	// Files is the number of files transferred.
	Files int `json:"files"`
	// Skipped is the number of files left untouched.
	Skipped int `json:"skipped"`
	// Bytes is the number of bytes transferred.
	Bytes int64 `json:"bytes"`
}

// report prints the batch summary.
func (s transferStats) report(verb string) error {
	if flags.json {
		return ui.JSON(out, s)
	}
	msg := fmt.Sprintf("%s %d file(s), %s", verb, s.Files, ui.Bytes(s.Bytes))
	if s.Skipped > 0 {
		msg += fmt.Sprintf(", skipped %d", s.Skipped)
	}
	logf("%s", msg)
	return nil
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

// getter downloads remote nodes to the local filesystem.
type getter struct {
	// c is the MEGA client.
	c *remote.Client
	// skipExisting skips files already present with the same size.
	skipExisting bool
	// stats accumulates results.
	stats transferStats
}

// file downloads a single remote file to dst.
func (g *getter) file(p remote.Path, n *mega.Node, dst string) error {
	if g.skipExisting && sameSize(dst, n.GetSize()) {
		g.stats.Skipped++
		return nil
	}
	pb := progress(p.String(), n.GetSize())
	err := g.c.Download(n, dst, pb.Add)
	pb.Done()
	if err != nil {
		return fmt.Errorf("%s: %w", p, err)
	}
	logf("%s -> %s", p, dst)
	g.stats.Files++
	g.stats.Bytes += n.GetSize()
	return nil
}

// tree downloads the folder at p into the local directory dst.
func (g *getter) tree(p remote.Path, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	return g.c.Walk(p, func(cp remote.Path, n *mega.Node) error {
		rel := filepath.Join(cp.Parts[len(p.Parts):]...)
		target := filepath.Join(dst, rel)
		if remote.IsDir(n) {
			return os.MkdirAll(target, 0o755)
		}
		return g.file(cp, n, target)
	})
}

// newGetCmd builds `mega get`.
func newGetCmd() *cobra.Command {
	var skip bool
	cmd := &cobra.Command{
		Use:     "get <remote> [local]",
		Aliases: []string{"download"},
		Short:   "Download files or folders",
		Long: `Download a file or folder. Folders are downloaded recursively.

If local is an existing directory the item is placed inside it, otherwise
local is the target name. Use "-" as local to write a file to stdout.
Files are written atomically and verified against MEGA's MAC.`,
		Example: `  mega get /docs/report.pdf
  mega get /photos ~/Pictures/
  mega get /backup/db.sql.gz - | gunzip | psql`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(_ *cobra.Command, args []string) error {
			local := "."
			if len(args) == 2 {
				local = args[1]
			}
			if local == "-" {
				return runCat(args[:1])
			}
			c, err := connect()
			if err != nil {
				return err
			}
			g := &getter{c: c, skipExisting: skip}
			if err := g.run(remote.Parse(args[0]), local); err != nil {
				return err
			}
			return g.stats.report("Downloaded")
		},
	}
	cmd.Flags().BoolVar(&skip, "skip-existing", false,
		"skip files that exist locally with the same size")
	return cmd
}

// run resolves the destination and downloads p.
func (g *getter) run(p remote.Path, local string) error {
	n, err := g.c.Lookup(p)
	if errors.Is(err, remote.ErrSharedRoot) {
		return fmt.Errorf("pick a specific shared folder, see `mega shares`")
	}
	if err != nil {
		return err
	}
	name := n.GetName()
	if p.IsRoot() {
		name = "MEGA"
	}
	dst := local
	if isLocalDir(local) {
		dst = filepath.Join(local, name)
	}
	if remote.IsDir(n) {
		return g.tree(p, dst)
	}
	return g.file(p, n, dst)
}

// newCatCmd builds `mega cat`.
func newCatCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cat <remote>...",
		Short: "Print file contents to stdout",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runCat(args)
		},
	}
}

// runCat streams each remote file to stdout.
func runCat(args []string) error {
	c, err := connect()
	if err != nil {
		return err
	}
	for _, a := range args {
		p := remote.Parse(a)
		n, err := c.Lookup(p)
		if err != nil {
			return err
		}
		if remote.IsDir(n) {
			return &fs.PathError{Op: "cat", Path: p.String(), Err: errors.New("is a folder")}
		}
		if err := c.Stream(n, os.Stdout, nil); err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
	}
	return nil
}
