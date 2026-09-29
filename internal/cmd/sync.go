package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/manzanit0/mega-cli/internal/mirror"
	"github.com/manzanit0/mega-cli/internal/remote"
	"github.com/manzanit0/mega-cli/internal/ui"
)

// syncFlags holds flags shared by the sync subcommands.
type syncFlags struct {
	// del removes destination items missing from the source.
	del bool
	// dryRun only prints the plan.
	dryRun bool
	// exclude holds glob patterns to skip.
	exclude []string
	// permanent destroys remote items instead of trashing them.
	permanent bool
	// yes skips the confirmation before permanent local deletes.
	yes bool
}

// register adds the shared flags to cmd.
func (f *syncFlags) register(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&f.del, "delete", false,
		"delete destination items that are not in the source")
	cmd.Flags().BoolVarP(&f.dryRun, "dry-run", "n", false, "show what would change")
	cmd.Flags().StringSliceVarP(&f.exclude, "exclude", "x", nil,
		"skip names matching a glob, e.g. -x .DS_Store -x '*.tmp' (repeatable)")
	cmd.Flags().BoolVarP(&f.yes, "yes", "y", false,
		"do not ask before permanently deleting local files")
}

// options converts flags into mirror options wired to CLI output.
func (f *syncFlags) options() mirror.Options {
	return mirror.Options{
		Delete:    f.del,
		Permanent: f.permanent,
		DryRun:    f.dryRun,
		Exclude:   f.exclude,
		OnAction:  f.announce,
		OnSkip: func(p string) {
			logf("skipping non-regular file %s", p)
		},
		Progress: func(label string, total int64) (remote.ProgressFunc, func()) {
			pb := progress(label, total)
			return pb.Add, pb.Done
		},
		ConfirmLocalDelete: f.confirmLocalDelete,
	}
}

// maxListedDeletes caps how many paths the delete prompt lists.
const maxListedDeletes = 10

// confirmLocalDelete asks before permanently deleting local files. It only
// prompts on an interactive terminal and is skipped with --yes.
func (f *syncFlags) confirmLocalDelete(dels []mirror.Action) error {
	if f.yes || !ui.IsTerminal(os.Stdin) {
		return nil
	}
	for i, a := range dels {
		if i == maxListedDeletes {
			fmt.Fprintf(os.Stderr, "  ... and %d more\n", len(dels)-i)
			break
		}
		fmt.Fprintf(os.Stderr, "  %s\n", a.Path)
	}
	q := fmt.Sprintf("Permanently delete %d local item(s) listed above?", len(dels))
	if !confirm(q) {
		return fmt.Errorf("aborted; nothing changed (use --dry-run to review, --yes to skip)")
	}
	return nil
}

// announce prints an action: to stdout for a dry run, else as progress.
func (f *syncFlags) announce(a mirror.Action) {
	line := fmt.Sprintf("%-6s  %s", a.Op, a.Path)
	if a.Size > 0 {
		line += fmt.Sprintf("  (%s)", ui.Bytes(a.Size))
	}
	if f.dryRun && !flags.json {
		fmt.Fprintln(out, line)
		return
	}
	logf("%s", line)
}

// report prints the run summary.
func report(r mirror.Result) error {
	if flags.json {
		if r.Actions == nil {
			r.Actions = []mirror.Action{}
		}
		return ui.JSON(out, r)
	}
	if r.DryRun {
		logf("Dry run: %d change(s) planned, nothing modified", len(r.Actions))
		return nil
	}
	if len(r.Actions) == 0 {
		logf("Already in sync")
		return nil
	}
	logf("Synced: %d file(s) (%s), %d folder(s) created, %d deleted",
		r.Files, ui.Bytes(r.Bytes), r.Folders, r.Deleted)
	return nil
}

// location parses a sync or cp argument, reporting whether it is on MEGA.
// "mega://Photos" and "mega:///Photos" both mean the remote path /Photos.
func location(arg string) (remote.Path, bool) {
	if !remote.IsRemote(arg) {
		return remote.Path{}, false
	}
	return remote.Parse(arg), true
}

// newSyncCmd builds `mega sync`.
func newSyncCmd() *cobra.Command {
	var f syncFlags
	cmd := &cobra.Command{
		Use:   "sync <source> <destination>",
		Short: "Make a destination folder match a source folder",
		Long: `Make a destination folder match a source folder. Exactly one side must
be a MEGA path, written with the mega:// prefix; the other is local.

Only new and changed files are transferred, compared by size and time.
Downloaded files are stamped with MEGA's timestamp so later runs only fetch
changes; local files are uploaded when modified after their MEGA copy, and
replaced versions go to the MEGA trash.

With --delete, destination items missing from the source are removed:
permanently on disk, or to the MEGA trash (destroyed with --permanent).
Before permanent local deletes you are asked to confirm, unless --yes is
given or stdin is not a terminal. Always try --dry-run first.`,
		Example: `  mega sync -n --delete mega://Photos /Volumes/Backup/Photos
  mega sync --delete -x .DS_Store /Volumes/Backup/Photos mega://Photos
  mega sync -j 8 ~/Documents mega://Backups/Documents`,
		Args: cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			return runSync(&f, args[0], args[1])
		},
	}
	f.register(cmd)
	cmd.Flags().BoolVar(&f.permanent, "permanent", false,
		"with --delete, destroy MEGA items instead of trashing them")
	return cmd
}

// runSync validates the locations and runs the sync in their direction.
func runSync(f *syncFlags, srcArg, dstArg string) error {
	src, srcRemote := location(srcArg)
	dst, dstRemote := location(dstArg)
	switch {
	case srcRemote == dstRemote:
		return fmt.Errorf("exactly one of source and destination must be a %s path", remote.Scheme)
	case f.permanent && !f.del:
		return fmt.Errorf("--permanent requires --delete")
	case f.permanent && !dstRemote:
		return fmt.Errorf("--permanent only applies when the destination is on MEGA")
	}
	c, err := connect()
	if err != nil {
		return err
	}
	var r mirror.Result
	if srcRemote {
		r, err = mirror.Down(c, src, dstArg, f.options())
	} else {
		r, err = mirror.Up(c, srcArg, dst, f.options())
	}
	if err != nil {
		return err
	}
	return report(r)
}
