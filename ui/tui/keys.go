package tui

import "github.com/charmbracelet/bubbles/key"

// keyMap defines all keyboard bindings for the TUI.
type keyMap struct {
	Quit     key.Binding
	Tab      key.Binding
	Up       key.Binding
	Down     key.Binding
	Filter   key.Binding
	Sort     key.Binding
	ToggleIO key.Binding
	Pause    key.Binding
	Help     key.Binding
}

var defaultKeys = keyMap{
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "switch panel"),
	),
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "move up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "move down"),
	),
	Filter: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "filter"),
	),
	Sort: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "cycle sort"),
	),
	ToggleIO: key.NewBinding(
		key.WithKeys("f"),
		key.WithHelp("f", "toggle file I/O"),
	),
	Pause: key.NewBinding(
		key.WithKeys("p", "F5"),
		key.WithHelp("p", "pause/resume"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
}
