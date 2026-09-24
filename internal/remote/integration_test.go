package remote

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// liveClient logs in with MEGA_EMAIL and MEGA_PASSWORD or skips the test.
func liveClient(t *testing.T) *Client {
	t.Helper()
	email, pass := os.Getenv("MEGA_EMAIL"), os.Getenv("MEGA_PASSWORD")
	if email == "" || pass == "" {
		t.Skip("set MEGA_EMAIL and MEGA_PASSWORD to run live tests")
	}
	c, err := Login(email, pass, os.Getenv("MEGA_MFA"), Options{})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	return c
}

// TestLiveRoundTrip exercises mkdir, upload, download, rename, move, link
// and delete against a real account inside a throwaway folder.
func TestLiveRoundTrip(t *testing.T) {
	c := liveClient(t)
	base := Parse(fmt.Sprintf("/mega-cli-test-%d", time.Now().UnixNano()))
	dir, err := c.Mkdir(base.Join("sub"), true)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() {
		if n, err := c.Lookup(base); err == nil {
			_ = c.Delete(n, true)
		}
	})

	tmp := t.TempDir()
	src := filepath.Join(tmp, "in.txt")
	want := []byte(strings.Repeat("mega-cli ", 50000))
	if err := os.WriteFile(src, want, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Upload(src, dir, "a.txt", false, nil); err != nil {
		t.Fatalf("upload: %v", err)
	}
	if _, err := c.Upload(src, dir, "a.txt", false, nil); err == nil {
		t.Fatalf("expected upload over existing file to fail without replace")
	}

	n, err := c.Lookup(base.Join("sub").Join("a.txt"))
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if top, _ := c.Lookup(base); c.Size(top) != int64(len(want)) {
		t.Fatalf("folder size = %d, want %d", c.Size(top), len(want))
	}
	dst := filepath.Join(tmp, "out.txt")
	if err := c.Download(n, dst, nil); err != nil {
		t.Fatalf("download: %v", err)
	}
	if got, _ := os.ReadFile(dst); !bytes.Equal(got, want) {
		t.Fatalf("downloaded content mismatch")
	}
	var buf bytes.Buffer
	if err := c.Stream(n, &buf, nil); err != nil || !bytes.Equal(buf.Bytes(), want) {
		t.Fatalf("stream mismatch: %v", err)
	}

	if err := c.Rename(n, "b.txt"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	root, _ := c.Lookup(base)
	if err := c.Move(n, root); err != nil {
		t.Fatalf("move: %v", err)
	}
	if _, err := c.Lookup(base.Join("b.txt")); err != nil {
		t.Fatalf("lookup after move: %v", err)
	}
	link, err := c.Link(n, true)
	if err != nil || !strings.Contains(link, "#!") {
		t.Fatalf("link: %q %v", link, err)
	}
	if err := c.Delete(n, false); err != nil {
		t.Fatalf("trash: %v", err)
	}
	if _, err := c.Lookup(base.Join("b.txt")); err == nil {
		t.Fatalf("file still present after trashing")
	}
}

// TestLiveLogout checks a logged-out session can no longer be resumed.
func TestLiveLogout(t *testing.T) {
	c := liveClient(t)
	sid, key := c.SessionID(), c.MasterKey()
	if err := Logout(sid); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if _, err := Resume(sid, key, Options{}); err == nil {
		t.Fatal("resume succeeded after logout")
	}
}
