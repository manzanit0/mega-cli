package transfer

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/manzanit0/mega-cli/pkg/remote"
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

// TestLiveTransferRoundTrip uploads a tree, finds, moves, downloads and
// removes it inside a throwaway folder.
func TestLiveTransferRoundTrip(t *testing.T) {
	c := liveClient(t)
	base := remote.Parse(fmt.Sprintf("/mega-cli-transfer-%d", time.Now().UnixNano()))
	t.Cleanup(func() {
		_, _ = c.Remove(base, remote.RemoveOptions{Recursive: true, Permanent: true})
	})

	src := filepath.Join(t.TempDir(), "tree")
	if err := os.MkdirAll(filepath.Join(src, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	want := []byte(strings.Repeat("mega-cli ", 1000))
	if err := os.WriteFile(filepath.Join(src, "sub", "a.txt"), want, 0o600); err != nil {
		t.Fatal(err)
	}

	stats, err := Upload(c, []string{src}, base.String()+"/", Options{})
	if err != nil || stats.Files != 1 {
		t.Fatalf("upload: %+v %v", stats, err)
	}
	found, err := c.Find(base, remote.FindOptions{Name: "a.*", Type: remote.TypeFile})
	if err != nil || len(found) != 1 {
		t.Fatalf("find: %v %v", found, err)
	}
	moved := base.Join("tree").Join("sub").Join("a.txt")
	renamed := base.Join("tree").Join("b.txt")
	if err := c.MovePaths([]remote.Path{moved}, renamed, nil); err != nil {
		t.Fatalf("move: %v", err)
	}

	dst := t.TempDir()
	stats, err = Download(c, []remote.Path{base.Join("tree")}, dst, Options{})
	if err != nil || stats.Files != 1 {
		t.Fatalf("download: %+v %v", stats, err)
	}
	if got, _ := os.ReadFile(filepath.Join(dst, "tree", "b.txt")); !bytes.Equal(got, want) {
		t.Fatal("downloaded content mismatch")
	}

	stats, err = UploadReader(c, strings.NewReader("hi"), base.Join("stdin.txt"), Options{})
	if err != nil || stats.Files != 1 {
		t.Fatalf("upload reader: %+v %v", stats, err)
	}
	var buf bytes.Buffer
	if err := c.Cat(base.Join("stdin.txt"), &buf, nil); err != nil || buf.String() != "hi" {
		t.Fatalf("cat: %q %v", buf.String(), err)
	}

	res, err := c.Remove(base.Join("tree"), remote.RemoveOptions{Recursive: true})
	if err != nil || !res.Removed || res.Destroyed {
		t.Fatalf("remove: %+v %v", res, err)
	}
}
