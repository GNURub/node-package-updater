package audit

import (
	"fmt"
	"strings"

	"github.com/GNURub/node-package-updater/internal/styles"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type findingItem struct {
	finding Finding
}

func (i findingItem) Title() string {
	return styles.SeverityBadge(i.finding.Severity) + " " +
		styles.AccentStyle.Render(i.finding.PackageName) +
		styles.MutedStyle.Render("@"+i.finding.Version)
}

func (i findingItem) Description() string {
	if i.finding.Summary != "" {
		return styles.MutedStyle.Render(i.finding.ID+" · ") + truncate(i.finding.Summary, 70)
	}
	return styles.MutedStyle.Render(i.finding.ID)
}

func (i findingItem) FilterValue() string {
	return i.finding.PackageName + " " + i.finding.ID + " " + strings.Join(i.finding.Aliases, " ")
}

var (
	listPaneStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, true, false, false).
			BorderForeground(lipgloss.Color("#3F4757")).
			PaddingRight(1)

	detailPaneStyle = lipgloss.NewStyle().
			Padding(0, 2)
)

type tuiModel struct {
	result        *Result
	list          list.Model
	viewport      viewport.Model
	width, height int
	ready         bool
	lastSelected  string
}

func newTUIModel(result *Result) tuiModel {
	sorted := sortFindingsBySeverity(result.Findings)
	items := make([]list.Item, 0, len(sorted))
	for _, f := range sorted {
		items = append(items, findingItem{finding: f})
	}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color("#FF75B7")).
		BorderForeground(lipgloss.Color("#FF75B7"))
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(lipgloss.Color("#C8B5C8")).
		BorderForeground(lipgloss.Color("#FF75B7"))

	l := list.New(items, delegate, 0, 0)
	l.Title = "Findings"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)
	l.Styles.Title = l.Styles.Title.
		Background(lipgloss.Color("#FF75B7")).
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true).
		Padding(0, 1)

	return tuiModel{
		result:   result,
		list:     l,
		viewport: viewport.New(0, 0),
	}
}

func (m tuiModel) Init() tea.Cmd { return nil }

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.list.FilterState() != list.Filtering {
			switch msg.String() {
			case "q", "ctrl+c", "esc":
				return m, tea.Quit
			}
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		m.ready = true
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)

	m.refreshDetail()

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *tuiModel) layout() {
	if m.width == 0 || m.height == 0 {
		return
	}

	header := m.headerView()
	footer := m.footerView()
	bodyHeight := m.height - lipgloss.Height(header) - lipgloss.Height(footer) - 1
	if bodyHeight < 8 {
		bodyHeight = 8
	}

	listWidth := m.width / 2
	if listWidth < 32 {
		listWidth = 32
	}
	if listWidth > 60 {
		listWidth = 60
	}
	detailWidth := m.width - listWidth - 6
	if detailWidth < 20 {
		detailWidth = 20
	}

	m.list.SetSize(listWidth, bodyHeight)
	m.viewport.Width = detailWidth
	m.viewport.Height = bodyHeight
	m.lastSelected = "" // force re-render
}

func (m *tuiModel) refreshDetail() {
	item, ok := m.list.SelectedItem().(findingItem)
	if !ok {
		m.viewport.SetContent("")
		return
	}
	key := item.finding.ID + "|" + fmt.Sprint(m.viewport.Width)
	if key == m.lastSelected {
		return
	}
	m.lastSelected = key
	m.viewport.SetContent(renderFindingDetail(item.finding, m.viewport.Width))
	m.viewport.GotoTop()
}

func (m tuiModel) View() string {
	if !m.ready {
		return "Loading…"
	}

	body := lipgloss.JoinHorizontal(lipgloss.Top,
		listPaneStyle.Render(m.list.View()),
		detailPaneStyle.Render(m.viewport.View()),
	)
	return lipgloss.JoinVertical(lipgloss.Left,
		m.headerView(),
		body,
		m.footerView(),
	)
}

func (m tuiModel) headerView() string {
	r := m.result
	width := m.width - 2
	if width < 20 {
		width = 20
	}
	body := lipgloss.JoinVertical(lipgloss.Left,
		styles.TitleStyle.Render("npu audit"),
		styles.ErrorStyle.Bold(true).Render(
			fmt.Sprintf("🚨 %d vulnerabilities · %d packages affected",
				r.Summary.TotalFindings, r.Summary.AffectedPackages),
		),
		styles.MutedStyle.Render(fmt.Sprintf("%s  ·  %d lockfile(s) · %d directory(ies)",
			r.Summary.RootDir, r.Summary.LockfilesScanned, r.Summary.DirectoriesScanned)),
		renderSeverityBar(r.Findings),
	)
	return styles.PanelStyle.Width(width).Render(body)
}

func (m tuiModel) footerView() string {
	return styles.MutedStyle.PaddingLeft(2).Render(
		"↑/↓ navigate · / filter · pgup/pgdn scroll details · q exit",
	)
}

func renderFindingDetail(f Finding, width int) string {
	if width < 20 {
		width = 80
	}

	pkg := lipgloss.JoinHorizontal(lipgloss.Left,
		styles.AccentStyle.Render(f.PackageName),
		styles.MutedStyle.Render("@"+f.Version),
		"  ",
		styles.SeverityBadge(f.Severity),
	)

	lines := []string{pkg, ""}
	lines = append(lines, fieldLine("ID", f.ID))
	lines = append(lines, fieldLine("Source", fmt.Sprintf("%s (%s)", f.SourcePath, f.SourceType)))
	if f.Ecosystem != "" {
		lines = append(lines, fieldLine("Ecosystem", f.Ecosystem))
	}
	if len(f.Aliases) > 0 {
		lines = append(lines, fieldLine("Aliases", strings.Join(f.Aliases, ", ")))
	}
	if f.Summary != "" {
		lines = append(lines, "", styles.LabelStyle.Render("Summary"), wrap(f.Summary, width))
	}
	if f.Details != "" {
		lines = append(lines, "", styles.LabelStyle.Render("Details"), wrap(f.Details, width))
	}
	lines = append(lines,
		"",
		styles.SuccessStyle.Bold(true).Render("▸ Suggested fix"),
		wrap(f.Recommendation, width),
	)
	return strings.Join(lines, "\n")
}

// RunInteractive launches the bubbletea browser for the audit findings.
// It returns nil immediately when there is nothing to display.
func RunInteractive(result *Result) error {
	if result == nil || len(result.Findings) == 0 {
		return nil
	}
	p := tea.NewProgram(newTUIModel(result), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
