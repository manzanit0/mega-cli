package mirror

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/manzanit0/mega-cli/internal/remote"
)

// liveClient logs in with MEGA_EMAIL and MEGA_PASSWORD or skips the test.
func liveClient(t *testing.T) *remote.Client {
	t.Helper()
	email, pass := os.Getenv("MEGA_EMAIL"), os.Getenv("MEGA_PASSWORD")
	if email == "" || pass == "" {
		t.Skip("set MEGA_EMAIL and MEGA_PASSWORD to run live tests")
	}
	c, err := remote.Login(email, pass, os.Getenv("MEGA_MFA"), remote.Options{})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	return c
}

// mustSync runs a sync step and checks how many actions it planned.
func mustSync(t *testing.T, step string, want int, r Result, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", step, err)
	}
	if len(r.Actions) != want {
		t.Fatalf("%s: %d actions %v, want %d", step, len(r.Actions), r.Actions, want)
	}
}

// TestLiveSyncRoundTrip syncs up, edits, deletes, then syncs down and
// checks that repeated runs are no-ops.
func TestLiveSyncRoundTrip(t *testing.T) {
	c := liveClient(t)
	base := remote.Parse(fmt.Sprintf("/mega-cli-sync-%d", time.Now().UnixNano()))
	t.Cleanup(func() {
		if n, err := c.Lookup(base); err == nil {
			_ = c.Delete(n, true)
		}
	})
	src := t.TempDir()
	write(t, src, "a/one.txt", "one")
	write(t, src, "two.txt", "two")
	o := Options{Delete: true}

	r, err := Up(c, src, base, o)
	mustSync(t, "first up", 3, r, err)
	r, err = Up(c, src, base, o)
	mustSync(t, "repeat up", 0, r, err)

	future := time.Now().Add(time.Hour)
	write(t, src, "two.txt", "TWO")
	if err := os.Chtimes(filepath.Join(src, "two.txt"), future, future); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(src, "a")); err != nil {
		t.Fatal(err)
	}
	r, err = Up(c, src, base, o)
	mustSync(t, "up with edit and delete", 2, r, err)

	dst := t.TempDir()
	write(t, dst, "stale.txt", "x")
	r, err = Down(c, base, dst, o)
	mustSync(t, "first down", 2, r, err)
	r, err = Down(c, base, dst, o)
	mustSync(t, "repeat down", 0, r, err)
	if b, _ := os.ReadFile(filepath.Join(dst, "two.txt")); string(b) != "TWO" {
		t.Fatalf("downloaded content = %q", b)
	}
}
