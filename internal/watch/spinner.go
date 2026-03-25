package watch

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"golang.org/x/term"
)

type doneMsg struct {
	err error
}

type model struct {
	spinner spinner.Model
	message string
	done    bool
}

func newModel(message string) model {
	s := spinner.New()
	s.Spinner = spinner.Dot

	return model{
		spinner: s,
		message: message,
	}
}

func (m model) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case doneMsg:
		m.done = true
		if msg.err != nil {
			m.message = fmt.Sprintf("CloudFormation apply failed: %v", msg.err)
		} else {
			m.message = "CloudFormation apply completed"
		}
		return m, tea.Quit
	case spinner.TickMsg:
		if m.done {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m model) View() tea.View {
	if m.done {
		return tea.NewView("")
	}

	return tea.NewView(fmt.Sprintf("%s %s", m.spinner.View(), m.message))
}

func Run(ctx context.Context, out io.Writer, message string, operation func(context.Context) error) error {
	if !isInteractive() {
		_, _ = fmt.Fprintf(out, "%s\n", message)
		return operation(ctx)
	}

	program := tea.NewProgram(newModel(message), tea.WithOutput(out))
	errCh := make(chan error, 1)

	go func() {
		err := operation(ctx)
		errCh <- err
		program.Send(doneMsg{err: err})
	}()

	_, runErr := program.Run()
	opErr := <-errCh
	if opErr != nil {
		return opErr
	}
	if runErr != nil {
		return runErr
	}

	return nil
}

func isInteractive() bool {
	if envBool("NUON_NO_TTY") || envBool("NUON_NOTTY") {
		return false
	}

	if _, ok := os.LookupEnv("CI"); ok {
		return false
	}

	return term.IsTerminal(int(os.Stdout.Fd()))
}

func envBool(key string) bool {
	value := os.Getenv(key)
	if value == "" {
		return false
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false
	}

	return parsed
}
