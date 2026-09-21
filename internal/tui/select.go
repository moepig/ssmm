package tui

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/uncho/ssmm/internal/inventory"
	"github.com/uncho/ssmm/internal/render"
	"github.com/uncho/ssmm/internal/target"
)

type SelectResult struct {
	Key        target.InstanceKey
	HasKey     bool
	Canceled   bool
	Refresh    bool
	Filter     string
	Generation uint64
	Snapshot   inventory.InventorySnapshot
}

type selectModel struct {
	ctx       context.Context
	snapshot  inventory.InventorySnapshot
	filter    string
	focus     *target.InstanceKey
	width     int
	snapshots <-chan inventory.InventorySnapshot
	input     io.Reader
	output    io.Writer
	done      chan SelectResult
}

func Select(ctx context.Context, input *os.File, output io.Writer, snapshot inventory.InventorySnapshot, filter string) (SelectResult, error) {
	snapshots := make(chan inventory.InventorySnapshot, 1)
	snapshots <- snapshot
	close(snapshots)
	return SelectLive(ctx, input, output, snapshots, filter, nil)
}

// SelectLive displays snapshots as the inventory aggregator publishes them.
// It returns Refresh for Ctrl+R so the caller can finish the current
// generation before starting the next one.
func SelectLive(ctx context.Context, input *os.File, output io.Writer, snapshots <-chan inventory.InventorySnapshot, filter string, focus *target.InstanceKey) (SelectResult, error) {
	if input == nil {
		return SelectResult{}, fmt.Errorf("selection terminal is nil")
	}
	if output == nil {
		output = os.Stdout
	}
	done := make(chan SelectResult, 1)
	programDone := make(chan struct{})
	var initialFocus *target.InstanceKey
	if focus != nil {
		key := *focus
		initialFocus = &key
	}
	m := selectModel{ctx: ctx, filter: filter, focus: initialFocus, snapshots: snapshots, input: input, output: output, done: done}
	p := tea.NewProgram(m, tea.WithInput(input), tea.WithOutput(output))
	go func() {
		select {
		case <-ctx.Done():
			p.Send(tea.KeyMsg{Type: tea.KeyEsc})
		case <-programDone:
		}
	}()
	finalModel, err := p.Run()
	if err != nil {
		close(programDone)
		return SelectResult{}, err
	}
	if current, ok := finalModel.(selectModel); ok {
		m = current
	}
	close(programDone)
	select {
	case result := <-done:
		return result, nil
	default:
		return SelectResult{Canceled: true, Filter: m.filter, Snapshot: m.snapshot, Generation: m.snapshot.Generation}, nil
	}
}

type snapshotMessage inventory.InventorySnapshot

type snapshotsClosed struct{}

func nextSnapshot(snapshots <-chan inventory.InventorySnapshot) tea.Cmd {
	return func() tea.Msg {
		snapshot, ok := <-snapshots
		if !ok {
			return snapshotsClosed{}
		}
		return snapshotMessage(snapshot)
	}
}

func (m selectModel) Init() tea.Cmd {
	if m.snapshots == nil {
		return nil
	}
	return nextSnapshot(m.snapshots)
}

func (m selectModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	if size, ok := message.(tea.WindowSizeMsg); ok {
		m.width = size.Width
		return m, nil
	}
	if snapshot, ok := message.(snapshotMessage); ok {
		m.snapshot = inventory.InventorySnapshot(snapshot).Clone()
		if m.focus == nil {
			rows := target.Filter(m.snapshot.Instances, m.filter)
			if len(rows) > 0 {
				key := rows[0].Key
				m.focus = &key
			}
		}
		return m, nextSnapshot(m.snapshots)
	}
	if _, ok := message.(snapshotsClosed); ok {
		m.snapshots = nil
		return m, nil
	}
	if key, ok := message.(tea.KeyMsg); ok {
		switch key.Type {
		case tea.KeyEsc, tea.KeyCtrlC:
			m.done <- SelectResult{Canceled: true, Filter: m.filter, Generation: m.snapshot.Generation, Snapshot: m.snapshot.Clone()}
			return m, tea.Quit
		case tea.KeyEnter:
			rows := target.Filter(m.snapshot.Instances, m.filter)
			row, ok := focusedRow(rows, m.focus)
			if !ok {
				return m, nil
			}
			if !row.Running() {
				return m, nil
			}
			m.done <- SelectResult{Key: row.Key, HasKey: true, Filter: m.filter, Generation: m.snapshot.Generation, Snapshot: m.snapshot.Clone()}
			return m, tea.Quit
		case tea.KeyUp:
			m.moveFocus(-1)
			return m, nil
		case tea.KeyDown:
			m.moveFocus(1)
			return m, nil
		case tea.KeyBackspace:
			if len(m.filter) > 0 {
				m.filter = m.filter[:len(m.filter)-1]
				m.resetFocus()
			}
			return m, nil
		case tea.KeyRunes:
			m.filter += string(key.Runes)
			m.resetFocus()
			return m, nil
		case tea.KeyCtrlR:
			result := SelectResult{Refresh: true, Filter: m.filter, Generation: m.snapshot.Generation, Snapshot: m.snapshot.Clone()}
			if m.focus != nil {
				result.Key = *m.focus
				result.HasKey = true
			}
			m.done <- result
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m selectModel) View() string {
	rows := target.Filter(m.snapshot.Instances, m.filter)
	var b strings.Builder
	b.WriteString(render.Line(m.width, 0, "Profile selection") + "\n")
	b.WriteString(render.Line(m.width, 0, "Filter: "+m.filter) + "\n\n")
	table := render.Table{Gap: 3, Width: m.width}
	for _, row := range rows {
		marker := ""
		if m.focus != nil && row.Key == *m.focus {
			marker = ">"
		}
		name := "-"
		if row.Name != nil {
			name = *row.Name
		}
		table.Rows = append(table.Rows, []string{marker, name, row.Key.Region, row.Key.InstanceID, row.EC2State, string(row.SSMStatus)})
	}
	b.WriteString(table.String())
	progress := "searching"
	if m.snapshot.Finished {
		progress = "search complete"
	}
	b.WriteString("\n" + render.Line(m.width, 2, progress, "↑↓ select", "Enter connect", "Ctrl+R refresh", "Esc cancel") + "\n")
	return b.String()
}

func focusedRow(rows []target.Instance, focus *target.InstanceKey) (target.Instance, bool) {
	if focus == nil {
		return target.Instance{}, false
	}
	for _, row := range rows {
		if row.Key == *focus {
			return row, true
		}
	}
	return target.Instance{}, false
}

func (m *selectModel) resetFocus() {
	rows := target.Filter(m.snapshot.Instances, m.filter)
	if len(rows) == 0 {
		m.focus = nil
		return
	}
	key := rows[0].Key
	m.focus = &key
}

func (m *selectModel) moveFocus(step int) {
	rows := target.Filter(m.snapshot.Instances, m.filter)
	if len(rows) == 0 {
		m.focus = nil
		return
	}
	if m.focus == nil {
		key := rows[0].Key
		if step < 0 {
			key = rows[len(rows)-1].Key
		}
		m.focus = &key
		return
	}
	position := -1
	for i, row := range rows {
		if row.Key == *m.focus {
			position = i
			break
		}
	}
	if position < 0 {
		key := rows[0].Key
		if step < 0 {
			key = rows[len(rows)-1].Key
		}
		m.focus = &key
		return
	}
	next := position + step
	if next < 0 {
		next = 0
	}
	if next >= len(rows) {
		next = len(rows) - 1
	}
	key := rows[next].Key
	m.focus = &key
}
