package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/manzanit0/mega-cli/internal/ui"
	"github.com/manzanit0/mega-cli/pkg/transfer"
)

// reportStats prints the batch summary.
func reportStats(s transfer.Stats, verb string) error {
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
			return runDownload(args[:1], local, skip)
		},
	}
	cmd.Flags().BoolVar(&skip, "skip-existing", false,
		"skip files that exist locally with the same size")
	return cmd
}

// runDownload downloads the MEGA paths in srcs to local and reports the
// result.
func runDownload(srcs []string, local string, skip bool) error {
	c, err := connect()
	if err != nil {
		return err
	}
	o := transfer.Options{
		SkipExisting: skip,
		Progress:     progressFactory,
		OnFile:       func(src, dst string) { logf("%s -> %s", src, dst) },
	}
	stats, err := transfer.Download(c, pathArgs(srcs), local, o)
	if err != nil {
		return err
	}
	return reportStats(stats, "Downloaded")
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
	for _, p := range pathArgs(args) {
		if err := c.Cat(p, os.Stdout, nil); err != nil {
			return err
		}
	}
	return nil
}
