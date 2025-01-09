package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ProgressMsg float64

type progressModel struct {
	progress    progress.Model
	total       int
	packageName string
	err         error
	index       int
	width       int
	height      int
	done        bool
}

var (
	currentPkgNameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("211"))
	doneStyle           = lipgloss.NewStyle().Margin(1, 2)
)

func NewProgress(total int) *progressModel {
	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(40),
		progress.WithoutPercentage(),
	)

	return &progressModel{
		progress: p,
		total:    total,
		width:    40,
	}
}

func (m progressModel) Init() tea.Cmd {
	return nil
}

func (m progressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case ProgressMsg:
		percentage := float64(msg)
		if percentage >= 100 {
			m.done = true
			return m, tea.Sequence(
				m.progress.SetPercent(100),
				tea.ClearScreen,
				tea.Quit, // exit the program
			)
		}

		cmd := m.progress.SetPercent(percentage)
		return m, cmd

	case error:
		m.err = msg
		return m, tea.Quit

	case string:
		m.packageName = msg

	case int:
		m.index = msg

	case progress.FrameMsg:
		newModel, cmd := m.progress.Update(msg)
		if newModel, ok := newModel.(progress.Model); ok {
			m.progress = newModel
		}
		return m, cmd
	}

	return m, nil
}

func (m progressModel) View() string {
	w := lipgloss.Width(fmt.Sprintf("%d", m.total))

	var s strings.Builder

	style := lipgloss.NewStyle().Margin(1, 2)

	if m.done {
		s.WriteString("")
		return s.String()
	}

	if m.packageName != "" {
		pkgName := currentPkgNameStyle.Render(m.packageName)
		s.WriteString(fmt.Sprintf("Fetching: %s\n", pkgName))
	}

	pad := strings.Repeat(" ", 2)
	prog := m.progress.View()
	pkgCount := fmt.Sprintf(" %*d/%*d", w, m.index, w, m.total)

	s.WriteString(pad + prog + pkgCount + "\n")

	return style.Render(s.String())
}

func ShowProgressBar(total int) *tea.Program {
	model := NewProgress(total)
	program := tea.NewProgram(model)

	go func() {
		if _, err := program.Run(); err != nil {
			fmt.Printf("Error running progress: %v", err)
		}
	}()

	return program
}
