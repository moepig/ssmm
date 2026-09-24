package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rivo/uniseg"
)

type promptModel struct {
	label        string
	defaultValue string
	text         []string
	cursor       int
	done         bool
	canceled     bool
}

// 操作端末で 1 行を編集し、入力値とキャンセル状態を返す。終了時に端末設定を復元する。
func Prompt(ctx context.Context, input *os.File, output io.Writer, label, defaultValue string) (string, bool, error) {
	if input == nil {
		return "", false, fmt.Errorf("prompt input is nil")
	}
	if output == nil {
		output = os.Stdout
	}
	m := promptModel{label: label, defaultValue: defaultValue}
	p := tea.NewProgram(m, tea.WithContext(ctx), tea.WithInput(input), tea.WithOutput(output))
	final, err := p.Run()
	if err != nil {
		if errors.Is(err, tea.ErrProgramKilled) || errors.Is(err, tea.ErrInterrupted) {
			_, _ = fmt.Fprintln(output)
			return "", true, nil
		}
		return "", false, err
	}
	result, ok := final.(promptModel)
	if !ok {
		return "", false, fmt.Errorf("unexpected prompt model %T", final)
	}
	_, _ = fmt.Fprintln(output)
	if result.canceled || !result.done {
		return "", true, nil
	}
	return strings.Join(result.text, ""), false, nil
}

func (m promptModel) Init() tea.Cmd { return nil }

func (m promptModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := message.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.Type {
	case tea.KeyEnter:
		m.done = true
		return m, tea.Quit
	case tea.KeyEsc, tea.KeyCtrlC:
		m.done = true
		m.canceled = true
		return m, tea.Quit
	case tea.KeyLeft, tea.KeyCtrlB:
		if m.cursor > 0 {
			m.cursor--
		}
	case tea.KeyRight, tea.KeyCtrlF:
		if m.cursor < len(m.text) {
			m.cursor++
		}
	case tea.KeyHome, tea.KeyCtrlA:
		m.cursor = 0
	case tea.KeyEnd, tea.KeyCtrlE:
		m.cursor = len(m.text)
	case tea.KeyBackspace, tea.KeyCtrlH:
		if m.cursor > 0 {
			m.cursor--
			m.text = append(m.text[:m.cursor], m.text[m.cursor+1:]...)
		}
	case tea.KeyDelete, tea.KeyCtrlD:
		if key.Type == tea.KeyCtrlD && len(m.text) == 0 {
			m.done = true
			m.canceled = true
			return m, tea.Quit
		}
		if m.cursor < len(m.text) {
			m.text = append(m.text[:m.cursor], m.text[m.cursor+1:]...)
		}
	case tea.KeyRunes, tea.KeySpace:
		inserted := string(key.Runes)
		if inserted != "" {
			before := strings.Join(m.text[:m.cursor], "")
			value := before + inserted + strings.Join(m.text[m.cursor:], "")
			cursorByte := len(before) + len(inserted)
			m.text = splitGraphemes(value)
			m.cursor = graphemeIndexAt(value, cursorByte)
		}
	}
	return m, nil
}

func (m promptModel) View() string {
	prefix := fmt.Sprintf("%s [%s]: ", m.label, m.defaultValue)
	value := strings.Join(m.text, "")
	if m.done {
		return prefix + value
	}
	return prefix + strings.Join(m.text[:m.cursor], "") + "│" + strings.Join(m.text[m.cursor:], "")
}

func splitGraphemes(value string) []string {
	var clusters []string
	graphemes := uniseg.NewGraphemes(value)
	for graphemes.Next() {
		clusters = append(clusters, graphemes.Str())
	}
	return clusters
}

func graphemeIndexAt(value string, byteOffset int) int {
	clusters := splitGraphemes(value)
	index := 0
	offset := 0
	for _, cluster := range clusters {
		offset += len(cluster)
		index++
		if offset >= byteOffset {
			break
		}
	}
	return index
}
