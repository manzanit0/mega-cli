package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/manzanit0/mega-cli/internal/ui"
	"github.com/manzanit0/mega-cli/pkg/remote"
)

// newTrashCmd builds `mega trash` and its subcommands.
func newTrashCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trash",
		Short: "Inspect and empty the Rubbish Bin",
		Long: `Inspect and empty the Rubbish Bin. Trash paths use the "trash:"
prefix with every other command, e.g. "mega mv trash:/a.txt /".`,
	}
	cmd.AddCommand(newTrashLsCmd(), newTrashEmptyCmd())
	return cmd
}

// newTrashLsCmd builds `mega trash ls`.
func newTrashLsCmd() *cobra.Command {
	var long bool
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List items in the trash",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			c, err := connect()
			if err != nil {
				return err
			}
			entries, err := c.List(remote.Parse("trash:"), false)
			if err != nil {
				return err
			}
			if flags.json {
				return ui.JSON(out, nonNil(entries))
			}
			return printEntries(out, entries, long, false)
		},
	}
	cmd.Flags().BoolVarP(&long, "long", "l", false, "show type, size and date")
	return cmd
}

// newTrashEmptyCmd builds `mega trash empty`.
func newTrashEmptyCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "empty",
		Short: "Permanently delete everything in the trash",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			c, err := connect()
			if err != nil {
				return err
			}
			items, err := c.List(remote.Parse("trash:"), false)
			if err != nil {
				return err
			}
			if len(items) == 0 {
				logf("Trash is already empty")
				return nil
			}
			if !yes && !confirm(fmt.Sprintf("Permanently delete %d item(s)?", len(items))) {
				return fmt.Errorf("aborted")
			}
			n, err := c.EmptyTrash()
			if err != nil {
				return err
			}
			logf("Deleted %d item(s)", n)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "do not ask for confirmation")
	return cmd
}

// confirm asks a yes/no question, defaulting to no when not interactive.
func confirm(question string) bool {
	if !ui.IsTerminal(os.Stdin) {
		return false
	}
	answer, err := prompt(question + " [y/N] ")
	if err != nil {
		return false
	}
	answer = strings.ToLower(answer)
	return answer == "y" || answer == "yes"
}

// newSharesCmd builds `mega shares`.
func newSharesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "shares",
		Short: "List folders shared with you",
		Long: `List folders other users shared with you. Access their contents with
the "shared:" prefix, e.g. "mega ls shared:/Team".`,
		Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			c, err := connect()
			if err != nil {
				return err
			}
			entries, err := c.List(remote.Parse("shared:"), false)
			if err != nil {
				return err
			}
			if flags.json {
				return ui.JSON(out, nonNil(entries))
			}
			return printEntries(out, entries, false, true)
		},
	}
}
