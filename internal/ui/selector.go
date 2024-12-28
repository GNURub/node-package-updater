package ui

import (
	"fmt"
	"strings"

	"github.com/GNURub/node-package-updater/internal/dependency"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Dependency struct {
	Name       string
	Version    string
	IsSelected bool
	Type       string // "dependencies" o "devDependencies"
}

type model struct {
	dependencies []Dependency
	cursor       int
	selected     map[int]struct{}
	quitting     bool
	table        table.Model
}

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF75B7")).
			MarginLeft(2)

	baseStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#63B0B8"))
)

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			m.quitting = true
			return m, tea.Quit
		case " ": // Espacio para seleccionar
			_, ok := m.selected[m.cursor]
			if ok {
				delete(m.selected, m.cursor)
			} else {
				m.selected[m.cursor] = struct{}{}
			}
		case "enter":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.dependencies)-1 {
				m.cursor++
			}
		}
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m model) View() string {
	if m.quitting {
		return "Operación cancelada\n"
	}

	var s strings.Builder

	s.WriteString(titleStyle.Render("Selecciona las dependencias a actualizar\n\n"))

	// Crear filas para la tabla
	var rows []table.Row
	for i, dep := range m.dependencies {
		selected := " "
		if _, ok := m.selected[i]; ok {
			selected = "×"
		}

		style := ""
		if i == m.cursor {
			style = "→"
		} else {
			style = " "
		}

		rows = append(rows, table.Row{
			style + selected,
			dep.Name,
			dep.Version,
			dep.Type,
		})
	}

	// Actualizar tabla
	m.table.SetRows(rows)
	s.WriteString(baseStyle.Render(m.table.View()) + "\n\n")

	s.WriteString(lipgloss.NewStyle().MarginLeft(2).Render(
		"↑/↓: navegar • espacio: seleccionar • enter: confirmar • q: cancelar\n",
	))

	return s.String()
}

func SelectDependencies(deps map[string]dependency.Dependencies) ([]string, error) {
	var dependencies []Dependency

	// Agregar dependencias regulares
	for env, deps := range deps {
		for _, dep := range deps {
			dependencies = append(dependencies, Dependency{
				Name:    dep.PackageName,
				Version: dep.NextVersion,
				Type:    env,
			})
		}
	}

	// Configurar columnas de la tabla
	columns := []table.Column{
		{Title: " ", Width: 3},
		{Title: "Dependencia", Width: 30},
		{Title: "Versión", Width: 15},
		{Title: "Tipo", Width: 15},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	// Estilo de la tabla
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

	// Obtener dependencias seleccionadas
	m := finalModel.(model)
	if m.quitting {
		return nil, fmt.Errorf("selección cancelada por el usuario")
	}

	var selectedDeps []string
	for idx := range m.selected {
		selectedDeps = append(selectedDeps, m.dependencies[idx].Name)
	}

	return selectedDeps, nil
}
