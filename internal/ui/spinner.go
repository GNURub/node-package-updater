package ui

import (
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

type spinnerModel struct {
	spinner  spinner.Model
	quitting bool
	message  string
}

func NewSpinner(message string) *spinnerModel {
	return &spinnerModel{
		spinner: spinner.New(),
		message: message,
	}
}

func (m spinnerModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m spinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		// ignore
	}

	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m spinnerModel) View() string {
	if m.quitting {
		return ""
	}
	return m.spinner.View() + " " + m.message
}

// RunSpinner ejecuta el spinner hasta que se cierre el canal done
func RunSpinner(message string, done <-chan struct{}) {
	model := NewSpinner(message)
	p := tea.NewProgram(model)
	go func() {
		<-done
		p.Quit()
	}()
	_, _ = p.Run()
}
