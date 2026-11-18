// Package cmd implements the mega command-line interface.
package cmd

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/spf13/cobra"
	mega "github.com/t3rm1n4l/go-mega"

	"github.com/manzanit0/mega-cli/internal/ui"
	"github.com/manzanit0/mega-cli/pkg/auth"
	"github.com/manzanit0/mega-cli/pkg/remote"
)

// Version is the CLI version, set at build time via -ldflags.
var Version = "dev"

// globals holds flags shared by every command.
type globals struct {
	// json switches output to machine-readable JSON.
	json bool
	// quiet suppresses progress output.
	quiet bool
	// debug enables go-mega debug logging.
	debug bool
	// workers is the number of parallel transfer workers.
	workers int
}

// flags is the parsed set of global flags.
var flags globals

// NewRoot builds the root command.
func NewRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "mega",
		Short: "A developer-friendly CLI for MEGA cloud storage",
		Long: `A developer-friendly CLI for MEGA cloud storage.

Remote paths are absolute from the Cloud Drive root ("docs/a.txt",
"/docs/a.txt" and "mega://docs/a.txt" are the same). The mega:// prefix is
required by cp and sync, where it tells MEGA paths from local ones. Use
"trash:" for the Rubbish Bin or "shared:" for folders shared with you,
e.g. "trash:/old.txt".

Credentials: run "mega login" once, or set MEGA_EMAIL and MEGA_PASSWORD
(and optionally MEGA_MFA) for non-interactive use.`,
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	pf := root.PersistentFlags()
	pf.BoolVar(&flags.json, "json", false, "output JSON")
	pf.BoolVarP(&flags.quiet, "quiet", "q", false, "suppress progress output")
	pf.BoolVar(&flags.debug, "debug", false, "log MEGA API calls to stderr")
	pf.IntVarP(&flags.workers, "jobs", "j", 0, "parallel transfer workers (max 30)")

	root.AddCommand(
		newLoginCmd(), newLogoutCmd(), newWhoamiCmd(), newQuotaCmd(),
		newLsCmd(), newTreeCmd(), newFindCmd(), newStatCmd(), newDuCmd(),
		newCpCmd(), newGetCmd(), newCatCmd(), newPutCmd(),
		newMkdirCmd(), newMvCmd(), newRmCmd(), newLinkCmd(),
		newSyncCmd(), newTrashCmd(), newSharesCmd(),
	)
	return root
}

// Execute runs the CLI and returns the process exit code.
func Execute() int {
	if err := NewRoot().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "mega:", friendly(err))
		return 1
	}
	return 0
}

// friendly rewrites terse go-mega errors into actionable messages.
func friendly(err error) error {
	switch {
	case errors.Is(err, mega.ESID):
		return errors.New("session expired or revoked: run `mega login`")
	case errors.Is(err, remote.ErrSharedRoot):
		return errors.New("pick a specific shared folder, see `mega shares`")
	case errors.Is(err, mega.EOVERQUOTA), errors.Is(err, mega.EGOINGOVERQUOTA):
		return fmt.Errorf("%w: storage or transfer quota exceeded", err)
	}
	return err
}

// options returns remote client options derived from global flags.
func options() remote.Options {
	o := remote.Options{}
	if flags.debug {
		o.Debugf = log.New(os.Stderr, "mega-debug: ", log.LstdFlags).Printf
	}
	return o
}

// connect returns a logged-in client from the stored session or from
// MEGA_EMAIL and MEGA_PASSWORD.
func connect() (*remote.Client, error) {
	c, err := auth.Connect(options())
	if err != nil {
		return nil, err
	}
	return c, c.SetWorkers(flags.workers)
}

// progress returns a progress indicator for a transfer when appropriate.
func progress(label string, total int64) *ui.Progress {
	enabled := !flags.quiet && !flags.json && ui.IsTerminal(os.Stderr)
	return ui.NewProgress(os.Stderr, enabled, label, total)
}

// progressFactory adapts progress to the library's progress hook.
func progressFactory(label string, total int64) (remote.ProgressFunc, func()) {
	pb := progress(label, total)
	return pb.Add, pb.Done
}

// out is where command results are written.
var out io.Writer = os.Stdout

// logf prints an informational message to stderr unless quiet.
func logf(format string, a ...any) {
	if flags.quiet || flags.json {
		return
	}
	fmt.Fprintf(os.Stderr, format+"\n", a...)
}
