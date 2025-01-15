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
	dependencies              dependency.Dependencies
	selected                  map[int]struct{}
	showVersionsForDependency *dependency.Dependency
	quitting                  bool
	done                      bool
	dependencyTable           table.Model
	versionsTable             table.Model
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
			if m.dependencyTable.Focused() {
				m.dependencyTable.Blur()
			} else {
				m.dependencyTable.Focus()
			}
		case "ctrl+a":
			if m.dependencyTable.Focused() {
				for i := range m.dependencies {
					m.selected[i] = struct{}{}
				}
			}
		case "ctrl+u":
			if m.dependencyTable.Focused() {
				for i := range m.dependencies {
					delete(m.selected, i)
				}
			}
		case "ctrl+d":
			if m.dependencyTable.Focused() {
				for i, dep := range m.dependencies {
					if dep.Env == "dev" {
						m.selected[i] = struct{}{}
					}
				}
			}
		case "ctrl+p":
			if m.dependencyTable.Focused() {
				for i, dep := range m.dependencies {
					if dep.Env == "prod" {
						m.selected[i] = struct{}{}
					}
				}
			}
		case "q", "ctrl+c":
			m.quitting = true
			m.selected = make(map[int]struct{})
			return m, tea.Sequence(
				tea.ClearScreen,
				tea.Quit,
			)
		case "enter":
			if m.showVersionsForDependency != nil {
				return m, nil
			}

			m.done = true
			return m, tea.Sequence(
				tea.ClearScreen,
				tea.Quit,
			)
		case "right", "l":
			if m.dependencyTable.Focused() {
				cursor := m.dependencyTable.Cursor()
				m.showVersionsForDependency = m.dependencies[cursor]

				// Actualizar las filas de la tabla de versiones
				var rows []table.Row
				for _, v := range m.showVersionsForDependency.Versions {
					rows = append(rows, table.Row{m.showVersionsForDependency.PackageName, v})
				}
				m.versionsTable.SetRows(rows)

				m.versionsTable.Focus()
				m.dependencyTable.Blur()
			}
			return m, nil

		case "left", "h":
			if m.versionsTable.Focused() {
				m.showVersionsForDependency = nil
				m.dependencyTable.Focus()
				m.versionsTable.Blur()
			}
			return m, nil

		case " ":
			if m.showVersionsForDependency != nil {
				cursorDep := m.dependencyTable.Cursor()
				cursor := m.versionsTable.Cursor()

				m.dependencies[cursorDep].NextVersion = m.showVersionsForDependency.Versions[cursor]

				m.showVersionsForDependency = nil
				m.dependencyTable.Focus()
				m.versionsTable.Blur()

				return m, nil
			}
			if m.dependencyTable.Focused() {
				cursor := m.dependencyTable.Cursor()
				if _, ok := m.selected[cursor]; ok {
					delete(m.selected, cursor)
				} else {
					m.selected[cursor] = struct{}{}
				}
			}
			return m, nil
		}
	}

	// Actualizar la tabla enfocada
	if m.versionsTable.Focused() {
		m.versionsTable, cmd = m.versionsTable.Update(msg)
	} else {
		m.dependencyTable, cmd = m.dependencyTable.Update(msg)
	}

	return m, cmd
}

func (m model) View() string {
	var s strings.Builder
	if m.quitting || m.done {
		s.WriteString("")
		return s.String()
	}

	if m.showVersionsForDependency != nil {
		s.WriteString(baseStyle.Render(m.versionsTable.View()) + "\n\n")
		s.WriteString(lipgloss.NewStyle().MarginLeft(2).Render("←/h: back • ↑/↓: navigate • space: select\n"))
		return s.String()
	}

	var rows []table.Row

	for i, dep := range m.dependencies {
		var selected string
		if _, ok := m.selected[i]; ok {
			selected = "✔"
		} else {
			selected = " "
		}

		rows = append(rows, table.Row{
			selected,
			dep.PackageName,
			dep.CurrentVersion,
			dep.NextVersion,
			dep.Env,
		})
	}

	m.dependencyTable.SetRows(rows)

	s.WriteString(baseStyle.Render(m.dependencyTable.View()) + "\n\n")

	s.WriteString(lipgloss.NewStyle().MarginLeft(2).Render(
		"↑/↓: navigate • space: select • enter: select • ctrl+a: select all • ctrl+p: select only prod • ctrl+d: select only dev • ctrl+u: unselect all • q: quit\n",
	))

	return s.String()
}

func SelectDependencies(deps dependency.Dependencies) (dependency.Dependencies, error) {
	dependencyTableColumns := []table.Column{
		{Title: "", Width: 2},
		{Title: "Dependency", Width: 30},
		{Title: "Current Version", Width: 15},
		{Title: "New Version", Width: 15},
		{Title: "Type", Width: 15},
	}

	versionsTableColumns := []table.Column{
		{Title: "Dependency", Width: 15},
		{Title: "Version", Width: 15},
	}

	dependencyTable := table.New(
		table.WithColumns(dependencyTableColumns),
		table.WithFocused(true),
		table.WithHeight(7),
	)

	versionsTable := table.New(
		table.WithColumns(versionsTableColumns),
		table.WithFocused(false),
		table.WithHeight(7),
	)

	defaultTableStyles := table.DefaultStyles()
	defaultTableStyles.Header = defaultTableStyles.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#63B0B8")).
		BorderBottom(true).
		Bold(true)
	defaultTableStyles.Selected = defaultTableStyles.Selected.
		Foreground(lipgloss.Color("#ffffff")).
		Background(lipgloss.Color("#FF75B7")).
		Bold(true)
	dependencyTable.SetStyles(defaultTableStyles)
	versionsTable.SetStyles(defaultTableStyles)

	initialModel := model{
		dependencies:    deps,
		selected:        make(map[int]struct{}),
		dependencyTable: dependencyTable,
		versionsTable:   versionsTable,
	}

	p := tea.NewProgram(initialModel)
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("error running bubbletea program: %w", err)
	}

	m := finalModel.(model)
	if m.quitting {
		return nil, fmt.Errorf("selection cancelled by user")
	}

	for i := range m.selected {
		deps[i].HaveToUpdate = true
	}

	return deps, nil
}
