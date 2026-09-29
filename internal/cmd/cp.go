package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/manzanit0/mega-cli/internal/remote"
)

// newCpCmd builds `mega cp`.
func newCpCmd() *cobra.Command {
	var force, skip bool
	cmd := &cobra.Command{
		Use:     "cp <source>... <destination>",
		Aliases: []string{"copy"},
		Short:   "Copy files and folders between disk and MEGA",
		Long: `Copy files and folders between disk and MEGA, like "aws s3 cp".

MEGA paths use the mega:// prefix. Either every source is local and the
destination is on MEGA (upload), or every source is on MEGA and the
destination is local (download). Folders are copied recursively.

If the destination is an existing folder, or ends in "/", sources go
inside it; otherwise a single source is copied under that name. Use "-"
for stdin (as source) or stdout (as destination).`,
		Example: `  mega cp report.pdf mega://docs/
  mega cp -j 8 ./photos mega://backup/photos
  mega cp mega://docs/report.pdf mega://docs/notes.md ~/Downloads/
  pg_dump db | gzip | mega cp - mega://backup/db.sql.gz
  mega cp mega://backup/db.sql.gz - | gunzip | psql`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			srcs, dst := args[:len(args)-1], args[len(args)-1]
			return runCp(srcs, dst, force, skip)
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false,
		"replace existing MEGA files when uploading (old versions go to the trash)")
	cmd.Flags().BoolVar(&skip, "skip-existing", false,
		"skip files that already exist at the destination with the same size")
	return cmd
}

// checkCpArgs ensures sources and destination are on opposite sides.
func checkCpArgs(srcs []string, dst string) error {
	up := remote.IsRemote(dst)
	for _, s := range srcs {
		if remote.IsRemote(s) == up {
			return fmt.Errorf("cp copies local -> %s or %s -> local; got %q -> %q",
				remote.Scheme, remote.Scheme, s, dst)
		}
	}
	if !up && len(srcs) > 1 && dst != "-" && !isLocalDir(dst) {
		return fmt.Errorf("%s: not a directory", dst)
	}
	return nil
}

// runCp validates the arguments and uploads or downloads accordingly.
func runCp(srcs []string, dst string, force, skip bool) error {
	if err := checkCpArgs(srcs, dst); err != nil {
		return err
	}
	if dst == "-" {
		return runCat(srcs)
	}
	c, err := connect()
	if err != nil {
		return err
	}
	if remote.IsRemote(dst) {
		u := &putter{c: c, force: force, skipExisting: skip}
		if err := u.run(srcs, dst); err != nil {
			return err
		}
		return u.stats.report("Uploaded")
	}
	g := &getter{c: c, skipExisting: skip}
	for _, s := range srcs {
		if err := g.run(remote.Parse(s), dst); err != nil {
			return err
		}
	}
	return g.stats.report("Downloaded")
}
