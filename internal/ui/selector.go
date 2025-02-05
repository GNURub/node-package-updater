package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/GNURub/node-package-updater/internal/dependency"
	"github.com/GNURub/node-package-updater/internal/semver"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	baseStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#63B0B8"))
	getFatStyle     = lipgloss.NewStyle().Margin(0).Foreground(lipgloss.Color("#63B0B8")).Render("↓")
	loseWeightStyle = lipgloss.NewStyle().Margin(0).Foreground(lipgloss.Color("#FF75B7")).Render("↑")
)

type sessionState uint

const (
	depsView sessionState = iota
	versionsView
)

type model struct {
	state           sessionState
	dependencies    dependency.Dependencies
	selected        map[int]struct{}
	quitting        bool
	done            bool
	dependencyTable table.Model
	versionsTable   table.Model
}

type tickMsg struct{}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

func drawStyleForNewVersion(dep *dependency.Dependency) string {
	var s strings.Builder
	sufix := ""

	if dep.NextVersion.Deprecated {
		sufix = " [DEPRECATED]"
	}

	if dep.CurrentVersion.Major() < dep.NextVersion.Major() {
		s.WriteString(
			lipgloss.NewStyle().Foreground(lipgloss.Color("#ff4757")).Render(dep.NextVersion.String() + sufix),
		)
	} else if dep.CurrentVersion.Minor() < dep.NextVersion.Minor() {
		s.WriteString(
			lipgloss.NewStyle().Foreground(lipgloss.Color("#ff7f50")).Render(dep.NextVersion.String() + sufix),
		)
	} else if dep.CurrentVersion.Patch() < dep.NextVersion.Patch() {
		s.WriteString(
			lipgloss.NewStyle().Foreground(lipgloss.Color("#2ed573")).Render(dep.NextVersion.String() + sufix),
		)
	} else {
		s.WriteString(dep.NextVersion.String() + sufix)
	}

	return s.String()
}

func (m model) Init() tea.Cmd {
	return tick()
}

func updateVersions(msg tea.Msg, m model) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "left", "h", "esc":
			m.state = depsView
			m.dependencyTable.Focus()
			m.versionsTable.Blur()

		case " ", "enter":
			depCursor := m.dependencyTable.Cursor()
			versionCursor := m.versionsTable.Cursor()
			m.dependencies[depCursor].NextVersion = m.dependencies[depCursor].Versions.Values()[versionCursor]
			m.selected[depCursor] = struct{}{}
			m.state = depsView
			m.dependencyTable.Focus()
			m.versionsTable.Blur()
			m.dependencyTable, cmd = m.dependencyTable.Update(msg)
			return m, cmd

		}
	}

	m.versionsTable, cmd = m.versionsTable.Update(msg)

	return m, cmd
}

func updateDeps(msg tea.Msg, m model) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+a":
			for i := range m.dependencies {
				m.selected[i] = struct{}{}
			}
		case "ctrl+u":
			m.selected = make(map[int]struct{})

		case "ctrl+d":
			for i, dep := range m.dependencies {
				if dep.Env == "dev" {
					m.selected[i] = struct{}{}
				}
			}
		case "ctrl+z":
			for i, dep := range m.dependencies {
				if dep.Env == "prod" {
					m.selected[i] = struct{}{}
				}
			}
		case "ctrl+x":
			// Select only patch versions
			for i, dep := range m.dependencies {
				if dep.CurrentVersion.Diff(dep.NextVersion.Version) == semver.Patch {
					m.selected[i] = struct{}{}
				}
			}
		case "ctrl+b":
			// Select only minor versions
			for i, dep := range m.dependencies {
				if dep.CurrentVersion.Diff(dep.NextVersion.Version) == semver.Minor {
					m.selected[i] = struct{}{}
				}
			}

		case "enter":
			m.done = true
			return m, tea.Sequence(
				tea.ClearScreen,
				tea.Quit,
			)

		case "right", "l":
			cursor := m.dependencyTable.Cursor()
			var rows []table.Row
			cursorVersion := 0
			currentWeight := uint64(0)

			for _, v := range m.dependencies[cursor].Versions.Values() {
				if m.dependencies[cursor].CurrentVersion.Compare(v.Version) == 0 {
					currentWeight = v.Weight
					break
				}
			}

			for i, v := range m.dependencies[cursor].Versions.Values() {
				strVersion := v.String()

				var s strings.Builder

				diff := int64(v.Weight - currentWeight)
				if diff == 0 {
					s.WriteString(
						fmt.Sprintf("  %dKB", diff/1024),
					)
				} else if diff > 0 {
					s.WriteString(
						fmt.Sprintf("%s %dKB", loseWeightStyle, diff/1024),
					)
				} else {
					s.WriteString(
						fmt.Sprintf("%s %dKB", getFatStyle, diff/1024),
					)
				}

				rows = append(rows, table.Row{
					m.dependencies[cursor].PackageName,
					strVersion,
					s.String(),
				})

				if m.dependencies[cursor].NextVersion.Compare(v.Version) == 0 {
					cursorVersion = i
				}
			}

			m.versionsTable.SetRows(rows)

			m.versionsTable.SetCursor(cursorVersion)

			m.state = versionsView
			m.versionsTable.Focus()
			m.dependencyTable.Blur()

		case " ":
			cursor := m.dependencyTable.Cursor()
			if _, ok := m.selected[cursor]; ok {
				delete(m.selected, cursor)
			} else {
				m.selected[cursor] = struct{}{}
			}
			return m, tick()
		}
	}

	m.dependencyTable, cmd = m.dependencyTable.Update(msg)

	return m, cmd
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.dependencyTable.Focused() {
				m.dependencyTable.Blur()
			} else {
				m.dependencyTable.Focus()
			}
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Sequence(
				tea.ClearScreen,
				tea.Quit,
			)
		}
	}

	if m.state == versionsView {
		return updateVersions(msg, m)
	}

	return updateDeps(msg, m)
}

func (m model) View() string {
	var s strings.Builder
	if m.quitting || m.done {
		return ""
	}

	var footer string
	if m.state == versionsView {
		s.WriteString(baseStyle.Render(m.versionsTable.View()) + "\n\n")
		footer = "\u2190/h: back \u2022 \u2191/\u2193: navigate \u2022 space|enter: select"
	} else {
		rows := m.dependencyTable.Rows()
		for i := range rows {
			if _, ok := m.selected[i]; ok {
				rows[i][0] = "✓"
			} else {
				rows[i][0] = " "
			}
			rows[i][3] = drawStyleForNewVersion(m.dependencies[i])
		}
		m.dependencyTable.SetRows(rows)

		s.WriteString(baseStyle.Render(m.dependencyTable.View()) + "\n\n")

		footer = "\u2191/\u2193: navigate \u2022 space|enter: select \u2022 ctrl+a: select all \u2022 ctrl+z: select only prod \u2022 ctrl+x: select patchs \u2022 ctrl+b: select minors \u2022 ctrl+d: select only dev \u2022 ctrl+u: unselect all"
	}

	s.WriteString(lipgloss.NewStyle().MarginLeft(2).Render(fmt.Sprintf("%s \u2022 q|ctrl+c: exit\n", footer)))

	return s.String()
}

func SelectDependencies(deps dependency.Dependencies) (dependency.Dependencies, error) {
	dependencyTableColumns := []table.Column{
		{Title: "", Width: 2},
		{Title: "Dependency", Width: 30},
		{Title: "Current Version", Width: 15},
		{Title: "New Version", Width: 40},
		{Title: "Environment", Width: 15},
		{Title: "Workspace", Width: 20},
	}

	versionsTableColumns := []table.Column{
		{Title: "Dependency", Width: 15},
		{Title: "Version", Width: 15},
		{Title: "Diff weight", Width: 30},
	}

	var rows []table.Row
	for _, dep := range deps {
		rows = append(rows, table.Row{
			" ",
			dep.PackageName,
			dep.CurrentVersion.String(),
			drawStyleForNewVersion(dep),
			dep.Env,
			dep.Workspace,
		})
	}

	dependencyTable := table.New(
		table.WithColumns(dependencyTableColumns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	versionsTable := table.New(
		table.WithColumns(versionsTableColumns),
		table.WithFocused(false),
		table.WithHeight(10),
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
		state:           depsView,
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
