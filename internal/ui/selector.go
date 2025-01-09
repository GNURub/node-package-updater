package ui

import (
	"fmt"
	"strings"

	"github.com/GNURub/node-package-updater/internal/dependency"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type model struct {
	dependencies dependency.Dependencies
	selected     map[int]struct{}
	quitting     bool
	done         bool
	table        table.Model
}

var (
	baseStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#63B0B8"))
)

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.table.Focused() {
				m.table.Blur()
			} else {
				m.table.Focus()
			}
		case "ctrl+a":
			for i := 0; i < len(m.table.Rows()); i++ {
				m.selected[i] = struct{}{}
			}
			return m, nil
		case "q", "ctrl+c":
			m.quitting = true

			m.selected = make(map[int]struct{})

			return m, tea.Sequence(
				tea.ClearScreen,
				tea.Quit,
			)
		case "enter":
			m.done = true
			return m, tea.Sequence(
				tea.ClearScreen,
				tea.Quit,
			)
		case " ":
			cursor := m.table.Cursor()
			_, ok := m.selected[cursor]
			if ok {
				delete(m.selected, cursor)
			} else {
				m.selected[cursor] = struct{}{}
			}
		}
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m model) View() string {
	var s strings.Builder
	if m.quitting || m.done {
		s.WriteString("")
		return s.String()
	}

	rows := m.table.Rows()
	for i := range rows {
		if _, ok := m.selected[i]; ok {
			rows[i][0] = "✔"
		} else {
			rows[i][0] = ""
		}
	}

	m.table.SetRows(rows)

	s.WriteString(baseStyle.Render(m.table.View()) + "\n\n")

	s.WriteString(lipgloss.NewStyle().MarginLeft(2).Render(
		"↑/↓: navigate • space: select • enter: confirm • q: quit\n",
	))

	return s.String()
}

func SelectDependencies(deps map[string]dependency.Dependencies) (map[string]dependency.Dependencies, error) {
	var dependencies dependency.Dependencies

	// Add regular dependencies
	for _, envDeps := range deps {
		for _, dep := range envDeps {
			if dep.NextVersion != "" {
				// Create a copy of the dependency
				depCopy := dep
				depCopy.HaveToUpdate = false // Reset update flag
				dependencies = append(dependencies, depCopy)
			}
		}
	}

	// Configure table columns
	columns := []table.Column{
		{Title: "", Width: 2},
		{Title: "Dependency", Width: 30},
		{Title: "Current Version", Width: 15},
		{Title: "New Version", Width: 15},
		{Title: "Type", Width: 15},
	}

	var rows []table.Row
	for _, dep := range dependencies {
		rows = append(rows, table.Row{
			"",
			dep.PackageName,
			dep.CurrentVersion,
			dep.NextVersion,
			dep.Env,
		})
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(7),
	)

	// Table styles
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#63B0B8")).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("#ffffff")).
		Background(lipgloss.Color("#FF75B7")).
		Bold(true)
	t.SetStyles(s)

	initialModel := model{
		dependencies: dependencies,
		selected:     make(map[int]struct{}),
		table:        t,
	}

	p := tea.NewProgram(initialModel)
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("error running bubbletea program: %w", err)
	}

	// Get selected dependencies
	m := finalModel.(model)
	if m.quitting {
		return nil, fmt.Errorf("selection cancelled by user")
	}

	// Update the selected dependencies
	for idx := range m.selected {
		selectedDep := m.dependencies[idx]

		for i, dep := range deps[selectedDep.Env] {
			if dep.PackageName == selectedDep.PackageName {
				deps[selectedDep.Env][i].HaveToUpdate = true
				break
			}
		}
	}

	return deps, nil
}
