package cmd

import (
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/manzanit0/mega-cli/internal/ui"
	"github.com/manzanit0/mega-cli/pkg/remote"
)

// pathArgs parses args, defaulting to the root when empty.
func pathArgs(args []string) []remote.Path {
	if len(args) == 0 {
		return []remote.Path{remote.Parse("/")}
	}
	ps := make([]remote.Path, len(args))
	for i, a := range args {
		ps[i] = remote.Parse(a)
	}
	return ps
}

// displayName returns the name to show, marking folders with a slash.
func displayName(e remote.Entry, full bool) string {
	name := e.Name
	if full {
		name = e.Path
	}
	if e.IsDir() {
		name += "/"
	}
	return name
}

// showSizes reports whether short listings include sizes. Sizes are shown
// only on a terminal so piped output stays one name per line.
var showSizes = func() bool {
	return ui.IsTerminal(os.Stdout)
}

// sizeOf returns the human-readable size of an entry.
func sizeOf(e remote.Entry) string {
	return ui.Bytes(e.Size)
}

// printEntries renders entries as names or a long table.
func printEntries(w io.Writer, entries []remote.Entry, long, full bool) error {
	sizes := showSizes()
	if !long && !sizes {
		for _, e := range entries {
			fmt.Fprintln(w, displayName(e, full))
		}
		return nil
	}
	rows := make([][]string, len(entries))
	for i, e := range entries {
		name := displayName(e, full)
		if long {
			rows[i] = []string{e.Type, sizeOf(e),
				e.Modified.Local().Format("2006-01-02 15:04"), name}
		} else {
			rows[i] = []string{name, sizeOf(e)}
		}
	}
	if long {
		return ui.Table(w, []ui.Align{ui.Left, ui.Right, ui.Left, ui.Left}, rows)
	}
	return ui.Table(w, []ui.Align{ui.Left, ui.Right}, rows)
}

// newLsCmd builds `mega ls`.
func newLsCmd() *cobra.Command {
	var long, recursive bool
	var order remote.Order
	cmd := &cobra.Command{
		Use:     "ls [path...]",
		Aliases: []string{"list"},
		Short:   "List folder contents",
		Example: `  mega ls -l --sort size /backup
  mega ls -R -s size / | head`,
		RunE: func(_ *cobra.Command, args []string) error {
			if err := order.Validate(); err != nil {
				return err
			}
			c, err := connect()
			if err != nil {
				return err
			}
			ps := pathArgs(args)
			var all []remote.Entry
			for i, p := range ps {
				entries, err := c.List(p, recursive)
				if err != nil {
					return err
				}
				order.Sort(entries)
				if flags.json {
					all = append(all, entries...)
					continue
				}
				if len(ps) > 1 {
					if i > 0 {
						fmt.Fprintln(out)
					}
					fmt.Fprintf(out, "%s:\n", p)
				}
				if err := printEntries(out, entries, long, recursive); err != nil {
					return err
				}
			}
			if flags.json {
				return ui.JSON(out, nonNil(all))
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&long, "long", "l", false, "show type, size and date")
	cmd.Flags().BoolVarP(&recursive, "recursive", "R", false, "list recursively")
	addSortFlags(cmd, &order)
	return cmd
}

// nonNil turns a nil slice into an empty one so JSON prints [].
func nonNil(e []remote.Entry) []remote.Entry {
	if e == nil {
		return []remote.Entry{}
	}
	return e
}

// newTreeCmd builds `mega tree`.
func newTreeCmd() *cobra.Command {
	var depth int
	var order remote.Order
	cmd := &cobra.Command{
		Use:     "tree [path]",
		Short:   "Show a folder as a tree",
		Example: `  mega tree -L 2 --sort size /projects`,
		Args:    cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if err := order.Validate(); err != nil {
				return err
			}
			c, err := connect()
			if err != nil {
				return err
			}
			p := pathArgs(args)[0]
			fmt.Fprintln(out, p)
			t := treePrinter{c: c, order: order}
			return t.print(p, "", depth)
		},
	}
	cmd.Flags().IntVarP(&depth, "depth", "L", 0, "max depth (0 = unlimited)")
	addSortFlags(cmd, &order)
	return cmd
}

// treePrinter renders folders as trees.
type treePrinter struct {
	// c is the MEGA client.
	c *remote.Client
	// order sorts siblings at every level.
	order remote.Order
}

// print recursively prints the children of p with box-drawing guides.
func (t treePrinter) print(p remote.Path, indent string, depth int) error {
	entries, err := t.c.List(p, false)
	if err != nil {
		return err
	}
	t.order.Sort(entries)
	for i, e := range entries {
		branch, next := "├── ", "│   "
		if i == len(entries)-1 {
			branch, next = "└── ", "    "
		}
		line := indent + branch + displayName(e, false)
		if showSizes() {
			line += "  (" + ui.Bytes(e.Size) + ")"
		}
		fmt.Fprintln(out, line)
		if e.IsDir() && depth != 1 {
			if err := t.print(p.Join(e.Name), indent+next, depth-1); err != nil {
				return err
			}
		}
	}
	return nil
}

// newFindCmd builds `mega find`.
func newFindCmd() *cobra.Command {
	var name, kind string
	var ignoreCase bool
	cmd := &cobra.Command{
		Use:   "find [path]",
		Short: "Search for files and folders by name",
		Example: `  mega find --name '*.pdf'
  mega find /photos --type d
  mega find -i --name 'readme*' --json | jq -r '.[].path'`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			o, err := findOptions(name, kind, ignoreCase)
			if err != nil {
				return err
			}
			c, err := connect()
			if err != nil {
				return err
			}
			found, err := c.Find(pathArgs(args)[0], o)
			if err != nil {
				return err
			}
			if flags.json {
				return ui.JSON(out, nonNil(found))
			}
			return printEntries(out, found, false, true)
		},
	}
	cmd.Flags().StringVarP(&name, "name", "n", "", "glob to match against the name")
	cmd.Flags().StringVarP(&kind, "type", "t", "", "f for files, d for folders")
	cmd.Flags().BoolVarP(&ignoreCase, "ignore-case", "i", false, "case-insensitive --name")
	return cmd
}

// findOptions converts the find flags into library options.
func findOptions(name, kind string, ignoreCase bool) (remote.FindOptions, error) {
	o := remote.FindOptions{Name: name, IgnoreCase: ignoreCase}
	switch kind {
	case "":
	case "f":
		o.Type = remote.TypeFile
	case "d":
		o.Type = remote.TypeFolder
	default:
		return o, fmt.Errorf("--type must be f or d")
	}
	return o, o.Validate()
}

// newStatCmd builds `mega stat`.
func newStatCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stat <path>",
		Short: "Show details about a file or folder",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := connect()
			if err != nil {
				return err
			}
			e, err := c.Stat(remote.Parse(args[0]))
			if err != nil {
				return err
			}
			if flags.json {
				return ui.JSON(out, e)
			}
			tw := tabwriter.NewWriter(out, 0, 0, 1, ' ', 0)
			fmt.Fprintf(tw, "Path:\t%s\nType:\t%s\nSize:\t%s (%d bytes)\n",
				e.Path, e.Type, ui.Bytes(e.Size), e.Size)
			fmt.Fprintf(tw, "Modified:\t%s\nHandle:\t%s\n",
				e.Modified.Local().Format("2006-01-02 15:04:05 MST"), e.Handle)
			return tw.Flush()
		},
	}
}

// newDuCmd builds `mega du`.
func newDuCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "du [path...]",
		Short: "Summarise disk usage of folders",
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := connect()
			if err != nil {
				return err
			}
			var all []remote.Usage
			for _, p := range pathArgs(args) {
				u, err := c.DiskUsage(p)
				if err != nil {
					return err
				}
				all = append(all, u)
			}
			if flags.json {
				return ui.JSON(out, all)
			}
			tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			for _, u := range all {
				fmt.Fprintf(tw, "%s\t%d files\t%d folders\t%s\n",
					ui.Bytes(u.Bytes), u.Files, u.Folders, u.Path)
			}
			return tw.Flush()
		},
	}
}
