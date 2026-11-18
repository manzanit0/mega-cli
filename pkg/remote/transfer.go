package remote

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	mega "github.com/t3rm1n4l/go-mega"
)

// ProgressFunc receives the number of bytes transferred since the last call.
type ProgressFunc func(n int)

// drain forwards progress channel updates to fn until the channel closes.
func drain(ch chan int, fn ProgressFunc, done chan struct{}) {
	for n := range ch {
		if fn != nil {
			fn(n)
		}
	}
	close(done)
}

// Download writes a remote file to dst atomically, via a temp file in the
// destination folder that is renamed once the MAC has been verified.
func (c *Client) Download(n *mega.Node, dst string, fn ProgressFunc) error {
	tmp, err := os.CreateTemp(filepath.Dir(dst), "."+filepath.Base(dst)+".*.part")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if err := tmp.Close(); err != nil {
		return err
	}
	ch := make(chan int)
	done := make(chan struct{})
	go drain(ch, fn, done)
	err = c.m.DownloadFile(n, name, &ch)
	<-done
	if err != nil {
		_ = os.Remove(name)
		return err
	}
	if err := os.Chmod(name, 0o644); err != nil {
		return err
	}
	return os.Rename(name, dst)
}

// Stream writes a remote file to w sequentially. The MAC is verified at
// the end, so a corrupted stream is reported only after data was written.
func (c *Client) Stream(n *mega.Node, w io.Writer, fn ProgressFunc) error {
	d, err := c.m.NewDownload(n)
	if err != nil {
		return err
	}
	for id := 0; id < d.Chunks(); id++ {
		chunk, err := d.DownloadChunk(id)
		if err != nil {
			return err
		}
		if _, err := w.Write(chunk); err != nil {
			return err
		}
		if fn != nil {
			fn(len(chunk))
		}
	}
	return d.Finish()
}

// Upload stores a local file as name inside parent. When replace is set
// and a file with the same name exists, it is moved to the trash after the
// new upload succeeds.
func (c *Client) Upload(src string, parent *mega.Node, name string, replace bool,
	fn ProgressFunc) (*mega.Node, error) {
	existing, err := c.Child(parent, name)
	if err != nil {
		return nil, err
	}
	if existing != nil && IsDir(existing) {
		return nil, fmt.Errorf("%s: a folder with that name exists", name)
	}
	if existing != nil && !replace {
		return nil, fmt.Errorf("%s: already exists (use --force to replace)", name)
	}
	c.invalidate()
	ch := make(chan int)
	done := make(chan struct{})
	go drain(ch, fn, done)
	node, err := c.m.UploadFile(src, parent, name, &ch)
	<-done
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if err := c.Delete(existing, false); err != nil {
			return node, fmt.Errorf("uploaded, but trashing old %s failed: %w", name, err)
		}
	}
	return node, nil
}
