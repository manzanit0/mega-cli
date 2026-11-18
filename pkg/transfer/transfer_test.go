package transfer

import "testing"

// TestSourceName checks remote names derived from local sources.
func TestSourceName(t *testing.T) {
	cases := map[string]string{StdinName: "stdin", "a/b/": "b", "./c.txt": "c.txt"}
	for in, want := range cases {
		if got := sourceName(in); got != want {
			t.Errorf("sourceName(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestProgressNil checks a nil progress factory is safe to use.
func TestProgressNil(t *testing.T) {
	var o Options
	pf, done := o.Progress.Start("x", 1)
	if pf != nil {
		t.Error("expected nil counter")
	}
	done()
}
