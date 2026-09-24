package cmd

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// TestMatches checks the find filters.
func TestMatches(t *testing.T) {
	file := Entry{Name: "Report.PDF", Type: "file"}
	dir := Entry{Name: "docs", Type: "folder"}
	cases := []struct {
		e       Entry
		pattern string
		kind    string
		ci      bool
		want    bool
	}{
		{file, "", "", false, true},
		{file, "*.pdf", "", false, false},
		{file, "*.pdf", "", true, true},
		{file, "", "d", false, false},
		{dir, "", "d", false, true},
		{dir, "d*", "f", false, false},
	}
	for _, c := range cases {
		if got := matches(c.e, c.pattern, c.kind, c.ci); got != c.want {
			t.Errorf("matches(%v, %q, %q, %v) = %v", c.e, c.pattern, c.kind, c.ci, got)
		}
	}
}

// TestPrintEntries checks short and long listing output.
func TestPrintEntries(t *testing.T) {
	entries := []Entry{
		{Name: "a.txt", Path: "/x/a.txt", Type: "file", Size: 2048},
		{Name: "sub", Path: "/x/sub", Type: "folder"},
	}
	var b bytes.Buffer
	if err := printEntries(&b, entries, false, false); err != nil {
		t.Fatal(err)
	}
	if b.String() != "a.txt\nsub/\n" {
		t.Errorf("short = %q", b.String())
	}
	b.Reset()
	if err := printEntries(&b, entries, true, true); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "2.0 KiB") || !strings.Contains(b.String(), "/x/sub/") {
		t.Errorf("long = %q", b.String())
	}
}

// TestPrintEntriesSizes checks short listings show sizes on a terminal.
func TestPrintEntriesSizes(t *testing.T) {
	orig := showSizes
	t.Cleanup(func() { showSizes = orig })
	showSizes = func() bool { return true }
	entries := []Entry{
		{Name: "a.txt", Type: "file", Size: 2048},
		{Name: "subfolder", Type: "folder", Size: 3 << 20},
	}
	var b bytes.Buffer
	if err := printEntries(&b, entries, false, false); err != nil {
		t.Fatal(err)
	}
	want := "a.txt       2.0 KiB\nsubfolder/  3.0 MiB\n"
	if b.String() != want {
		t.Errorf("sizes = %q, want %q", b.String(), want)
	}
}

// TestSourceName checks remote names derived from local sources.
func TestSourceName(t *testing.T) {
	cases := map[string]string{"-": "stdin", "a/b/": "b", "./c.txt": "c.txt"}
	for in, want := range cases {
		if got := sourceName(in); got != want {
			t.Errorf("sourceName(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestSortOrder checks name, size and time ordering and reversal.
func TestSortOrder(t *testing.T) {
	now := time.Now()
	entries := func() []Entry {
		return []Entry{
			{Name: "b", Size: 10, Modified: now.Add(-time.Hour)},
			{Name: "a", Size: 10, Modified: now},
			{Name: "c", Size: 99, Modified: now.Add(-2 * time.Hour)},
		}
	}
	cases := []struct {
		o    sortOrder
		want string
	}{
		{sortOrder{key: "name"}, "abc"},
		{sortOrder{key: "name", reverse: true}, "cba"},
		{sortOrder{key: "size"}, "cab"},
		{sortOrder{key: "size", reverse: true}, "bac"},
		{sortOrder{key: "time"}, "abc"},
		{sortOrder{key: "time", reverse: true}, "cba"},
	}
	for _, c := range cases {
		es := entries()
		c.o.sortEntries(es)
		got := es[0].Name + es[1].Name + es[2].Name
		if got != c.want {
			t.Errorf("%+v: got %s, want %s", c.o, got, c.want)
		}
	}
	if err := (sortOrder{key: "bogus"}).validate(); err == nil {
		t.Error("expected invalid key error")
	}
}
