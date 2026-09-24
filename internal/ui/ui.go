// Package ui holds terminal output helpers.
package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"golang.org/x/term"
)

// Bytes formats a size using binary units, e.g. "1.5 MiB".
func Bytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

// IsTerminal reports whether f is attached to a terminal.
func IsTerminal(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}

// JSON writes v as indented JSON followed by a newline.
func JSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// Progress renders a single-line transfer indicator on a terminal.
type Progress struct {
	// w is where the progress line is drawn.
	w io.Writer
	// label names the item being transferred.
	label string
	// total is the expected number of bytes.
	total int64
	// mu guards done and last.
	mu sync.Mutex
	// done is the number of bytes transferred so far.
	done int64
	// last is when the line was last redrawn.
	last time.Time
	// start is when the transfer began.
	start time.Time
}

// NewProgress returns a Progress, or nil when enabled is false. All methods
// are safe to call on a nil Progress.
func NewProgress(w io.Writer, enabled bool, label string, total int64) *Progress {
	if !enabled {
		return nil
	}
	return &Progress{w: w, label: label, total: total, start: time.Now()}
}

// Add records n transferred bytes and redraws at most every 100ms.
func (p *Progress) Add(n int) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.done += int64(n)
	if time.Since(p.last) < 100*time.Millisecond && p.done < p.total {
		return
	}
	p.last = time.Now()
	p.draw()
}

// draw writes the current progress line. Callers must hold mu.
func (p *Progress) draw() {
	pct := 100.0
	if p.total > 0 {
		pct = float64(p.done) * 100 / float64(p.total)
	}
	rate := float64(p.done) / time.Since(p.start).Seconds()
	fmt.Fprintf(p.w, "\r\033[K%s  %s / %s  %3.0f%%  %s/s",
		p.label, Bytes(p.done), Bytes(p.total), pct, Bytes(int64(rate)))
}

// Done clears the progress line.
func (p *Progress) Done() {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	fmt.Fprint(p.w, "\r\033[K")
}
