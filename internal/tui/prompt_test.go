package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestPromptModelBackspaceAndDelete(t *testing.T) {
	for _, key := range []tea.KeyType{tea.KeyCtrlH, tea.KeyBackspace} {
		m := promptModel{text: splitGraphemes("abcd"), cursor: 3}
		updated, _ := m.Update(tea.KeyMsg{Type: key})
		got := updated.(promptModel)
		if value := strings.Join(got.text, ""); value != "abd" || got.cursor != 2 {
			t.Fatalf("backspace with key %d = %q at %d, want %q at 2", key, value, got.cursor, "abd")
		}
	}

	m := promptModel{text: splitGraphemes("abcd"), cursor: 2}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDelete})
	got := updated.(promptModel)
	if value := strings.Join(got.text, ""); value != "abd" || got.cursor != 2 {
		t.Fatalf("Delete = %q at %d, want %q at 2", value, got.cursor, "abd")
	}
}

func TestPromptModelCursorMovementAndInsertion(t *testing.T) {
	m := promptModel{text: splitGraphemes("abcd"), cursor: 2}
	for _, key := range []tea.KeyMsg{{Type: tea.KeyLeft}, {Type: tea.KeyRunes, Runes: []rune("X")}} {
		updated, _ := m.Update(key)
		m = updated.(promptModel)
	}
	if value := strings.Join(m.text, ""); value != "aXbcd" || m.cursor != 2 {
		t.Fatalf("cursor insertion = %q at %d, want %q at 2", value, m.cursor, "aXbcd")
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyHome})
	m = updated.(promptModel)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDelete})
	m = updated.(promptModel)
	if value := strings.Join(m.text, ""); value != "Xbcd" || m.cursor != 0 {
		t.Fatalf("Home and Delete = %q at %d, want %q at 0", value, m.cursor, "Xbcd")
	}
}

func TestPromptModelEditsWholeGraphemes(t *testing.T) {
	m := promptModel{text: splitGraphemes("A👩‍💻B"), cursor: 2}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	got := updated.(promptModel)
	if value := strings.Join(got.text, ""); value != "AB" || got.cursor != 1 {
		t.Fatalf("backspace split a grapheme: %q at %d", value, got.cursor)
	}

	m = promptModel{}
	for _, key := range []tea.KeyMsg{{Type: tea.KeyRunes, Runes: []rune("e")}, {Type: tea.KeyRunes, Runes: []rune("\u0301")}, {Type: tea.KeyBackspace}} {
		updated, _ := m.Update(key)
		m = updated.(promptModel)
	}
	if value := strings.Join(m.text, ""); value != "" {
		t.Fatalf("separate combining mark input left text after backspace: %q", value)
	}
}

func TestPromptModelEnterAndCancel(t *testing.T) {
	for _, key := range []tea.KeyType{tea.KeyEnter, tea.KeyEsc, tea.KeyCtrlC} {
		m := promptModel{text: splitGraphemes("value")}
		updated, command := m.Update(tea.KeyMsg{Type: key})
		got := updated.(promptModel)
		if !got.done || command == nil {
			t.Fatalf("key %d did not finish prompt", key)
		}
		if got.canceled != (key != tea.KeyEnter) {
			t.Fatalf("key %d canceled = %t", key, got.canceled)
		}
	}
	m := promptModel{}
	updated, command := m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	if got := updated.(promptModel); !got.canceled || command == nil {
		t.Fatal("Ctrl+D on an empty prompt did not cancel")
	}
}

func TestPromptModelViewShowsCaretAtCursor(t *testing.T) {
	m := promptModel{label: "Name", defaultValue: "default", text: splitGraphemes("abcd"), cursor: 2}
	if got, want := m.View(), "Name [default]: ab│cd"; got != want {
		t.Fatalf("View() = %q, want %q", got, want)
	}
}

// 空白のキーイベントを送り、カーソル位置への挿入とフィルタへの追加を検証する。
func TestInputModelsAcceptSpace(t *testing.T) {
	key := tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
	prompt, _ := (promptModel{text: splitGraphemes("key.pem"), cursor: 3}).Update(key)
	if got := strings.Join(prompt.(promptModel).text, ""); got != "key .pem" {
		t.Fatalf("prompt ignored space: %q", got)
	}
	selection, _ := (selectModel{filter: "web"}).Update(key)
	if got := selection.(selectModel).filter; got != "web " {
		t.Fatalf("filter ignored space: %q", got)
	}
}
