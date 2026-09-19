package render

import (
	"strings"
	"testing"

	"github.com/rivo/uniseg"
)

func TestTableAlignsDisplayColumns(t *testing.T) {
	table := Table{Indent: 2, Gap: 2, Rows: [][]string{
		{"NAME", "STATE"},
		{"日本語", "running"},
		{"e\u0301", "stopped"},
		{"👩‍💻", "running"},
		{"a-name-longer-than-the-old-fixed-width", "stopped"},
	}}
	lines := strings.Split(strings.TrimSuffix(table.String(), "\n"), "\n")
	column := -1
	for i, line := range lines {
		pos := strings.LastIndex(line, table.Rows[i][1])
		width := uniseg.StringWidth(line[:pos])
		if column < 0 {
			column = width
		} else if width != column {
			t.Fatalf("column starts at %d, want %d: %q", width, column, line)
		}
	}
}

func TestTableFitsTerminal(t *testing.T) {
	for width := 1; width <= 80; width++ {
		output := (Table{Width: width, Gap: 2, Rows: [][]string{
			{">", "日本語の長い名前👩‍💻", "ap-northeast-1", "i-0123456789abcdef0", "running", "Online"},
			{"", "web", "us-east-1", "i-1", "stopped", "Offline"},
		}}).String()
		for _, line := range strings.Split(output, "\n") {
			if got := uniseg.StringWidth(line); got > width {
				t.Fatalf("width %d: got %d in %q", width, got, line)
			}
		}
		if !strings.HasPrefix(output, ">") && width > 1 {
			t.Fatalf("selection marker lost: %q", output)
		}
	}
}

func TestTextEscapesTerminalControls(t *testing.T) {
	input := "a\n\t\r\x1b[31m\x7f\u009b"
	want := `a\u000a\u0009\u000d\u001b[31m\u007f\u009b`
	if got := Line(0, 0, input); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got := (Table{Rows: [][]string{{input}}}).String(); got != want+"\n" {
		t.Fatalf("table did not escape cell: %q", got)
	}
}

func TestLinePreservesGraphemes(t *testing.T) {
	for _, tc := range []struct {
		value string
		width int
		want  string
	}{
		{"👩‍💻abc", 3, "👩‍💻…"},
		{"e\u0301abc", 2, "e\u0301…"},
		{"日本語", 4, "日…"},
		{"日本語", 1, "…"},
		{"日本語", 0, "日本語"},
	} {
		if got := Line(tc.width, 0, tc.value); got != tc.want {
			t.Errorf("Line(%d, %q) = %q, want %q", tc.width, tc.value, got, tc.want)
		}
	}
}

func TestTableEmptyAndMissingCells(t *testing.T) {
	if got := (Table{}).String(); got != "" {
		t.Fatalf("empty table: %q", got)
	}
	if got := (Table{Gap: 2, Rows: [][]string{{"A", "B"}, {"x"}}}).String(); got != "A  B\nx\n" {
		t.Fatalf("missing cell: %q", got)
	}
}
