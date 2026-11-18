package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/manzanit0/mega-cli/pkg/transfer"
)

// uploadOptions builds transfer options wired to CLI output.
func uploadOptions(force, skip bool) transfer.Options {
	return transfer.Options{
		Force:        force,
		SkipExisting: skip,
		Progress:     progressFactory,
		Stdin:        os.Stdin,
		OnFile:       func(src, dst string) { logf("%s -> %s", src, dst) },
		OnSkip:       func(p string) { logf("skipping non-regular file %s", p) },
	}
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
			srcs, dst := args[:len(args)-1], args[len(args)-1]
			return runUpload(srcs, dst, force, skip)
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false,
		"replace existing files (old versions go to the trash)")
	cmd.Flags().BoolVar(&skip, "skip-existing", false,
		"skip files that exist remotely with the same size")
	return cmd
}

// runUpload uploads srcs to the MEGA path dst and reports the result.
func runUpload(srcs []string, dst string, force, skip bool) error {
	c, err := connect()
	if err != nil {
		return err
	}
	stats, err := transfer.Upload(c, srcs, dst, uploadOptions(force, skip))
	if err != nil {
		return err
	}
	return reportStats(stats, "Uploaded")
}
