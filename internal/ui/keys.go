package ui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines all keybindings for the application
type KeyMap struct {
	Up           key.Binding
	Down         key.Binding
	Enter        key.Binding
	Back         key.Binding
	Quit         key.Binding
	Help         key.Binding
	Filter       key.Binding
	Create       key.Binding
	Delete       key.Binding
	Reveal       key.Binding
	AddVersion   key.Binding
	Generate     key.Binding
	Refresh      key.Binding
	Config       key.Binding
	Copy         key.Binding
	Tab          key.Binding
	Escape       key.Binding
	Confirm      key.Binding
	Cancel       key.Binding
}

// DefaultKeyMap returns the default keybindings
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("Enter", "select"),
		),
		Back: key.NewBinding(
			key.WithKeys("backspace", "esc"),
			key.WithHelp("Esc", "back"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
		),
		Create: key.NewBinding(
			key.WithKeys("n", "c"),
			key.WithHelp("n", "new secret"),
		),
		Delete: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "delete"),
		),
		Reveal: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "reveal"),
		),
		AddVersion: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "add version"),
		),
		Generate: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", "generate code"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("ctrl+r"),
			key.WithHelp("^R", "refresh"),
		),
		Config: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("^S", "settings"),
		),
		Copy: key.NewBinding(
			key.WithKeys("y"),
			key.WithHelp("y", "copy"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("Tab", "next"),
		),
		Escape: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("Esc", "cancel"),
		),
		Confirm: key.NewBinding(
			key.WithKeys("y"),
			key.WithHelp("y", "yes"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("n", "esc"),
			key.WithHelp("n", "no"),
		),
	}
}

// FooterBinding represents a key binding shown in the footer
type FooterBinding struct {
	Key  string
	Desc string
}

// ListViewBindings returns the keybindings for the list view
func ListViewBindings() []FooterBinding {
	return []FooterBinding{
		{Key: "↑↓/jk", Desc: "navigate"},
		{Key: "g/G", Desc: "top/bottom"},
		{Key: "Enter/l", Desc: "open"},
		{Key: "Esc/h", Desc: "back"},
		{Key: "/", Desc: "filter"},
		{Key: "n", Desc: "new"},
		{Key: "d", Desc: "delete"},
		{Key: "^R", Desc: "refresh"},
		{Key: "^P", Desc: "project"},
		{Key: "q", Desc: "quit"},
	}
}

// DetailViewBindings returns the keybindings for the detail view
func DetailViewBindings() []FooterBinding {
	return []FooterBinding{
		{Key: "↑↓/jk", Desc: "versions"},
		{Key: "r", Desc: "reveal"},
		{Key: "c", Desc: "copy"},
		{Key: "a", Desc: "add version"},
		{Key: "g", Desc: "generate"},
		{Key: "d", Desc: "delete"},
		{Key: "Esc/h", Desc: "back"},
		{Key: "^P", Desc: "project"},
		{Key: "q", Desc: "quit"},
	}
}

// InputViewBindings returns the keybindings for input views
func InputViewBindings() []FooterBinding {
	return []FooterBinding{
		{Key: "Tab/↑↓", Desc: "fields"},
		{Key: "Enter", Desc: "submit"},
		{Key: "Esc", Desc: "cancel"},
	}
}

// ConfirmViewBindings returns the keybindings for confirm dialogs
func ConfirmViewBindings() []FooterBinding {
	return []FooterBinding{
		{Key: "y", Desc: "confirm"},
		{Key: "n/Esc", Desc: "cancel"},
	}
}

// GenerateViewBindings returns the keybindings for the generate code view
func GenerateViewBindings() []FooterBinding {
	return []FooterBinding{
		{Key: "↑↓/jk", Desc: "templates"},
		{Key: "Enter", Desc: "generate"},
		{Key: "Esc/h", Desc: "back"},
		{Key: "^P", Desc: "project"},
		{Key: "q", Desc: "quit"},
	}
}

// ConfigMenuBindings returns the keybindings for the config menu
func ConfigMenuBindings() []FooterBinding {
	return []FooterBinding{
		{Key: "↑↓/jk", Desc: "navigate"},
		{Key: "Enter/l", Desc: "select"},
		{Key: "Esc", Desc: "back"},
		{Key: "^P", Desc: "project"},
		{Key: "q", Desc: "quit"},
	}
}

// ConfigTemplatesBindings returns the keybindings for template list
func ConfigTemplatesBindings() []FooterBinding {
	return []FooterBinding{
		{Key: "↑↓/jk", Desc: "navigate"},
		{Key: "Enter/e", Desc: "edit"},
		{Key: "n", Desc: "new"},
		{Key: "d", Desc: "delete"},
		{Key: "Esc/h", Desc: "back"},
		{Key: "^P", Desc: "project"},
	}
}

// ConfigTemplateEditBindings returns the keybindings for template editing
func ConfigTemplateEditBindings() []FooterBinding {
	return []FooterBinding{
		{Key: "Tab", Desc: "fields"},
		{Key: "^S", Desc: "save"},
		{Key: "Esc", Desc: "cancel"},
	}
}

// ConfigRecentBindings returns the keybindings for recent projects
func ConfigRecentBindings() []FooterBinding {
	return []FooterBinding{
		{Key: "↑↓/jk", Desc: "navigate"},
		{Key: "Enter", Desc: "switch"},
		{Key: "d", Desc: "remove"},
		{Key: "Esc/h", Desc: "back"},
	}
}

// ProjectSwitchBindings returns the keybindings for quick project switch
func ProjectSwitchBindings() []FooterBinding {
	return []FooterBinding{
		{Key: "↑↓", Desc: "select"},
		{Key: "Enter", Desc: "switch/add"},
		{Key: "Esc", Desc: "cancel"},
	}
}

// RevealViewBindings returns the keybindings for the reveal view
func RevealViewBindings() []FooterBinding {
	return []FooterBinding{
		{Key: "c", Desc: "copy"},
		{Key: "Esc/r", Desc: "hide"},
		{Key: "^P", Desc: "project"},
		{Key: "q", Desc: "quit"},
	}
}

