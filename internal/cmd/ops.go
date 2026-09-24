package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	mega "github.com/t3rm1n4l/go-mega"

	"github.com/manzanit0/mega-cli/internal/remote"
	"github.com/manzanit0/mega-cli/internal/ui"
)

// newMkdirCmd builds `mega mkdir`.
func newMkdirCmd() *cobra.Command {
	var parents bool
	cmd := &cobra.Command{
		Use:   "mkdir <path>...",
		Short: "Create folders",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := connect()
			if err != nil {
				return err
			}
			for _, p := range pathArgs(args) {
				if _, err := c.Mkdir(p, parents); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&parents, "parents", "p", false,
		"create missing parents; no error if the folder exists")
	return cmd
}

// newMvCmd builds `mega mv`.
func newMvCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "mv <src>... <dst>",
		Aliases: []string{"move", "rename"},
		Short:   "Move or rename files and folders",
		Long: `Move or rename files and folders.

If dst is an existing folder, sources are moved into it. Otherwise a single
source is moved and renamed to dst. Restore from the trash with
"mega mv trash:/file /".`,
		Example: `  mega mv /a.txt /b.txt
  mega mv /a.txt /b.txt /archive/
  mega mv trash:/report.pdf /docs`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := connect()
			if err != nil {
				return err
			}
			ps := pathArgs(args)
			return runMv(c, ps[:len(ps)-1], ps[len(ps)-1])
		},
	}
}

// runMv moves each source into or onto dst.
func runMv(c *remote.Client, srcs []remote.Path, dst remote.Path) error {
	d, err := c.Lookup(dst)
	if err != nil && !isNotFound(err) {
		return err
	}
	if d != nil && remote.IsDir(d) {
		for _, s := range srcs {
			if err := moveTo(c, s, d, dst.Join(s.Base())); err != nil {
				return err
			}
		}
		return nil
	}
	if d != nil {
		return fmt.Errorf("%s: already exists", dst)
	}
	if len(srcs) > 1 {
		return fmt.Errorf("%s: no such folder", dst)
	}
	parent, err := c.Lookup(dst.Dir())
	if err != nil {
		return err
	}
	return moveTo(c, srcs[0], parent, dst)
}

// moveTo moves src into the folder parent under target's name.
func moveTo(c *remote.Client, src remote.Path, parent *mega.Node, target remote.Path) error {
	if src.IsRoot() {
		return fmt.Errorf("cannot move a root folder")
	}
	n, err := c.Lookup(src)
	if err != nil {
		return err
	}
	if !remote.IsDir(parent) {
		return fmt.Errorf("%s: %w", target.Dir(), remote.ErrNotDir)
	}
	clash, err := c.Child(parent, target.Base())
	if err != nil {
		return err
	}
	if clash != nil && clash != n {
		return fmt.Errorf("%s: already exists", target)
	}
	if src.Dir().String() != target.Dir().String() {
		if err := c.Move(n, parent); err != nil {
			return err
		}
	}
	if src.Base() != target.Base() {
		if err := c.Rename(n, target.Base()); err != nil {
			return err
		}
	}
	logf("%s -> %s", src, target)
	return nil
}

// isNotFound reports whether err is a remote.NotFoundError.
func isNotFound(err error) bool {
	var nf *remote.NotFoundError
	return errors.As(err, &nf)
}

// newRmCmd builds `mega rm`.
func newRmCmd() *cobra.Command {
	var recursive, permanent, force bool
	cmd := &cobra.Command{
		Use:     "rm <path>...",
		Aliases: []string{"delete"},
		Short:   "Move files and folders to the trash",
		Long: `Move files and folders to the trash. Folders require -r.
With --permanent, items are destroyed immediately and cannot be recovered.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := connect()
			if err != nil {
				return err
			}
			for _, p := range pathArgs(args) {
				if err := remove(c, p, recursive, permanent, force); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "remove folders")
	cmd.Flags().BoolVar(&permanent, "permanent", false, "destroy instead of trashing")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "ignore missing paths")
	return cmd
}

// remove deletes a single path according to the rm flags.
func remove(c *remote.Client, p remote.Path, recursive, permanent, force bool) error {
	if p.IsRoot() {
		return fmt.Errorf("refusing to remove %s", p)
	}
	n, err := c.Lookup(p)
	if force && isNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if remote.IsDir(n) && !recursive {
		return fmt.Errorf("%s: is a folder (use -r)", p)
	}
	if p.NS == remote.Trash {
		permanent = true
	}
	if err := c.Delete(n, permanent); err != nil {
		return fmt.Errorf("%s: %w", p, err)
	}
	if permanent {
		logf("destroyed %s", p)
	} else {
		logf("trashed %s", p)
	}
	return nil
}

// publicLink is the JSON output of `mega link`.
type publicLink struct {
	// Path is the exported remote path.
	Path string `json:"path"`
	// URL is the public link.
	URL string `json:"url"`
}

// newLinkCmd builds `mega link`.
func newLinkCmd() *cobra.Command {
	var noKey bool
	cmd := &cobra.Command{
		Use:     "link <path>...",
		Aliases: []string{"share-link"},
		Short:   "Create public links",
		Long: `Create public links. By default the decryption key is included so
anyone with the link can open it; use --no-key to share the key separately.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := connect()
			if err != nil {
				return err
			}
			var links []publicLink
			for _, p := range pathArgs(args) {
				n, err := c.Lookup(p)
				if err != nil {
					return err
				}
				u, err := c.Link(n, !noKey)
				if err != nil {
					return fmt.Errorf("%s: %w", p, err)
				}
				links = append(links, publicLink{Path: p.String(), URL: u})
			}
			if flags.json {
				return ui.JSON(out, links)
			}
			for _, l := range links {
				fmt.Fprintln(out, l.URL)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&noKey, "no-key", false, "omit the decryption key")
	return cmd
}
