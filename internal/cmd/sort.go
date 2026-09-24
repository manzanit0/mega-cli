package cmd

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"
)

// sortKeys lists the accepted --sort values.
var sortKeys = []string{"name", "size", "time"}

// sortOrder is how listings are ordered.
type sortOrder struct {
	// key is one of sortKeys.
	key string
	// reverse flips the order.
	reverse bool
}

// addSortFlags registers --sort and --reverse on cmd.
func addSortFlags(cmd *cobra.Command, o *sortOrder) {
	cmd.Flags().StringVarP(&o.key, "sort", "s", "name",
		"sort by "+strings.Join(sortKeys, ", ")+" (size and time: largest/newest first)")
	cmd.Flags().BoolVarP(&o.reverse, "reverse", "r", false, "reverse the sort order")
	_ = cmd.RegisterFlagCompletionFunc("sort",
		func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
			return sortKeys, cobra.ShellCompDirectiveNoFileComp
		})
}

// validate checks the sort key.
func (o sortOrder) validate() error {
	if slices.Contains(sortKeys, o.key) {
		return nil
	}
	return fmt.Errorf("--sort must be one of %s", strings.Join(sortKeys, ", "))
}

// compare orders a against b. Ties fall back to name order.
func (o sortOrder) compare(a, b Entry) int {
	c := 0
	switch o.key {
	case "size":
		c = cmp.Compare(b.Size, a.Size)
	case "time":
		c = b.Modified.Compare(a.Modified)
	}
	if c == 0 {
		c = strings.Compare(a.Name, b.Name)
	}
	if o.reverse {
		return -c
	}
	return c
}

// sortEntries orders entries in place.
func (o sortOrder) sortEntries(entries []Entry) {
	slices.SortStableFunc(entries, o.compare)
}
