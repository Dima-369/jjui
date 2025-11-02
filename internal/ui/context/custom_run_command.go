package context

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
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

// capturingProcess implements tea.ExecCommand to run an interactive command
// while capturing its stdout and stderr.
type capturingProcess struct {
	program  string
	args     []string
	location string
	env      map[string]string

	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer

	outBuf bytes.Buffer
	errBuf bytes.Buffer
}

func (p *capturingProcess) Run() error {
	cmd := exec.Command(p.program, p.args...)
	cmd.Dir = p.location
	cmd.Stdin = p.stdin

	cmd.Stdout = io.MultiWriter(p.stdout, &p.outBuf)
	cmd.Stderr = io.MultiWriter(p.stderr, &p.errBuf)

	var env []string
	for k, v := range p.env {
		name := strings.TrimPrefix(k, "$")
		env = append(env, name+"="+v)
	}
	cmd.Env = append(os.Environ(), env...)

	return cmd.Run()
}

func (p *capturingProcess) SetStdin(r io.Reader)  { p.stdin = r }
func (p *capturingProcess) SetStdout(w io.Writer) { p.stdout = w }
func (p *capturingProcess) SetStderr(w io.Writer) { p.stderr = w }

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
		case config.ShowOptionInteractiveNotification:
			program := os.Getenv("SHELL")
			if len(program) == 0 {
				program = "sh"
			}

			p := &capturingProcess{
				program:  program,
				args:     []string{"-c", shellCmd},
				location: ctx.Location,
				env:      replacements,
			}

			return tea.Batch(
				func() tea.Msg { return common.CommandRunningMsg(shellCmd) },
				tea.Exec(p, func(err error) tea.Msg {
					output := strings.TrimSpace(p.outBuf.String())
					errOutput := strings.TrimSpace(p.errBuf.String())

					finalOutput := output
					if finalOutput == "" {
						finalOutput = errOutput
					} else if errOutput != "" {
						finalOutput += "\n" + errOutput
					}

					if err != nil {
						return common.CommandCompletedMsg{Output: finalOutput, Err: err}
					}

					if finalOutput == "" {
						finalOutput = fmt.Sprintf("'%s' completed", shellCmd)
					}
					return common.CommandCompletedMsg{Output: finalOutput, Err: nil}
				}),
			)
		case config.ShowOptionNotification:
			return ctx.RunShellCommand(shellCmd)
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
	case config.ShowOptionInteractiveNotification:
		args := jj.TemplatedArgs(c.Args, replacements)
		p := &capturingProcess{
			program:  "jj",
			args:     args,
			location: ctx.Location,
			env:      replacements,
		}
		return tea.Batch(
			common.CommandRunning(args),
			tea.Exec(p, func(err error) tea.Msg {
				output := strings.TrimSpace(p.outBuf.String())
				errOutput := strings.TrimSpace(p.errBuf.String())

				finalOutput := output
				if finalOutput == "" {
					finalOutput = errOutput
				} else if errOutput != "" {
					finalOutput += "\n" + errOutput
				}

				if err != nil {
					return common.CommandCompletedMsg{Output: finalOutput, Err: err}
				}
				if finalOutput == "" {
					finalOutput = fmt.Sprintf("'jj %s' completed", strings.Join(args, " "))
				}
				return common.CommandCompletedMsg{Output: finalOutput, Err: nil}
			}),
		)
	case config.ShowOptionNotification:
		return ctx.RunCommand(jj.TemplatedArgs(c.Args, replacements))
	default:
		return ctx.RunCommand(jj.TemplatedArgs(c.Args, replacements), common.Refresh)
	}
}
