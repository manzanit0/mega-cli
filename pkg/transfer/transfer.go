// Package transfer copies files and folders between the local filesystem
// and MEGA, one-off and recursively.
package transfer

import (
	"io"

	"github.com/manzanit0/mega-cli/pkg/remote"
)

// Stats tallies a batch of transfers.
type Stats struct {
	// Files is the number of files transferred.
	Files int `json:"files"`
	// Skipped is the number of files left untouched.
	Skipped int `json:"skipped"`
	// Bytes is the number of bytes transferred.
	Bytes int64 `json:"bytes"`
}

// StdinName is the source name that makes Upload read Options.Stdin.
const StdinName = "-"

// Options tunes uploads and downloads. The zero value is valid.
type Options struct {
	// Force replaces existing remote files on upload; replaced versions go
	// to the trash.
	Force bool
	// SkipExisting leaves files that already exist at the destination with
	// the same size untouched. On upload it also replaces files that differ.
	SkipExisting bool
	// Progress reports byte counts for each file; may be nil.
	Progress remote.ProgressFactory
	// OnFile is called after each file is transferred; may be nil.
	OnFile func(src, dst string)
	// OnSkip is called for local items that cannot be uploaded, such as
	// sockets and symlinks; may be nil.
	OnSkip func(path string)
	// Stdin is read when a source of Upload is StdinName.
	Stdin io.Reader
}

// notify reports a transferred file.
func (o Options) notify(src, dst string) {
	if o.OnFile != nil {
		o.OnFile(src, dst)
	}
}

// skipped reports a local item that cannot be uploaded.
func (o Options) skipped(path string) {
	if o.OnSkip != nil {
		o.OnSkip(path)
	}
}
