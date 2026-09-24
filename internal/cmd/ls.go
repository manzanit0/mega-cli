package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	mega "github.com/t3rm1n4l/go-mega"

	"github.com/manzanit0/mega-cli/internal/remote"
	"github.com/manzanit0/mega-cli/internal/ui"
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

// listing collects the entries shown for one path argument.
func listing(c *remote.Client, p remote.Path, recursive bool) ([]Entry, error) {
	var entries []Entry
	if recursive {
		err := c.Walk(p, func(cp remote.Path, n *mega.Node) error {
			entries = append(entries, newEntry(c, cp, n))
			return nil
		})
		return entries, err
	}
	if !p.IsRoot() {
		n, err := c.Lookup(p)
		if err != nil && !errors.Is(err, remote.ErrSharedRoot) {
			return nil, err
		}
		if n != nil && !remote.IsDir(n) {
			return []Entry{newEntry(c, p, n)}, nil
		}
	}
	nodes, err := c.Children(p)
	if err != nil {
		return nil, err
	}
	for _, n := range nodes {
		entries = append(entries, newEntry(c, p.Join(n.GetName()), n))
	}
	return entries, nil
}

// displayName returns the name to show, marking folders with a slash.
func displayName(e Entry, full bool) string {
	name := e.Name
	if full {
		name = e.Path
	}
	if e.Type == "folder" {
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
func sizeOf(e Entry) string {
	return ui.Bytes(e.Size)
}

// printEntries renders entries as names or a long table.
func printEntries(w io.Writer, entries []Entry, long, full bool) error {
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
	var order sortOrder
	cmd := &cobra.Command{
		Use:     "ls [path...]",
		Aliases: []string{"list"},
		Short:   "List folder contents",
		Example: `  mega ls -l --sort size /backup
  mega ls -R -s size / | head`,
		RunE: func(_ *cobra.Command, args []string) error {
			if err := order.validate(); err != nil {
				return err
			}
			c, err := connect()
			if err != nil {
				return err
			}
			ps := pathArgs(args)
			var all []Entry
			for i, p := range ps {
				entries, err := listing(c, p, recursive)
				if err != nil {
					return err
				}
				order.sortEntries(entries)
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
func nonNil(e []Entry) []Entry {
	if e == nil {
		return []Entry{}
	}
	return e
}

// newTreeCmd builds `mega tree`.
func newTreeCmd() *cobra.Command {
	var depth int
	var order sortOrder
	cmd := &cobra.Command{
		Use:     "tree [path]",
		Short:   "Show a folder as a tree",
		Example: `  mega tree -L 2 --sort size /projects`,
		Args:    cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if err := order.validate(); err != nil {
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
	order sortOrder
}

// treeItem pairs a node with its display entry.
type treeItem struct {
	// e is the display entry.
	e Entry
	// n is the underlying node.
	n *mega.Node
}

// children returns the sorted children of p.
func (t treePrinter) children(p remote.Path) ([]treeItem, error) {
	nodes, err := t.c.Children(p)
	if err != nil {
		return nil, err
	}
	items := make([]treeItem, len(nodes))
	for i, n := range nodes {
		items[i] = treeItem{e: newEntry(t.c, p.Join(n.GetName()), n), n: n}
	}
	slices.SortStableFunc(items, func(a, b treeItem) int {
		return t.order.compare(a.e, b.e)
	})
	return items, nil
}

// print recursively prints the children of p with box-drawing guides.
func (t treePrinter) print(p remote.Path, indent string, depth int) error {
	items, err := t.children(p)
	if err != nil {
		return err
	}
	for i, it := range items {
		branch, next := "├── ", "│   "
		if i == len(items)-1 {
			branch, next = "└── ", "    "
		}
		line := indent + branch + displayName(it.e, false)
		if showSizes() {
			line += "  (" + ui.Bytes(it.e.Size) + ")"
		}
		fmt.Fprintln(out, line)
		if remote.IsDir(it.n) && depth != 1 {
			if err := t.print(p.Join(it.e.Name), indent+next, depth-1); err != nil {
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
			if kind != "" && kind != "f" && kind != "d" {
				return fmt.Errorf("--type must be f or d")
			}
			pattern := name
			if ignoreCase {
				pattern = strings.ToLower(pattern)
			}
			if _, err := path.Match(pattern, ""); err != nil {
				return fmt.Errorf("bad --name pattern: %w", err)
			}
			c, err := connect()
			if err != nil {
				return err
			}
			var found []Entry
			err = c.Walk(pathArgs(args)[0], func(p remote.Path, n *mega.Node) error {
				e := newEntry(c, p, n)
				if matches(e, pattern, kind, ignoreCase) {
					found = append(found, e)
				}
				return nil
			})
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

// matches reports whether e satisfies the find filters.
func matches(e Entry, pattern, kind string, ignoreCase bool) bool {
	if kind == "f" && e.Type != "file" || kind == "d" && e.Type != "folder" {
		return false
	}
	if pattern == "" {
		return true
	}
	n := e.Name
	if ignoreCase {
		n = strings.ToLower(n)
	}
	ok, _ := path.Match(pattern, n)
	return ok
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
			p := remote.Parse(args[0])
			n, err := c.Lookup(p)
			if err != nil {
				return err
			}
			e := newEntry(c, p, n)
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

// usage is the output of `mega du`.
type usage struct {
	// Path is the measured path.
	Path string `json:"path"`
	// Bytes is the total size of all files below the path.
	Bytes int64 `json:"bytes"`
	// Files is the number of files below the path.
	Files int `json:"files"`
	// Folders is the number of folders below the path.
	Folders int `json:"folders"`
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
			var all []usage
			for _, p := range pathArgs(args) {
				u, err := diskUsage(c, p)
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

// diskUsage totals the files and folders below p.
func diskUsage(c *remote.Client, p remote.Path) (usage, error) {
	u := usage{Path: p.String()}
	if !p.IsRoot() {
		n, err := c.Lookup(p)
		if err != nil {
			return u, err
		}
		if !remote.IsDir(n) {
			u.Bytes, u.Files = n.GetSize(), 1
			return u, nil
		}
	}
	err := c.Walk(p, func(_ remote.Path, n *mega.Node) error {
		if remote.IsDir(n) {
			u.Folders++
		} else {
			u.Files++
			u.Bytes += n.GetSize()
		}
		return nil
	})
	return u, err
}
