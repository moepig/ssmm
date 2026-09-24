package tui

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rivo/uniseg"
	"github.com/uncho/ssmm/internal/inventory"
	"github.com/uncho/ssmm/internal/target"
)

func TestViewResizesAndEscapesCells(t *testing.T) {
	name := "日本語\x1b[31m\nweb"
	m := selectModel{snapshot: inventory.InventorySnapshot{Instances: []target.Instance{{
		Key:  target.InstanceKey{Region: "ap-northeast-1", InstanceID: "i-0123456789abcdef0"},
		Name: &name, EC2State: "running", SSMStatus: target.SSMOnline,
	}}}}
	view := m.View()
	if strings.Contains(view, "\x1b") || !strings.Contains(view, `\u001b[31m\u000aweb`) {
		t.Fatalf("unsafe cell rendering: %q", view)
	}
	for _, width := range []int{20, 80, 10} {
		updated, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: 24})
		m = updated.(selectModel)
		for _, line := range strings.Split(m.View(), "\n") {
			if uniseg.StringWidth(line) > width {
				t.Fatalf("line exceeds terminal width %d: %q", width, line)
			}
		}
	}
}

func TestSelectModelConsumesIncrementalSnapshotsAndRequestsRefresh(t *testing.T) {
	name := "web"
	done := make(chan SelectResult, 1)
	m := selectModel{snapshot: inventory.InventorySnapshot{Generation: 3}, done: done, filter: "web"}
	updated, command := m.Update(snapshotMessage(inventory.InventorySnapshot{
		Generation: 3,
		Revision:   2,
		Instances:  []target.Instance{{Key: target.InstanceKey{Region: "us-east-1", InstanceID: "i-01234567"}, Name: &name, EC2State: "running"}},
	}))
	if command == nil {
		t.Fatal("incremental snapshot did not schedule the next snapshot")
	}
	m = updated.(selectModel)
	if len(m.snapshot.Instances) != 1 || m.snapshot.Generation != 3 {
		t.Fatalf("snapshot was not applied: %#v", m.snapshot)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	_ = updated
	select {
	case result := <-done:
		if !result.Refresh || result.Generation != 3 || result.Filter != "web" {
			t.Fatalf("unexpected refresh result: %#v", result)
		}
	case <-time.After(time.Second):
		t.Fatal("refresh action was not emitted")
	}
}

func TestSelectModelKeepsFocusedKeyWhenRowsAreInserted(t *testing.T) {
	focus := target.InstanceKey{Region: "us-east-1", InstanceID: "i-c"}
	done := make(chan SelectResult, 1)
	m := selectModel{focus: &focus, done: done}
	updated, _ := m.Update(snapshotMessage(inventory.InventorySnapshot{Generation: 4, Instances: []target.Instance{
		{Key: target.InstanceKey{Region: "us-east-1", InstanceID: "i-a"}, EC2State: "running"},
		{Key: target.InstanceKey{Region: "us-east-1", InstanceID: "i-b"}, EC2State: "running"},
		{Key: focus, EC2State: "running"},
	}}))
	m = updated.(selectModel)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_ = updated
	select {
	case result := <-done:
		if !result.HasKey || result.Key != focus {
			t.Fatalf("selected key = %#v, want %v", result, focus)
		}
	default:
		t.Fatal("Enter did not select the focused key")
	}
}

func TestSelectModelRetainsMissingFocusForRefresh(t *testing.T) {
	focus := target.InstanceKey{Region: "us-east-1", InstanceID: "i-c"}
	done := make(chan SelectResult, 1)
	m := selectModel{focus: &focus, done: done}
	updated, _ := m.Update(snapshotMessage(inventory.InventorySnapshot{Generation: 5, Instances: []target.Instance{
		{Key: target.InstanceKey{Region: "us-east-1", InstanceID: "i-a"}, EC2State: "running"},
	}}))
	m = updated.(selectModel)
	if strings.Contains(m.View(), ">") {
		t.Fatal("missing focus was displayed as a selected row")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	select {
	case result := <-done:
		if !result.Refresh || !result.HasKey || result.Key != focus {
			t.Fatalf("refresh did not retain missing focus: %#v", result)
		}
	default:
		t.Fatal("refresh action was not emitted")
	}
}

func TestSelectModelBackspaceRemovesLastGrapheme(t *testing.T) {
	tests := []struct {
		name  string
		key   tea.KeyType
		input string
		want  string
	}{
		{name: "DEL", key: tea.KeyBackspace, input: "web", want: "we"},
		{name: "BS", key: tea.KeyCtrlH, input: "web", want: "we"},
		{name: "multibyte rune", key: tea.KeyBackspace, input: "東京", want: "東"},
		{name: "combining grapheme", key: tea.KeyCtrlH, input: "e\u0301x", want: "e\u0301"},
		{name: "combining grapheme only", key: tea.KeyBackspace, input: "e\u0301", want: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			m := selectModel{filter: test.input}
			updated, _ := m.Update(tea.KeyMsg{Type: test.key})
			got := updated.(selectModel).filter
			if got != test.want {
				t.Fatalf("filter after backspace = %q, want %q", got, test.want)
			}
			if !utf8.ValidString(got) {
				t.Fatalf("filter after backspace is invalid UTF-8: %q", got)
			}
		})
	}
}
