package remote

import (
	"testing"
	"time"
)

// TestOrder checks name, size and time ordering and reversal.
func TestOrder(t *testing.T) {
	now := time.Now()
	entries := func() []Entry {
		return []Entry{
			{Name: "b", Size: 10, Modified: now.Add(-time.Hour)},
			{Name: "a", Size: 10, Modified: now},
			{Name: "c", Size: 99, Modified: now.Add(-2 * time.Hour)},
		}
	}
	cases := []struct {
		o    Order
		want string
	}{
		{Order{}, "abc"},
		{Order{Key: SortName}, "abc"},
		{Order{Key: SortName, Reverse: true}, "cba"},
		{Order{Key: SortSize}, "cab"},
		{Order{Key: SortSize, Reverse: true}, "bac"},
		{Order{Key: SortTime}, "abc"},
		{Order{Key: SortTime, Reverse: true}, "cba"},
	}
	for _, c := range cases {
		es := entries()
		c.o.Sort(es)
		got := es[0].Name + es[1].Name + es[2].Name
		if got != c.want {
			t.Errorf("%+v: got %s, want %s", c.o, got, c.want)
		}
	}
	if err := (Order{Key: "bogus"}).Validate(); err == nil {
		t.Error("expected invalid key error")
	}
	if err := (Order{}).Validate(); err != nil {
		t.Errorf("empty key = %v, want nil", err)
	}
}

// TestFindMatches checks the find filters.
func TestFindMatches(t *testing.T) {
	file := Entry{Name: "Report.PDF", Type: TypeFile}
	dir := Entry{Name: "docs", Type: TypeFolder}
	cases := []struct {
		e    Entry
		o    FindOptions
		want bool
	}{
		{file, FindOptions{}, true},
		{file, FindOptions{Name: "*.pdf"}, false},
		{file, FindOptions{Name: "*.pdf", IgnoreCase: true}, true},
		{file, FindOptions{Type: TypeFolder}, false},
		{dir, FindOptions{Type: TypeFolder}, true},
		{dir, FindOptions{Name: "d*", Type: TypeFile}, false},
	}
	for _, c := range cases {
		if got := c.o.Matches(c.e); got != c.want {
			t.Errorf("%+v.Matches(%v) = %v", c.o, c.e, got)
		}
	}
}

// TestFindValidate checks bad types and globs are rejected.
func TestFindValidate(t *testing.T) {
	if err := (FindOptions{Type: "x"}).Validate(); err == nil {
		t.Error("expected invalid type error")
	}
	if err := (FindOptions{Name: "[a"}).Validate(); err == nil {
		t.Error("expected invalid glob error")
	}
	if err := (FindOptions{Name: "*.go", Type: TypeFile}).Validate(); err != nil {
		t.Errorf("valid options = %v", err)
	}
}

// TestStoragePercent checks the capacity ratio.
func TestStoragePercent(t *testing.T) {
	if got := (Storage{Used: 25, Total: 100}).Percent(); got != 25 {
		t.Errorf("Percent = %v, want 25", got)
	}
	if got := (Storage{}).Percent(); got != 0 {
		t.Errorf("empty Percent = %v, want 0", got)
	}
}
