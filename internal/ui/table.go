package ui

import (
	"bufio"
	"io"
	"strings"

	"github.com/mattn/go-runewidth"
)

// Align is a column's horizontal alignment.
type Align int

const (
	// Left pads cells on the right.
	Left Align = iota
	// Right pads cells on the left.
	Right
)

// columnGap is the number of spaces between columns.
const columnGap = 2

// Table writes rows as aligned columns, measuring cells by terminal display
// width so wide (e.g. CJK) and combining characters line up. A trailing
// left-aligned column is not padded, avoiding trailing whitespace.
func Table(w io.Writer, align []Align, rows [][]string) error {
	widths := columnWidths(rows)
	bw := bufio.NewWriter(w)
	for _, row := range rows {
		bw.WriteString(formatRow(row, widths, align))
		bw.WriteByte('\n')
	}
	return bw.Flush()
}

// columnWidths returns the display width of the widest cell per column.
func columnWidths(rows [][]string) []int {
	var widths []int
	for _, row := range rows {
		for i, cell := range row {
			if i >= len(widths) {
				widths = append(widths, 0)
			}
			widths[i] = max(widths[i], runewidth.StringWidth(cell))
		}
	}
	return widths
}

// formatRow pads and joins the cells of one row.
func formatRow(row []string, widths []int, align []Align) string {
	var b strings.Builder
	for i, cell := range row {
		if i > 0 {
			b.WriteString(strings.Repeat(" ", columnGap))
		}
		pad := strings.Repeat(" ", widths[i]-runewidth.StringWidth(cell))
		a := Left
		if i < len(align) {
			a = align[i]
		}
		switch {
		case a == Right:
			b.WriteString(pad + cell)
		case i == len(row)-1:
			b.WriteString(cell)
		default:
			b.WriteString(cell + pad)
		}
	}
	return b.String()
}
