package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/manzanit0/mega-cli/pkg/mirror"
	"github.com/manzanit0/mega-cli/pkg/remote"
)

// TestPrintEntries checks short and long listing output.
func TestPrintEntries(t *testing.T) {
	entries := []remote.Entry{
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
	entries := []remote.Entry{
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

// TestLocation checks mega:// parsing for sync arguments.
func TestLocation(t *testing.T) {
	for in, want := range map[string]string{
		"mega://Photos":       "/Photos",
		"mega:///Photos/2024": "/Photos/2024",
		"mega://":             "/",
		"mega://trash:/old":   "trash:/old",
	} {
		p, ok := location(in)
		if !ok || p.String() != want {
			t.Errorf("location(%q) = %q %v, want %q", in, p, ok, want)
		}
	}
	for _, local := range []string{"/Volumes/Backup", "Photos", "./mega://x"} {
		if _, ok := location(local); ok {
			t.Errorf("location(%q) treated as remote", local)
		}
	}
}

// TestRunSyncValidation checks argument errors happen before connecting.
func TestRunSyncValidation(t *testing.T) {
	cases := []struct {
		f        syncFlags
		src, dst string
	}{
		{syncFlags{}, "/a", "/b"},
		{syncFlags{}, "mega://a", "mega://b"},
		{syncFlags{permanent: true}, "/a", "mega://b"},
		{syncFlags{permanent: true, del: true}, "mega://a", "/b"},
	}
	for _, c := range cases {
		if err := runSync(&c.f, c.src, c.dst); err == nil {
			t.Errorf("runSync(%+v, %q, %q) = nil, want error", c.f, c.src, c.dst)
		}
	}
}

// TestCheckCpArgs checks cp rejects same-side and mixed copies.
func TestCheckCpArgs(t *testing.T) {
	dir := t.TempDir()
	ok := [][]string{
		{"a.txt", "mega://docs/"},
		{"a", "b", "mega://docs"},
		{"-", "mega://x"},
		{"mega://a", "mega://b", dir},
		{"mega://a", "-"},
		{"mega://a", "out.txt"},
	}
	for _, args := range ok {
		if err := checkCpArgs(args[:len(args)-1], args[len(args)-1]); err != nil {
			t.Errorf("checkCpArgs(%v) = %v", args, err)
		}
	}
	bad := [][]string{
		{"a", "b"},
		{"mega://a", "mega://b"},
		{"a", "mega://b", "mega://c"},
		{"mega://a", "b", dir},
		{"mega://a", "mega://b", dir + "/missing"},
	}
	for _, args := range bad {
		if err := checkCpArgs(args[:len(args)-1], args[len(args)-1]); err == nil {
			t.Errorf("checkCpArgs(%v) = nil, want error", args)
		}
	}
}

// TestConfirmLocalDeleteNonInteractive checks prompts are skipped off-TTY.
func TestConfirmLocalDeleteNonInteractive(t *testing.T) {
	f := syncFlags{}
	if err := f.confirmLocalDelete([]mirror.Action{{Op: mirror.Delete, Path: "x"}}); err != nil {
		t.Errorf("non-interactive confirm = %v, want nil", err)
	}
}
