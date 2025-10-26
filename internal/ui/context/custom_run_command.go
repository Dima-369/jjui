package context

import (
	"fmt"
	"os"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/idursun/jjui/internal/config"
	"github.com/idursun/jjui/internal/jj"
	"github.com/idursun/jjui/internal/ui/common"
	"github.com/idursun/jjui/internal/ui/exec_process"
)

type CustomRunCommand struct {
	CustomCommandBase
	Args  []string          `toml:"args"`
	Shell string            `toml:"shell"`
	Show  config.ShowOption `toml:"show"`
}

func (c CustomRunCommand) IsApplicableTo(item SelectedItem) bool {
	var checkSource []string
	if c.Shell != "" {
		checkSource = []string{c.Shell}
	} else {
		checkSource = c.Args
	}

	hasChangeIdPlaceholder := slices.ContainsFunc(checkSource, func(s string) bool { return strings.Contains(s, jj.ChangeIdPlaceholder) })
	hasCommitIdPlaceholder := slices.ContainsFunc(checkSource, func(s string) bool { return strings.Contains(s, jj.CommitIdPlaceholder) })
	hasFilePlaceholder := slices.ContainsFunc(checkSource, func(s string) bool { return strings.Contains(s, jj.FilePlaceholder) })
	hasOperationIdPlaceholder := slices.ContainsFunc(checkSource, func(s string) bool { return strings.Contains(s, jj.OperationIdPlaceholder) })
	if !hasChangeIdPlaceholder && !hasFilePlaceholder && !hasOperationIdPlaceholder && !hasCommitIdPlaceholder {
		// If no placeholders are used, the command is applicable to any item
		return true
	}

	switch item.(type) {
	case SelectedRevision:
		return hasChangeIdPlaceholder || hasCommitIdPlaceholder
	case SelectedFile:
		return hasFilePlaceholder
	case SelectedOperation:
		return hasOperationIdPlaceholder
	default:
		return false
	}
}

func (c CustomRunCommand) Description(ctx *MainContext) string {
	replacements := ctx.CreateReplacements()
	if c.Shell != "" {
		shellCmd := c.Shell
		for k, v := range replacements {
			shellCmd = strings.ReplaceAll(shellCmd, k, v)
		}
		return shellCmd
	}
	args := jj.TemplatedArgs(c.Args, ctx.CreateReplacements())
	return fmt.Sprintf("jj %s", strings.Join(args, " "))
}

func (c CustomRunCommand) Prepare(ctx *MainContext) tea.Cmd {
	replacements := ctx.CreateReplacements()

	if c.Shell != "" {
		shellCmd := c.Shell
		for k, v := range replacements {
			shellCmd = strings.ReplaceAll(shellCmd, k, v)
		}

		switch c.Show {
		case config.ShowOptionDiff:
			return func() tea.Msg {
				output, _ := ctx.RunShellCommandImmediate(shellCmd)
				return common.ShowDiffMsg(output)
			}
		case config.ShowOptionInteractive:
			program := os.Getenv("SHELL")
			if len(program) == 0 {
				program = "sh"
			}
			args := []string{"-c", shellCmd}
			return exec_process.ExecProgram(program, args, ctx.Location, replacements)
		default:
			return ctx.RunShellCommand(shellCmd, common.Refresh)
		}
	}
	switch c.Show {
	case config.ShowOptionDiff:
		return func() tea.Msg {
			output, _ := ctx.RunCommandImmediate(jj.TemplatedArgs(c.Args, replacements))
			return common.ShowDiffMsg(output)
		}
	case config.ShowOptionInteractive:
		return ctx.RunInteractiveCommand(jj.TemplatedArgs(c.Args, replacements), common.Refresh)
	default:
		return ctx.RunCommand(jj.TemplatedArgs(c.Args, replacements), common.Refresh)
	}
}
