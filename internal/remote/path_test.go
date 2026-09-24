package remote

import (
	"reflect"
	"testing"
)

// TestParse checks path normalisation and namespace detection.
func TestParse(t *testing.T) {
	cases := []struct {
		in    string
		ns    Namespace
		parts []string
		str   string
	}{
		{"", Cloud, nil, "/"},
		{"/", Cloud, nil, "/"},
		{"docs", Cloud, []string{"docs"}, "/docs"},
		{"/docs/a.txt", Cloud, []string{"docs", "a.txt"}, "/docs/a.txt"},
		{"docs//b/../a/", Cloud, []string{"docs", "a"}, "/docs/a"},
		{"../..", Cloud, nil, "/"},
		{"trash:", Trash, nil, "trash:/"},
		{"trash:/old", Trash, []string{"old"}, "trash:/old"},
		{"shared:team/x", Shared, []string{"team", "x"}, "shared:/team/x"},
	}
	for _, c := range cases {
		p := Parse(c.in)
		if p.NS != c.ns {
			t.Errorf("Parse(%q).NS = %v, want %v", c.in, p.NS, c.ns)
		}
		if !reflect.DeepEqual(p.Parts, c.parts) {
			t.Errorf("Parse(%q).Parts = %v, want %v", c.in, p.Parts, c.parts)
		}
		if p.String() != c.str {
			t.Errorf("Parse(%q).String() = %q, want %q", c.in, p.String(), c.str)
		}
	}
}

// TestPathHelpers checks Base, Dir and Join.
func TestPathHelpers(t *testing.T) {
	p := Parse("/a/b/c")
	if p.Base() != "c" {
		t.Errorf("Base = %q", p.Base())
	}
	if got := p.Dir().String(); got != "/a/b" {
		t.Errorf("Dir = %q", got)
	}
	d := p.Dir()
	j := d.Join("x")
	if j.String() != "/a/b/x" || p.String() != "/a/b/c" {
		t.Errorf("Join mutated or wrong: %q %q", j, p)
	}
	if r := Parse("/"); r.Dir().String() != "/" || r.Base() != "" {
		t.Errorf("root helpers wrong")
	}
}
