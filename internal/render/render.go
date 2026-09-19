package render

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/rivo/uniseg"
)

type Table struct {
	Rows   [][]string
	Indent int
	Gap    int
	Width  int
}

func escape(value string) string {
	var b strings.Builder
	for _, r := range value {
		if unicode.IsControl(r) {
			fmt.Fprintf(&b, "\\u%04x", r)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func Line(width, gap int, parts ...string) string {
	values := make([]string, len(parts))
	for i, part := range parts {
		values[i] = escape(part)
	}
	line := strings.Join(values, strings.Repeat(" ", max(0, gap)))
	if width > 0 {
		return truncate(line, width)
	}
	return line
}

func (t Table) String() string {
	columns := 0
	for _, row := range t.Rows {
		columns = max(columns, len(row))
	}
	if columns == 0 {
		return ""
	}
	widths := make([]int, columns)
	rows := make([][]string, len(t.Rows))
	for i, row := range t.Rows {
		rows[i] = make([]string, columns)
		for j, cell := range row {
			rows[i][j] = escape(cell)
			widths[j] = max(widths[j], uniseg.StringWidth(rows[i][j]))
		}
	}
	indent, gap := max(0, t.Indent), max(0, t.Gap)
	total := indent + gap*(columns-1)
	for _, width := range widths {
		total += width
	}
	if t.Width > 0 {
		for total > t.Width {
			widest := 0
			for j := range widths {
				if widths[j] > widths[widest] {
					widest = j
				}
			}
			if widths[widest] <= 1 {
				break
			}
			widths[widest]--
			total--
		}
	}
	var b strings.Builder
	for _, row := range rows {
		var line strings.Builder
		line.WriteString(strings.Repeat(" ", indent))
		for j, cell := range row {
			cell = truncate(cell, widths[j])
			line.WriteString(cell)
			if j+1 < columns {
				line.WriteString(strings.Repeat(" ", widths[j]-uniseg.StringWidth(cell)+gap))
			}
		}
		value := strings.TrimRight(line.String(), " ")
		if t.Width > 0 {
			value = truncate(value, t.Width)
		}
		b.WriteString(value)
		b.WriteByte('\n')
	}
	return b.String()
}

func truncate(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if uniseg.StringWidth(value) <= width {
		return value
	}
	var b strings.Builder
	g := uniseg.NewGraphemes(value)
	used := 0
	for g.Next() {
		if used+g.Width() > width-1 {
			break
		}
		b.WriteString(g.Str())
		used += g.Width()
	}
	b.WriteRune('…')
	return b.String()
}
