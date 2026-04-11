package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/GNURub/node-package-updater/internal/dependency"
	"github.com/GNURub/node-package-updater/internal/semver"
	"github.com/GNURub/node-package-updater/internal/styles"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// UI Constants and styles
const (
	tickDuration = time.Second
)

// AuditRunner is a function that audits dependencies in the background.
// Implementations should call onResult for each dependency as results arrive.
// The ctx is cancelled when the user closes the TUI — respect it.
type AuditRunner func(
	ctx context.Context,
	deps dependency.Dependencies,
	onResult func(index int, status dependency.AuditStatus, severity string, count int),
)

var (
	baseStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#63B0B8"))

	getFatStyle = lipgloss.NewStyle().
			Margin(0).
			Foreground(lipgloss.Color("#63B0B8")).
			Render("↓")

	loseWeightStyle = lipgloss.NewStyle().
			Margin(0).
			Foreground(lipgloss.Color("#FF75B7")).
			Render("↑")

	versionColors = map[semver.VersionDiff]lipgloss.Style{
		semver.Prerelease: lipgloss.NewStyle().Foreground(lipgloss.Color("#ff3f18")),
		semver.Build:      lipgloss.NewStyle().Foreground(lipgloss.Color("#ff3f18")),
		semver.Major:      lipgloss.NewStyle().Foreground(lipgloss.Color("#ff4757")),
		semver.Minor:      lipgloss.NewStyle().Foreground(lipgloss.Color("#ff7f50")),
		semver.Patch:      lipgloss.NewStyle().Foreground(lipgloss.Color("#2ed573")),
		semver.None:       lipgloss.NewStyle().Foreground(lipgloss.Color("#576574")),
	}
)

type sessionState int

const (
	depsView sessionState = iota
	versionsView
)

// auditResultMsg is sent from the background audit goroutine to the model.
type auditResultMsg struct {
	Index    int
	Status   dependency.AuditStatus
	Severity string
	Count    int
}

type model struct {
	state           sessionState
	dependencies    dependency.Dependencies
	selected        map[int]struct{}
	quitting        bool
	done            bool
	dependencyTable table.Model
	versionsTable   table.Model
	spinner         spinner.Model
	auditCancel     context.CancelFunc
}

type tickMsg struct{}

// UI Configuration
var (
	dependencyTableColumns = []table.Column{
		{Title: "", Width: 2},
		{Title: "Dependency", Width: 26},
		{Title: "Current Version", Width: 15},
		{Title: "New Version", Width: 28},
		{Title: "Security", Width: 14},
		{Title: "Environment", Width: 12},
		{Title: "Workspace", Width: 18},
	}

	versionsTableColumns = []table.Column{
		{Title: "Dependency", Width: 15},
		{Title: "Version", Width: 15},
		{Title: "Diff weight", Width: 30},
	}
)

// Helper functions
func createTableStyles() table.Styles {
	styles := table.DefaultStyles()
	styles.Header = styles.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#63B0B8")).
		BorderBottom(true).
		Bold(true)
	styles.Selected = styles.Selected.
		Foreground(lipgloss.Color("#ffffff")).
		Background(lipgloss.Color("#FF75B7")).
		Bold(true)
	return styles
}

func tick() tea.Cmd {
	return tea.Tick(tickDuration, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

func drawStyleForNewVersion(dep *dependency.Dependency) string {
	v := dep.NextVersion.String()
	if dep.NextVersion.Deprecated {
		v += "🚩"
	}

	diff := dep.CurrentVersion.Diff(dep.NextVersion.Version)
	if style, ok := versionColors[diff]; ok {
		return style.Render(v)
	}
	return v
}

// drawSecurityCell renders the security column cell for a dependency.
// spin is the current spinner frame string to show while scanning.
func drawSecurityCell(dep *dependency.Dependency, spin string) string {
	status, severity, count := dep.SnapshotAudit()
	switch status {
	case dependency.AuditPending, dependency.AuditScanning:
		return styles.MutedStyle.Render(spin + " scanning")
	case dependency.AuditClean:
		return styles.SuccessStyle.Render("✓ safe")
	case dependency.AuditVulnerable:
		return fmt.Sprintf("⚠ %s ×%d", styles.SeverityForeground(severity), count)
	case dependency.AuditError:
		return styles.MutedStyle.Render("? unknown")
	}
	return ""
}

func (m *model) handleSelectionCommand(selector func(*dependency.Dependency) bool) {
	for i, dep := range m.dependencies {
		if selector(dep) {
			m.selected[i] = struct{}{}
		}
	}
}

// Model methods
func (m model) Init() tea.Cmd {
	return tea.Batch(tick(), m.spinner.Tick)
}

func (m model) updateVersions(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		}
	}

	m.versionsTable, cmd = m.versionsTable.Update(msg)
	return m, cmd
}

func (m model) updateDeps(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+a":
			m.handleSelectionCommand(func(dep *dependency.Dependency) bool { return true })
		case "ctrl+u":
			m.selected = make(map[int]struct{})
		case "ctrl+d":
			m.handleSelectionCommand(func(dep *dependency.Dependency) bool { return dep.Env == "dev" })
		case "ctrl+z":
			m.handleSelectionCommand(func(dep *dependency.Dependency) bool { return dep.Env == "prod" })
		case "ctrl+x":
			m.handleSelectionCommand(func(dep *dependency.Dependency) bool {
				return dep.CurrentVersion.Diff(dep.NextVersion.Version) == semver.Patch
			})
		case "ctrl+b":
			m.handleSelectionCommand(func(dep *dependency.Dependency) bool {
				return dep.CurrentVersion.Diff(dep.NextVersion.Version) == semver.Minor
			})
		case "ctrl+v":
			m.handleSelectionCommand(func(dep *dependency.Dependency) bool {
				st, _, _ := dep.SnapshotAudit()
				return st == dependency.AuditVulnerable
			})
		case "enter":
			m.done = true
			return m, tea.Sequence(tea.ClearScreen, tea.Quit)
		case "right", "l":
			m.switchToVersionsView()
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

func (m *model) switchToVersionsView() {
	cursor := m.dependencyTable.Cursor()
	rows := m.buildVersionRows(cursor)
	m.versionsTable.SetRows(rows)
	m.versionsTable.SetCursor(m.findCurrentVersionIndex(cursor))
	m.state = versionsView
	m.versionsTable.Focus()
	m.dependencyTable.Blur()
}

func (m *model) buildVersionRows(cursor int) []table.Row {
	var rows []table.Row
	currentDep := m.dependencies[cursor]
	currentWeight := m.getCurrentWeight(currentDep)

	for _, v := range currentDep.Versions.Values() {
		strVersion := v.String()
		if v.Deprecated {
			strVersion += "🚩"
		}

		weightDiff := m.formatWeightDiff(v.Weight, currentWeight)
		rows = append(rows, table.Row{
			currentDep.PackageName,
			strVersion,
			weightDiff,
		})
	}
	return rows
}

func (m *model) getCurrentWeight(dep *dependency.Dependency) uint64 {
	for _, v := range dep.Versions.Values() {
		if dep.CurrentVersion.Compare(v.Version) == 0 {
			return v.Weight
		}
	}
	return 0
}

func (m *model) formatWeightDiff(weight, currentWeight uint64) string {
	diff := int64(weight - currentWeight)
	if diff == 0 {
		return fmt.Sprintf("  %dKB", diff/1024)
	}
	if diff > 0 {
		return fmt.Sprintf("%s %dKB", loseWeightStyle, diff/1024)
	}
	return fmt.Sprintf("%s %dKB", getFatStyle, diff/1024)
}

func (m *model) findCurrentVersionIndex(cursor int) int {
	currentDep := m.dependencies[cursor]
	for i, v := range currentDep.Versions.Values() {
		if currentDep.NextVersion.Compare(v.Version) == 0 {
			return i
		}
	}
	return 0
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case auditResultMsg:
		dep := m.dependencies[msg.Index]
		switch msg.Status {
		case dependency.AuditClean:
			dep.SetAuditClean()
		case dependency.AuditVulnerable:
			dep.SetAuditResult(msg.Severity, msg.Count)
		default:
			dep.SetAuditError()
		}
		return m, nil

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
			return m, tea.Sequence(tea.ClearScreen, tea.Quit)
		}
	}

	if m.state == versionsView {
		return m.updateVersions(msg)
	}
	return m.updateDeps(msg)
}

func (m model) View() string {
	if m.quitting || m.done {
		return ""
	}

	var s strings.Builder
	if m.state == versionsView {
		s.WriteString(baseStyle.Render(m.versionsTable.View()) + "\n\n")
		s.WriteString(m.getVersionsViewFooter())
	} else {
		m.updateDependencyTableRows()
		s.WriteString(baseStyle.Render(m.dependencyTable.View()) + "\n\n")
		s.WriteString(m.getDepsViewFooter())
	}

	return s.String()
}

func (m *model) updateDependencyTableRows() {
	rows := m.dependencyTable.Rows()
	spin := m.spinner.View()
	for i := range rows {
		if _, ok := m.selected[i]; ok {
			rows[i][0] = "✓"
		} else {
			rows[i][0] = " "
		}
		rows[i][3] = drawStyleForNewVersion(m.dependencies[i])
		rows[i][4] = drawSecurityCell(m.dependencies[i], spin)
	}
	m.dependencyTable.SetRows(rows)
}

func (m model) getVersionsViewFooter() string {
	return lipgloss.NewStyle().MarginLeft(2).Render(
		"←/h: back • ↑/↓: navigate • space|enter: select • q|ctrl+c: exit\n",
	)
}

func (m model) getDepsViewFooter() string {
	return lipgloss.NewStyle().MarginLeft(2).Render(
		"↑/↓: navigate • space|enter: select • ctrl+a: select all • " +
			"ctrl+z: select only prod • ctrl+x: select patchs • " +
			"ctrl+b: select minors • ctrl+d: select only dev • " +
			"ctrl+v: select vulnerable • ctrl+u: unselect all • q|ctrl+c: exit\n",
	)
}

// SelectDependencies is the main entry point for the interactive selector UI.
// If auditRunner is non-nil it will be run in a background goroutine to
// populate the Security column while the user interacts with the table.
func SelectDependencies(deps dependency.Dependencies, auditRunner AuditRunner) (dependency.Dependencies, error) {
	m := createInitialModel(deps)

	auditCtx, auditCancel := context.WithCancel(context.Background())
	m.auditCancel = auditCancel

	p := tea.NewProgram(m)

	if auditRunner != nil {
		go func() {
			auditRunner(auditCtx, deps, func(index int, status dependency.AuditStatus, severity string, count int) {
				p.Send(auditResultMsg{
					Index:    index,
					Status:   status,
					Severity: severity,
					Count:    count,
				})
			})
		}()
	}

	finalModel, err := p.Run()
	auditCancel() // stop any in-flight audit work when the user exits

	if err != nil {
		return nil, fmt.Errorf("error running bubbletea program: %w", err)
	}

	m = finalModel.(model)
	if m.quitting {
		return nil, fmt.Errorf("selection cancelled by user")
	}

	for i := range m.selected {
		deps[i].HaveToUpdate = true
	}

	return deps, nil
}

func createInitialModel(deps dependency.Dependencies) model {
	var rows []table.Row
	for _, dep := range deps {
		rows = append(rows, table.Row{
			" ",
			dep.PackageName,
			dep.CurrentVersion.String(),
			drawStyleForNewVersion(dep),
			"",
			dep.Env.ToEnv(),
			dep.Workspace,
		})
	}

	tableStyles := createTableStyles()

	dependencyTable := table.New(
		table.WithColumns(dependencyTableColumns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(10),
	)
	dependencyTable.SetStyles(tableStyles)

	versionsTable := table.New(
		table.WithColumns(versionsTableColumns),
		table.WithFocused(false),
		table.WithHeight(10),
	)
	versionsTable.SetStyles(tableStyles)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#63B0B8"))

	return model{
		state:           depsView,
		dependencies:    deps,
		selected:        make(map[int]struct{}),
		dependencyTable: dependencyTable,
		versionsTable:   versionsTable,
		spinner:         sp,
	}
}
