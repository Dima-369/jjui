package diff

import (
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/idursun/jjui/internal/config"
	"github.com/idursun/jjui/internal/jj"
	"github.com/idursun/jjui/internal/ui/context"
	"github.com/idursun/jjui/internal/ui/common"
)

type Model struct {
	view      viewport.Model
	keymap    config.KeyMappings[key.Binding]
	output    string
	revision  string
	context   *context.MainContext
}

func (m *Model) ShortHelp() []key.Binding {
	return []key.Binding{
		m.keymap.DiffMode.Up, m.keymap.DiffMode.Down, m.keymap.DiffMode.HalfPageDown, m.keymap.DiffMode.HalfPageUp, m.keymap.DiffMode.PageDown, m.keymap.DiffMode.PageUp,
		m.keymap.CopyGitDiff,
		m.keymap.Cancel}
}

func (m *Model) FullHelp() [][]key.Binding {
	return [][]key.Binding{m.ShortHelp()}
}

func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) SetHeight(h int) {
	m.view.Height = h
}

func (m *Model) Update(msg tea.Msg) (*Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keymap.Cancel):
			return m, common.Close
		case key.Matches(msg, m.keymap.CopyGitDiff):
			return m, func() tea.Msg {
				if m.context != nil && m.revision != "" {
					gitDiffOutput, err := m.context.RunCommandImmediate(jj.DiffGitUncolored(m.revision))
					if err != nil {
						return common.CommandCompletedMsg{Err: err, Output: "Error running jj diff --git command"}
					}
					err = clipboard.WriteAll(string(gitDiffOutput))
					if err != nil {
						return common.CommandCompletedMsg{Err: err}
					}
					return common.CommandCompletedMsg{Output: "Copied 'jj diff --git' output to clipboard"}
				} else {
					err := clipboard.WriteAll(m.output)
					if err != nil {
						return common.CommandCompletedMsg{Err: err}
					}
					return common.CommandCompletedMsg{Output: "Copied git diff to clipboard"}
				}
			}
		case key.Matches(msg, m.keymap.DiffMode.Top):
			m.view.GotoTop()
			return m, nil
		case key.Matches(msg, m.keymap.DiffMode.Bottom):
			m.view.GotoBottom()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.view, cmd = m.view.Update(msg)
	return m, cmd
}

func (m *Model) View() string {
	return m.view.View()
}

func New(output string, width int, height int, ctx *context.MainContext, revision string) *Model {
	view := viewport.New(width, height)
	keymap := config.Current.GetKeyMap()

	view.KeyMap.Up = keymap.DiffMode.Up
	view.KeyMap.Down = keymap.DiffMode.Down
	view.KeyMap.PageUp = keymap.DiffMode.PageUp
	view.KeyMap.PageDown = keymap.DiffMode.PageDown
	view.KeyMap.HalfPageUp = keymap.DiffMode.HalfPageUp
	view.KeyMap.HalfPageDown = keymap.DiffMode.HalfPageDown

	content := strings.ReplaceAll(output, "\r", "")
	if content == "" {
		content = "(empty)"
	}
	view.SetContent(content)
	return &Model{
		view:     view,
		keymap:   keymap,
		output:   output,
		context:  ctx,
		revision: revision,
	}
}
