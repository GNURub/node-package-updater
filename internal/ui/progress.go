package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ProgressMsg struct {
	Percentage          float64
	CurrentPackageIndex int
}
type PackageName string

// progressModel representa el modelo de progreso.
type progressModel struct {
	progress    progress.Model
	total       int
	packageName string
	index       int
	done        bool
	width       int
	height      int
}

var (
	quitMessage         = tea.Sequence(tea.ShowCursor, tea.Quit)
	currentPkgNameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("211"))
	style               = lipgloss.NewStyle().Margin(1, 2)
)

// NewProgress inicializa un nuevo modelo de progreso.
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
		percentage := msg.Percentage
		m.index = msg.CurrentPackageIndex

		if percentage >= 1.0 {
			m.done = true

			return m, quitMessage
		}

		cmd := m.progress.SetPercent(percentage)
		return m, cmd

	case PackageName:
		m.packageName = string(msg)

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
	if m.done {
		return style.Render("🎉! All dependencies updated successfully!\n")
	}

	var s strings.Builder
	w := lipgloss.Width(fmt.Sprintf("%d", m.total))

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

// ShowProgressBar inicia y muestra la barra de progreso.
func ShowProgressBar(total int) (*tea.Program, error) {
	model := NewProgress(total)
	program := tea.NewProgram(model)

	// go func() {
	// 	if _, err := program.Run(); err != nil {
	// 		fmt.Println("Error starting program:", err)
	// 	}
	// }()

	return program, nil
}
