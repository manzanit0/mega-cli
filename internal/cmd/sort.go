package cmd

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/manzanit0/mega-cli/pkg/remote"
)

// addSortFlags registers --sort and --reverse on cmd.
func addSortFlags(cmd *cobra.Command, o *remote.Order) {
	cmd.Flags().StringVarP(&o.Key, "sort", "s", remote.SortName,
		"sort by "+strings.Join(remote.SortKeys, ", ")+" (size and time: largest/newest first)")
	cmd.Flags().BoolVarP(&o.Reverse, "reverse", "r", false, "reverse the sort order")
	_ = cmd.RegisterFlagCompletionFunc("sort",
		func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
			return remote.SortKeys, cobra.ShellCompDirectiveNoFileComp
		})
}
