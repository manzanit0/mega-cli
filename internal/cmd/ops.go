package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/manzanit0/mega-cli/internal/ui"
	"github.com/manzanit0/mega-cli/pkg/remote"
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
	return c.MovePaths(srcs, dst, func(src, dst remote.Path) {
		logf("%s -> %s", src, dst)
	})
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
				o := remote.RemoveOptions{Recursive: recursive, Permanent: permanent, IgnoreMissing: force}
				if err := remove(c, p, o); err != nil {
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
func remove(c *remote.Client, p remote.Path, o remote.RemoveOptions) error {
	r, err := c.Remove(p, o)
	if errors.Is(err, remote.ErrIsDir) {
		return fmt.Errorf("%w (use -r)", err)
	}
	switch {
	case err != nil:
		return err
	case r.Destroyed:
		logf("destroyed %s", p)
	case r.Removed:
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
				u, err := c.PublicLink(p, !noKey)
				if err != nil {
					return err
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
