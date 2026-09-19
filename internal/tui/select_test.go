package tui

import (
	"strings"
	"testing"
	"time"

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
