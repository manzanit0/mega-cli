package remote

import (
	"bytes"
	"errors"
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
	sub := base.Join("sub")
	if _, err := c.Mkdir(sub, true); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() {
		_, _ = c.Remove(base, RemoveOptions{Recursive: true, Permanent: true})
	})

	top, err := c.lookup(base)
	if err != nil {
		t.Fatalf("lookup base: %v", err)
	}
	if _, err := c.m.CreateDir("2014/2015", top); err != nil {
		t.Fatalf("create slash folder: %v", err)
	}
	if p, e, err := c.Resolve(base.Join("2014").Join("2015")); err != nil ||
		e.Name != "2014/2015" || p.Base() != "2014/2015" {
		t.Fatalf("resolve slash folder: %v %v", p, err)
	}

	tmp := t.TempDir()
	src := filepath.Join(tmp, "in.txt")
	want := []byte(strings.Repeat("mega-cli ", 50000))
	if err := os.WriteFile(src, want, 0o600); err != nil {
		t.Fatal(err)
	}
	file := sub.Join("a.txt")
	if _, err := c.Upload(src, file, false, nil); err != nil {
		t.Fatalf("upload: %v", err)
	}
	if _, err := c.Upload(src, file, false, nil); !errors.Is(err, ErrExists) {
		t.Fatalf("upload over existing file = %v, want ErrExists", err)
	}

	e, err := c.Stat(file)
	if err != nil || e.Size != int64(len(want)) || e.IsDir() {
		t.Fatalf("stat: %+v %v", e, err)
	}
	if top, err := c.Stat(base); err != nil || top.Size != int64(len(want)) {
		t.Fatalf("folder size: %+v %v", top, err)
	}
	dst := filepath.Join(tmp, "out.txt")
	if err := c.Download(file, dst, nil); err != nil {
		t.Fatalf("download: %v", err)
	}
	if got, _ := os.ReadFile(dst); !bytes.Equal(got, want) {
		t.Fatalf("downloaded content mismatch")
	}
	var buf bytes.Buffer
	if err := c.Cat(file, &buf, nil); err != nil || !bytes.Equal(buf.Bytes(), want) {
		t.Fatalf("cat mismatch: %v", err)
	}

	moved := base.Join("b.txt")
	if err := c.MovePaths([]Path{file}, moved, nil); err != nil {
		t.Fatalf("move: %v", err)
	}
	if _, err := c.Stat(moved); err != nil {
		t.Fatalf("stat after move: %v", err)
	}
	link, err := c.PublicLink(moved, true)
	if err != nil || !strings.Contains(link, "#!") {
		t.Fatalf("link: %q %v", link, err)
	}
	res, err := c.Remove(moved, RemoveOptions{})
	if err != nil || !res.Removed || res.Destroyed {
		t.Fatalf("trash: %+v %v", res, err)
	}
	if _, err := c.Stat(moved); !IsNotFound(err) {
		t.Fatalf("file still present after trashing: %v", err)
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
