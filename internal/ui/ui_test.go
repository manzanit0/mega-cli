package ui

import (
	"strings"
	"testing"
)

// TestBytes checks human-readable size formatting.
func TestBytes(t *testing.T) {
	cases := map[int64]string{
		0:               "0 B",
		1023:            "1023 B",
		1024:            "1.0 KiB",
		1536:            "1.5 KiB",
		5 * 1024 * 1024: "5.0 MiB",
		1 << 40:         "1.0 TiB",
	}
	for in, want := range cases {
		if got := Bytes(in); got != want {
			t.Errorf("Bytes(%d) = %q, want %q", in, got, want)
		}
	}
}

// TestNilProgress checks that a disabled Progress is a no-op.
func TestNilProgress(t *testing.T) {
	p := NewProgress(nil, false, "x", 10)
	p.Add(5)
	p.Done()
}

// TestTable checks alignment with wide and combining characters.
func TestTable(t *testing.T) {
	var b strings.Builder
	rows := [][]string{
		{"【Ethereum wallet】.txt", "261 B"},
		{"Educación/", "1.0 GiB"},
		{"a", "792.4 MiB"},
	}
	if err := Table(&b, []Align{Left, Right}, rows); err != nil {
		t.Fatal(err)
	}
	want := "【Ethereum wallet】.txt      261 B\n" +
		"Educación/                 1.0 GiB\n" +
		"a                        792.4 MiB\n"
	if b.String() != want {
		t.Errorf("got:\n%s\nwant:\n%s", b.String(), want)
	}
}

// TestTableNoTrailingSpace checks a left-aligned last column is unpadded.
func TestTableNoTrailingSpace(t *testing.T) {
	var b strings.Builder
	_ = Table(&b, nil, [][]string{{"x", "long"}, {"yy", "s"}})
	if b.String() != "x   long\nyy  s\n" {
		t.Errorf("got %q", b.String())
	}
}
