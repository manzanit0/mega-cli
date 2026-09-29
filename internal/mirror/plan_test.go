package mirror

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

// t0 is a fixed reference time for tests.
var t0 = time.Unix(1_700_000_000, 0)

// f builds a file entry.
func f(size int64, mtime time.Time) Entry {
	return Entry{Size: size, MTime: mtime}
}

// d is a folder entry.
var d = Entry{Dir: true}

// render flattens actions to "op path" strings for comparison.
func render(as []Action) []string {
	out := make([]string, len(as))
	for i, a := range as {
		out[i] = a.Op.String() + " " + a.Path
	}
	return out
}

// TestPlanCopiesUpdatesAndKeeps checks the no-delete plan.
func TestPlanCopiesUpdatesAndKeeps(t *testing.T) {
	src := Tree{
		"a":       d,
		"a/new":   f(1, t0),
		"a/same":  f(2, t0),
		"a/grown": f(9, t0),
		"b":       d,
	}
	dst := Tree{
		"a":       d,
		"a/same":  f(2, t0),
		"a/grown": f(3, t0),
		"extra":   f(1, t0),
	}
	got, err := Plan(src, dst, ChangedDown, false)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"update a/grown", "copy a/new", "mkdir b"}
	if !reflect.DeepEqual(render(got), want) {
		t.Errorf("got %v, want %v", render(got), want)
	}
}

// TestPlanDeletesTopmostOnly checks extras are deleted once per subtree.
func TestPlanDeletesTopmostOnly(t *testing.T) {
	src := Tree{"keep": f(1, t0)}
	dst := Tree{
		"keep":     f(1, t0),
		"old":      d,
		"old/x":    f(1, t0),
		"old/y":    d,
		"old/y/z":  f(1, t0),
		"stale.md": f(1, t0),
	}
	got, err := Plan(src, dst, ChangedDown, true)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"delete old", "delete stale.md"}
	if !reflect.DeepEqual(render(got), want) {
		t.Errorf("got %v, want %v", render(got), want)
	}
}

// TestPlanTypeClash checks file/folder clashes need --delete.
func TestPlanTypeClash(t *testing.T) {
	src := Tree{"x": d, "x/in": f(1, t0)}
	dst := Tree{"x": f(5, t0)}
	if _, err := Plan(src, dst, ChangedDown, false); err == nil {
		t.Fatal("expected clash error")
	}
	got, err := Plan(src, dst, ChangedDown, true)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"delete x", "mkdir x", "copy x/in"}
	if !reflect.DeepEqual(render(got), want) {
		t.Errorf("got %v, want %v", render(got), want)
	}
}

// TestPlanOrdersParentsFirst checks "a" subtree precedes "a b".
func TestPlanOrdersParentsFirst(t *testing.T) {
	src := Tree{"a": d, "a b": d, "a/c": d}
	got, _ := Plan(src, Tree{}, ChangedDown, false)
	want := []string{"mkdir a", "mkdir a/c", "mkdir a b"}
	if !reflect.DeepEqual(render(got), want) {
		t.Errorf("got %v, want %v", render(got), want)
	}
}

// TestChanged checks direction-specific change detection.
func TestChanged(t *testing.T) {
	later := t0.Add(time.Hour)
	if ChangedDown(f(1, t0), f(1, t0.Add(300*time.Millisecond))) {
		t.Error("down: sub-second difference counted as change")
	}
	if !ChangedDown(f(1, t0), f(1, later)) {
		t.Error("down: locally touched file not re-downloaded")
	}
	if ChangedUp(f(1, t0), f(1, later)) {
		t.Error("up: file older than its upload counted as change")
	}
	if !ChangedUp(f(1, later), f(1, t0)) {
		t.Error("up: file edited after upload not detected")
	}
	if !ChangedUp(f(2, t0), f(1, later)) {
		t.Error("up: size change not detected")
	}
}

// TestConfirmDeletes checks only delete actions are sent for approval and
// that a refusal is returned.
func TestConfirmDeletes(t *testing.T) {
	plan := []Action{{Op: Copy, Path: "a"}, {Op: Delete, Path: "b"}}
	var seen []Action
	o := Options{ConfirmLocalDelete: func(d []Action) error {
		seen = d
		return errAbort
	}}
	if err := confirmDeletes(plan, o); err != errAbort {
		t.Fatalf("confirmDeletes = %v", err)
	}
	if len(seen) != 1 || seen[0].Path != "b" {
		t.Fatalf("confirm saw %v", seen)
	}
	if err := confirmDeletes(plan[:1], o); err != nil {
		t.Fatalf("no deletes should not prompt: %v", err)
	}
}

// errAbort is a sentinel returned by test confirmations.
var errAbort = errors.New("abort")
