package ui

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"f/internal/notes"
	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/rivo/uniseg"
)

type PickerModel struct {
	input        textinput.Model
	entries      []notes.Entry
	allEntries   []notes.Entry
	matches      []notes.Match
	preview      notes.PreviewMatch
	options      Options
	cursor       int
	width        int
	height       int
	selected     *notes.Entry
	selectedLine int
	edit         bool
	printOnly    bool
	createKind   string
	previewHit   int
	cancelled    bool
	previewCache map[string]notes.PreviewMatch
	theme        Theme
	syncStatus   SyncStatus
	syncStream   <-chan SyncStatus
	activeKind   string
	spinner      spinner.Model
	details      bool
	detailOffset int
}

type syncPollMsg struct{}

func NewPicker(entries []notes.Entry, initialQuery string, theme Theme, options Options) PickerModel {
	input := textinput.New()
	input.Placeholder = ""
	input.Prompt = "> "
	input.SetValue(initialQuery)
	input.Focus()
	input.CharLimit = 256
	input.Width = 48
	input.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.RowFG))
	input.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.InputPrompt)).Bold(true)
	input.PlaceholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.HelpFG))
	input.Cursor.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.StatusWarnFG))
	input.Cursor.SetMode(cursor.CursorHide)
	syncSpinner := spinner.New()
	syncSpinner.Spinner = spinner.MiniDot
	syncSpinner.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.StatusRunFG))

	m := PickerModel{
		input:        input,
		entries:      entries,
		allEntries:   entries,
		theme:        theme,
		options:      options,
		previewCache: make(map[string]notes.PreviewMatch),
		syncStatus:   options.InitialSync,
		syncStream:   options.SyncStatusStream,
		activeKind:   "all",
		spinner:      syncSpinner,
	}
	m.refresh()
	return m
}

func (m PickerModel) Init() tea.Cmd {
	cmds := []tea.Cmd{}
	if m.syncStream != nil {
		cmds = append(cmds, m.pollSyncStatus())
	}
	if m.syncStatus.State == SyncStateRunning {
		cmds = append(cmds, m.spinner.Tick)
	}
	return tea.Batch(cmds...)
}

func (m PickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = m.inputWidth()
		return m, nil
	case syncPollMsg:
		if m.syncStream == nil {
			return m, nil
		}
		select {
		case status, ok := <-m.syncStream:
			if !ok {
				m.syncStream = nil
				return m, nil
			}
			m.syncStatus = status
		default:
		}
		return m, m.pollSyncStatus()
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if m.syncStatus.State == SyncStateRunning {
			return m, cmd
		}
		return m, nil
	case tea.KeyMsg:
		if m.details {
			m.detailOffset = minInt(maxInt(0, m.detailOffset), m.detailMaximumOffset())
			switch msg.String() {
			case "tab", "q":
				m.details = false
				m.detailOffset = 0
				return m, nil
			case "up", "ctrl+k":
				if m.detailOffset > 0 {
					m.detailOffset--
				}
				return m, nil
			case "down", "ctrl+j":
				if m.detailOffset < m.detailMaximumOffset() {
					m.detailOffset++
				}
				return m, nil
			case "ctrl+c":
				m.cancelled = true
				return m, tea.Quit
			case "enter", "ctrl+enter", "alt+enter", "ctrl+y":
				if len(m.matches) == 0 {
					return m, nil
				}
				entry := m.matches[m.cursor].Entry
				m.selected = &entry
				m.selectedLine = entry.PreviewHitLine(m.preview, m.activePreviewHit())
				m.printOnly = isPrintOnlyKey(msg.String())
				return m, tea.Quit
			case "ctrl+e", "alt+e":
				if len(m.matches) == 0 {
					return m, nil
				}
				entry := m.matches[m.cursor].Entry
				if !entry.Editable {
					return m, nil
				}
				m.selected = &entry
				m.selectedLine = entry.PreviewHitLine(m.preview, m.activePreviewHit())
				m.edit = true
				return m, tea.Quit
			default:
				return m, nil
			}
		}
		switch msg.String() {
		case "tab":
			if len(m.matches) > 0 {
				m.details = true
				m.detailOffset = 0
			}
			return m, nil
		case "f1":
			m.activeKind = "all"
			m.refresh()
			return m, nil
		case "f2":
			m.activeKind = "host"
			m.refresh()
			return m, nil
		case "f3":
			m.activeKind = "cmd"
			m.refresh()
			return m, nil
		case "f4":
			m.activeKind = "fwd"
			m.refresh()
			return m, nil
		case "f5":
			m.activeKind = "jump"
			m.refresh()
			return m, nil
		case "ctrl+c":
			m.cancelled = true
			return m, tea.Quit
		case "esc":
			m.cancelled = true
			return m, tea.Quit
		case "enter", "ctrl+enter", "alt+enter", "ctrl+y":
			if len(m.matches) == 0 {
				return m, nil
			}
			entry := m.matches[m.cursor].Entry
			m.selected = &entry
			m.selectedLine = entry.PreviewHitLine(m.preview, m.activePreviewHit())
			m.printOnly = isPrintOnlyKey(msg.String())
			return m, tea.Quit
		case "ctrl+e", "alt+e":
			if len(m.matches) == 0 {
				return m, nil
			}
			entry := m.matches[m.cursor].Entry
			if !entry.Editable {
				return m, nil
			}
			m.selected = &entry
			m.selectedLine = entry.PreviewHitLine(m.preview, m.activePreviewHit())
			m.edit = true
			return m, tea.Quit
		case "ctrl+n", "alt+n":
			m.createKind = "host"
			return m, tea.Quit
		case "up", "ctrl+k":
			m.moveCursor(-1)
			return m, nil
		case "down", "ctrl+j":
			m.moveCursor(1)
			return m, nil
		case "right":
			m.advancePreviewHit(1)
			return m, nil
		case "left":
			m.advancePreviewHit(-1)
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.refresh()
	return m, cmd
}

func (m PickerModel) View() string {
	rowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.RowFG))
	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.theme.MatchFG)).
		Bold(true)
	detailStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.DetailFG))
	statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.TitleDimFG))
	contentWidth := m.contentWidth()
	if m.details {
		return m.detailsView(contentWidth, rowStyle, detailStyle, statusStyle)
	}

	input := m.input
	input.Width = m.inputWidth()
	inputLine := lipgloss.NewStyle().
		Width(contentWidth).
		Render(truncateRunes(input.View(), contentWidth))
	statusLine := truncateRunes(m.renderStatusBar(statusStyle), contentWidth)
	commandBlock := m.renderCommandBlock(contentWidth, statusStyle, detailStyle)
	body := []string{}
	if m.shouldRenderResults() {
		body = m.resultLines(contentWidth, rowStyle, selectedStyle, detailStyle)
		limit := m.maxResultLines()
		if len(body) > limit {
			body = body[:limit]
		}
		if len(commandBlock) > 0 && !m.options.FullScreen {
			for len(body) < limit {
				body = append(body, "")
			}
		}
	}

	lines := []string{m.renderHeader(contentWidth)}
	if m.isBottomLayout() && !m.options.FullScreen {
		lines = append(lines, body...)
		lines = append(lines, inputLine)
	} else {
		lines = append(lines, inputLine)
		lines = append(lines, body...)
	}
	if m.options.FullScreen {
		reserved := len(commandBlock) + 1
		for len(lines)+reserved < m.effectiveHeight() {
			lines = append(lines, "")
		}
	}
	lines = append(lines, commandBlock...)
	lines = append(lines, statusLine)
	content := strings.Join(clipLines(lines, m.effectiveHeight()), "\n")
	return lipgloss.NewStyle().MarginLeft(2).Render(content)
}

func (m PickerModel) detailsView(width int, rowStyle, detailStyle, statusStyle lipgloss.Style) string {
	if len(m.matches) == 0 || m.cursor < 0 || m.cursor >= len(m.matches) {
		return lipgloss.NewStyle().MarginLeft(2).Render(detailStyle.Render("No details"))
	}
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.TitleDimFG)).Bold(true)
	lines := m.detailBodyLines(width, rowStyle, detailStyle, labelStyle)
	maximum := m.effectiveHeight()
	visibleBody := maxInt(1, maximum-1)
	maxOffset := maxInt(0, len(lines)-visibleBody)
	start := minInt(maxInt(0, m.detailOffset), maxOffset)
	if maxOffset > 0 {
		lines = append([]string(nil), lines[start:start+visibleBody]...)
	}
	footerText := "Q/Tab back  •  Enter run  •  Ctrl+Y print"
	if maxOffset > 0 {
		footerText = fmt.Sprintf("Q/Tab back  •  Enter run  •  Ctrl+Y print  •  ↑/↓ %d/%d", start+1, maxOffset+1)
	}
	footer := statusStyle.Render(truncateRunes(footerText, width))
	lines = append(lines, footer)
	if m.options.FullScreen {
		for len(lines) < maximum {
			lines = append(lines, "")
		}
	}
	content := strings.Join(clipLines(lines, maximum), "\n")
	return lipgloss.NewStyle().MarginLeft(2).Render(content)
}

func (m PickerModel) detailBodyLines(width int, rowStyle, detailStyle, labelStyle lipgloss.Style) []string {
	if len(m.matches) == 0 || m.cursor < 0 || m.cursor >= len(m.matches) {
		return nil
	}
	entry := m.matches[m.cursor].Entry
	if block := strings.TrimSpace(entry.SSHConfigBlock); block != "" {
		lines := make([]string, 0, strings.Count(block, "\n")+1)
		for _, line := range detailCommandLines(block, width) {
			lines = append(lines, rowStyle.Render(line))
		}
		return lines
	}
	badge := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#171717")).
		Background(lipgloss.Color(m.theme.InputPrompt)).
		Bold(true).
		Padding(0, 1).
		Render("DETAILS")
	lines := []string{badge}
	lines = appendDetailField(lines, "Name", entry.DisplayName(), width, labelStyle, rowStyle)
	lines = appendDetailField(lines, "Address", entry.Address, width, labelStyle, detailStyle)
	lines = appendDetailField(lines, "Type", entry.Kind, width, labelStyle, rowStyle)
	lines = appendDetailField(lines, "Mode", entry.Mode, width, labelStyle, rowStyle)
	if action := entry.PrimaryAction(); action != nil {
		lines = appendDetailField(lines, "About", action.Desc, width, labelStyle, rowStyle)
		if command := action.Cmd; strings.TrimSpace(command) != "" {
			lines = append(lines, labelStyle.Render("Command"))
			for _, line := range detailCommandLines(command, maxInt(8, width-2)) {
				lines = append(lines, rowStyle.Render("  "+line))
			}
		}
	}
	if source := detailSource(entry); source != "" {
		lines = appendDetailField(lines, "Source", source, width, labelStyle, detailStyle)
	}
	return lines
}

func (m PickerModel) detailMaximumOffset() int {
	lines := m.detailBodyLines(m.contentWidth(), lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle())
	return maxInt(0, len(lines)-maxInt(1, m.effectiveHeight()-1))
}

func detailCommandLines(command string, width int) []string {
	if width < 1 {
		width = 1
	}
	var lines []string
	for _, rawLine := range strings.Split(command, "\n") {
		lines = append(lines, wrapDisplayWidthPreferSpaces(rawLine, width)...)
	}
	return lines
}

func appendDetailField(lines []string, label, value string, width int, labelStyle, valueStyle lipgloss.Style) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return lines
	}
	prefix := label + "  "
	valueWidth := maxInt(8, width-utf8.RuneCountInString(prefix))
	wrapped := wrapText(value, valueWidth)
	lines = append(lines, labelStyle.Render(prefix)+valueStyle.Render(wrapped[0]))
	indent := strings.Repeat(" ", utf8.RuneCountInString(prefix))
	for _, line := range wrapped[1:] {
		lines = append(lines, valueStyle.Render(indent+line))
	}
	return lines
}

func detailSource(entry notes.Entry) string {
	source := strings.TrimSpace(entry.SourcePath)
	if source == "" {
		source = strings.TrimSpace(entry.SourceFile)
	}
	if source != "" && entry.SourceLine > 0 {
		return fmt.Sprintf("%s:%d", source, entry.SourceLine)
	}
	return source
}

func (m PickerModel) renderHeader(width int) string {
	label := map[string]string{
		"all":  "SSH",
		"host": "HOSTS",
		"cmd":  "COMMANDS",
		"fwd":  "FORWARDS",
		"jump": "JUMPS",
	}[m.activeKind]
	if label == "" {
		label = "SSH"
	}
	badge := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#171717")).
		Background(lipgloss.Color(m.theme.InputPrompt)).
		Bold(true).
		Padding(0, 1).
		Render(label)
	metaText := fmt.Sprintf("  %d", len(m.matches))
	if len(m.matches) != len(m.allEntries) {
		metaText = fmt.Sprintf("  %d / %d", len(m.matches), len(m.allEntries))
	}
	meta := lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.theme.TitleDimFG)).
		Render(metaText)
	left := badge + meta
	if m.syncStatus.State != SyncStateOK {
		if sync := m.renderSyncStatus(); sync != "" {
			left += lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.TitleDimFG)).Render("  ·  ") + sync
		}
	}
	return truncateRunes(left, width)
}

func (m PickerModel) pollSyncStatus() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(time.Time) tea.Msg {
		return syncPollMsg{}
	})
}

func (m PickerModel) Selected() *notes.Entry {
	return m.selected
}

func (m PickerModel) SelectedLine() int {
	return m.selectedLine
}

func (m PickerModel) Cancelled() bool {
	return m.cancelled
}

func (m PickerModel) EditRequested() bool {
	return m.edit
}

func (m PickerModel) PrintOnlyRequested() bool {
	return m.printOnly
}

func (m PickerModel) CreateKind() string {
	return m.createKind
}

func (m PickerModel) Query() string {
	return m.input.Value()
}

func (m PickerModel) shouldRenderResults() bool {
	return true
}

func (m *PickerModel) refresh() {
	m.entries = filterEntriesByKind(m.allEntries, m.activeKind)
	m.matches = notes.Filter(m.entries, m.input.Value())
	if m.cursor >= len(m.matches) {
		m.cursor = len(m.matches) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.refreshPreview()
	m.clampPreviewHit()
}

func filterEntriesByKind(entries []notes.Entry, kind string) []notes.Entry {
	if kind == "" || kind == "all" {
		return entries
	}
	filtered := make([]notes.Entry, 0, len(entries))
	for _, entry := range entries {
		for _, entryKind := range strings.Split(entry.Kind, "/") {
			if strings.TrimSpace(entryKind) == kind {
				filtered = append(filtered, entry)
				break
			}
		}
	}
	return filtered
}

func (m *PickerModel) refreshPreview() {
	if len(m.matches) == 0 {
		m.preview = notes.PreviewMatch{}
		return
	}
	m.preview = m.cachedPreview(m.matches[m.cursor].Entry)
}

func (m PickerModel) visibleMatches() []notes.Match {
	if len(m.matches) == 0 || !m.shouldRenderResults() {
		return nil
	}

	maxItems := m.maxVisibleItems()

	start := m.offset()
	end := start + maxItems
	if end > len(m.matches) {
		end = len(m.matches)
	}

	return m.matches[start:end]
}

func (m PickerModel) offset() int {
	if len(m.matches) == 0 {
		return 0
	}
	window := m.maxVisibleItems()

	if m.cursor < window/2 {
		return 0
	}

	start := m.cursor - window/2
	limit := len(m.matches) - window
	if limit < 0 {
		return 0
	}
	if start > limit {
		return limit
	}
	return start
}

func (m PickerModel) maxVisibleItems() int {
	available := m.maxResultLines()
	if available < 1 {
		available = 1
	}
	return available
}

func (m PickerModel) maxResultLines() int {
	available := m.viewHeight() - 3 - m.commandBlockHeight()
	if available < 1 {
		return 1
	}
	return available
}

func pickerProgramOptions(_ Options) []tea.ProgramOption {
	return []tea.ProgramOption{tea.WithAltScreen()}
}

func RunPicker(entries []notes.Entry, initialQuery string, options Options) (*notes.Entry, int, string, bool, bool, bool, string, error) {
	model := NewPicker(entries, initialQuery, DefaultTheme(), options)
	program := tea.NewProgram(model, pickerProgramOptions(options)...)
	result, err := program.Run()
	if err != nil {
		return nil, 0, initialQuery, false, false, false, "", fmt.Errorf("picker: %w", err)
	}

	finalModel, ok := result.(PickerModel)
	if !ok {
		return nil, 0, initialQuery, false, false, false, "", fmt.Errorf("picker returned unexpected model type")
	}

	return finalModel.Selected(), finalModel.SelectedLine(), finalModel.Query(), finalModel.Cancelled(), finalModel.EditRequested(), finalModel.PrintOnlyRequested(), finalModel.CreateKind(), nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (m PickerModel) contentWidth() int {
	return maxInt(16, m.cardWidth()-6)
}

func (m PickerModel) cardWidth() int {
	if m.width <= 0 {
		return 80
	}
	available := m.width - 4
	if available < 20 {
		return maxInt(8, m.width)
	}
	return minInt(96, available)
}

func (m PickerModel) horizontalMargin() int {
	if m.width <= m.cardWidth() {
		return 0
	}
	return (m.width - m.cardWidth()) / 2
}

func (m PickerModel) effectiveHeight() int {
	height := m.height
	if height <= 0 {
		height = m.options.Height
	}
	if m.options.Height > 0 && (height <= 0 || m.options.Height < height) {
		height = m.options.Height
	}
	if height < 6 {
		return 6
	}
	return height
}

func (m PickerModel) viewHeight() int {
	maximum := m.effectiveHeight()
	if m.options.FullScreen {
		return maximum
	}
	resultLines := 0
	emptyStyle := lipgloss.NewStyle()
	for index, match := range m.matches {
		resultLines += len(m.renderMatchLabelLines(match, match.Entry, m.contentWidth(), index == m.cursor, emptyStyle, emptyStyle, emptyStyle))
	}
	if resultLines == 0 {
		resultLines = 1
	}
	desired := resultLines + m.commandBlockHeight() + 3
	if desired < 8 {
		desired = 8
	}
	return minInt(maximum, desired)
}

func (m PickerModel) inputWidth() int {
	// Bubbles reserves one cell for the cursor. Leave that cell outside the
	// configured text width or it appends an overflow ellipsis to short queries.
	width := m.contentWidth() - lipgloss.Width(m.input.Prompt) - 1
	if width < 1 {
		return 1
	}
	return width
}

func (m PickerModel) isBottomLayout() bool {
	return strings.EqualFold(strings.TrimSpace(m.options.Layout), "bottom")
}

func (m PickerModel) showInlinePreview() bool {
	return false
}

func (m PickerModel) resultRowHeight() int {
	return 1
}

func (m PickerModel) statusLine() string {
	kind := m.activeKind
	if kind == "" {
		kind = "all"
	}
	status := fmt.Sprintf("%s  %d/%d", kind, len(m.matches), len(m.allEntries))
	if len(m.matches) == 0 {
		return status
	}
	if m.showInlinePreview() {
		if hits := m.currentPreviewHitCount(); hits > 1 {
			return fmt.Sprintf("%s  hit %d/%d", status, m.activePreviewHit()+1, hits)
		}
	}
	return status
}

func (m PickerModel) renderStatusBar(baseStyle lipgloss.Style) string {
	return baseStyle.Render("Ctrl+E edit  •  Tab details  •  F1–F5 categories")
}

func (m PickerModel) renderSyncStatus() string {
	label := strings.TrimSpace(m.syncStatusLabel())
	if label == "" {
		return ""
	}
	if m.syncStatus.State == SyncStateRunning {
		label = m.spinner.View() + " " + label
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.syncStatusColor())).
		Render(label)
}

func (m PickerModel) syncStatusLabel() string {
	switch m.syncStatus.State {
	case SyncStateRunning:
		return "sync"
	case SyncStateOK:
		return "sync ok"
	case SyncStateWarn:
		if msg := strings.TrimSpace(m.syncStatus.Message); msg != "" {
			return "sync " + msg
		}
		return "sync warn"
	case SyncStateError:
		if msg := strings.TrimSpace(m.syncStatus.Message); msg != "" {
			return "sync " + msg
		}
		return "sync failed"
	default:
		return ""
	}
}

func (m PickerModel) syncStatusColor() string {
	switch m.syncStatus.State {
	case SyncStateOK:
		return m.theme.StatusOKFG
	case SyncStateWarn:
		return m.theme.StatusWarnFG
	case SyncStateError:
		return m.theme.StatusErrFG
	case SyncStateRunning:
		return m.theme.StatusRunFG
	default:
		return m.theme.TitleDimFG
	}
}

func pickerHelpText() string {
	return string([]rune{0x2191, 0x2193}) + " select  enter ssh  ctrl+e edit  ctrl+y print  esc quit"
}

func isPrintOnlyKey(key string) bool {
	switch key {
	case "ctrl+enter", "alt+enter", "ctrl+y":
		return true
	default:
		return false
	}
}

func wrapText(value string, width int) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return []string{""}
	}

	var lines []string
	for _, rawLine := range strings.Split(value, "\n") {
		rawLine = strings.TrimRight(rawLine, " ")
		if rawLine == "" {
			lines = append(lines, "")
			continue
		}
		runes := []rune(rawLine)
		for len(runes) > width && width > 1 {
			lines = append(lines, string(runes[:width]))
			runes = runes[width:]
		}
		lines = append(lines, string(runes))
	}
	return lines
}

func clipLines(lines []string, height int) []string {
	if height <= 0 || len(lines) <= height {
		return lines
	}
	if height == 1 {
		return []string{"…"}
	}

	clipped := append([]string{}, lines[:height]...)
	clipped[height-1] = "…"
	return clipped
}

func excerptLines(lines []string, limit int) []string {
	if limit <= 0 || len(lines) <= limit {
		return lines
	}
	if limit == 1 {
		return []string{"…"}
	}

	clipped := append([]string{}, lines[:limit]...)
	clipped[limit-1] = "…"
	return clipped
}

func truncateRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if ansi.StringWidth(value) <= limit {
		return value
	}
	return ansi.Truncate(value, limit, "…")
}

func truncateMiddleRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if ansi.StringWidth(value) <= limit {
		return value
	}
	if limit == 1 {
		return "…"
	}

	const minPrefix = 8
	suffixWidth := (limit - 1) / 3
	if split := strings.LastIndex(value, " "); split >= 0 {
		targetWidth := ansi.StringWidth(strings.TrimSpace(value[split+1:]))
		if targetWidth > 0 && targetWidth <= limit-1-minPrefix {
			suffixWidth = targetWidth
		}
	}
	if suffixWidth < 1 {
		suffixWidth = 1
	}
	prefixWidth := limit - 1 - suffixWidth
	return ansi.Truncate(value, prefixWidth, "") + "…" + truncateLeftWidth(value, suffixWidth)
}

func truncateLeftWidth(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if ansi.StringWidth(value) <= limit {
		return value
	}
	graphemes := uniseg.NewGraphemes(value)
	clusters := make([]string, 0, utf8.RuneCountInString(value))
	for graphemes.Next() {
		clusters = append(clusters, graphemes.Str())
	}
	width := 0
	start := len(clusters)
	for start > 0 {
		clusterWidth := ansi.StringWidth(clusters[start-1])
		if width+clusterWidth > limit {
			break
		}
		start--
		width += clusterWidth
	}
	return strings.Join(clusters[start:], "")
}

func padRight(value string, width int) string {
	length := utf8.RuneCountInString(value)
	if length >= width {
		return value
	}
	return value + strings.Repeat(" ", width-length)
}

func normalizeRenderedLines(lines []string, width int) []string {
	if width <= 0 {
		return lines
	}
	style := lipgloss.NewStyle().Width(width).MaxWidth(width)
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, style.Render(line))
	}
	return out
}

func (m *PickerModel) advancePreviewHit(delta int) {
	if len(m.matches) == 0 {
		return
	}
	occurrences := m.currentPreviewHitCount()
	if occurrences <= 1 {
		m.previewHit = 0
		return
	}
	m.previewHit = (m.previewHit + delta + occurrences) % occurrences
}

func (m *PickerModel) clampPreviewHit() {
	if len(m.matches) == 0 {
		m.previewHit = 0
		return
	}
	occurrences := m.currentPreviewHitCount()
	if occurrences <= 0 {
		m.previewHit = 0
		return
	}
	if m.previewHit >= occurrences {
		m.previewHit = occurrences - 1
	}
	if m.previewHit < 0 {
		m.previewHit = 0
	}
}

func (m PickerModel) activePreviewHit() int {
	if len(m.matches) == 0 {
		return 0
	}
	occurrences := m.currentPreviewHitCount()
	if occurrences == 0 {
		return 0
	}
	if m.previewHit >= occurrences || m.previewHit < 0 {
		return 0
	}
	return m.previewHit
}

func (m PickerModel) currentPreviewHitCount() int {
	return previewHitCount(m.preview)
}

func previewHitCount(preview notes.PreviewMatch) int {
	if count := len(preview.Snippets); count > 0 {
		return count
	}
	return len(preview.Occurrences)
}

func commandPreviewLines(value string, width int) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return []string{""}
	}
	if strings.Contains(value, "\n") {
		return wrapText(value, width)
	}
	fields := strings.Fields(value)
	if len(fields) <= 3 || utf8.RuneCountInString(value) <= width {
		return wrapText(value, width)
	}
	groups := []string{fields[0]}
	for i := 1; i < len(fields); i++ {
		field := fields[i]
		if (field == "-o" || field == "-J" || field == "-p" || field == "-i" || field == "-L" || field == "-R" || field == "-D") && i+1 < len(fields) {
			groups = append(groups, field+" "+fields[i+1])
			i++
			continue
		}
		groups = append(groups, field)
	}
	lines := []string{groups[0] + " \\"}
	for i := 1; i < len(groups); i++ {
		line := "  " + groups[i]
		if i < len(groups)-1 {
			line += " \\"
		}
		lines = append(lines, line)
	}
	return lines
}

func (m PickerModel) resultLines(width int, rowStyle, selectedStyle, detailStyle lipgloss.Style) []string {
	if len(m.matches) == 0 {
		return []string{detailStyle.Render("No matches")}
	}

	type renderedRow struct {
		lines []string
	}
	budget := m.maxResultLines()
	rows := make([]renderedRow, 0, len(m.matches))
	for index, match := range m.matches {
		entry := match.Entry
		selected := index == m.cursor
		rowLines := m.renderMatchLabelLines(match, entry, width, selected, rowStyle, selectedStyle, detailStyle)
		if selected {
			rowLines = fitSelectedLabelLines(rowLines, "", width, budget, detailStyle)
		}
		rows = append(rows, renderedRow{lines: rowLines})
	}

	selectedIndex := minInt(maxInt(0, m.cursor), len(rows)-1)
	chosen := map[int]bool{selectedIndex: true}
	remaining := budget - len(rows[selectedIndex].lines)
	blockedAbove, blockedBelow := false, false
	for distance := 1; remaining > 0 && (!blockedAbove || !blockedBelow); distance++ {
		above := selectedIndex - distance
		if !blockedAbove {
			if above < 0 {
				blockedAbove = true
			} else if len(rows[above].lines) > remaining {
				blockedAbove = true
			} else {
				chosen[above] = true
				remaining -= len(rows[above].lines)
			}
		}
		below := selectedIndex + distance
		if !blockedBelow {
			if below >= len(rows) {
				blockedBelow = true
			} else if len(rows[below].lines) > remaining {
				blockedBelow = true
			} else {
				chosen[below] = true
				remaining -= len(rows[below].lines)
			}
		}
	}
	lines := make([]string, 0, budget)
	for index, row := range rows {
		if chosen[index] {
			lines = append(lines, row.lines...)
		}
	}
	return lines
}

func (m PickerModel) commandBlockHeight() int {
	if !m.options.ShowAddress {
		return 0
	}
	return 2
}

func (m PickerModel) renderCommandBlock(width int, labelStyle, commandStyle lipgloss.Style) []string {
	if m.commandBlockHeight() == 0 {
		return nil
	}
	command := "—"
	if len(m.matches) > 0 && m.cursor >= 0 && m.cursor < len(m.matches) {
		if selected := strings.TrimSpace(m.matches[m.cursor].Entry.Command); selected != "" {
			command = strings.Join(strings.Fields(selected), " ")
		}
	}
	commandWidth := maxInt(1, width-2)
	return []string{
		labelStyle.Render("command"),
		commandStyle.Render("  " + truncateMiddleRunes(command, commandWidth)),
	}
}

func fitSelectedLabelLines(lines []string, target string, width, budget int, detailStyle lipgloss.Style) []string {
	if budget <= 0 || len(lines) == 0 {
		return nil
	}
	if len(lines) <= budget {
		return lines
	}
	target = strings.TrimSpace(target)
	if target == "" {
		if budget == 1 {
			return []string{truncateRunes(lines[0], width)}
		}
		out := append([]string(nil), lines[:budget-1]...)
		out = append(out, detailStyle.Render("…"))
		return out
	}
	targetWidth := maxInt(4, maxInt(12, width)-ansi.StringWidth("    "))
	targetCount := len(wrapDisplayWidth(target, targetWidth))
	if targetCount >= budget {
		if targetCount <= len(lines) {
			return append([]string(nil), lines[len(lines)-targetCount:]...)
		}
		return append([]string(nil), lines[len(lines)-budget:]...)
	}
	nameSlots := budget - targetCount
	nameLines := lines[:len(lines)-targetCount]
	targetLines := lines[len(lines)-targetCount:]
	out := append([]string(nil), nameLines[:minInt(len(nameLines), nameSlots)]...)
	if len(nameLines) > nameSlots && len(out) > 0 {
		out[len(out)-1] = detailStyle.Render("…")
	}
	out = append(out, targetLines...)
	return out
}

func (m PickerModel) renderMatchLabelLines(match notes.Match, entry notes.Entry, width int, selected bool, rowStyle, selectedStyle, detailStyle lipgloss.Style) []string {
	label := strings.Join(strings.Fields(strings.TrimSpace(match.Label)), " ")
	if label == "" {
		label = strings.Join(strings.Fields(strings.TrimSpace(match.Detail)), " ")
	}
	prefix := "  "
	if selected {
		prefix = m.theme.SelectedMark + " "
	}
	rowWidth := maxInt(12, width)
	contentWidth := maxInt(8, rowWidth-ansi.StringWidth(prefix))
	if ansi.StringWidth(label) <= contentWidth {
		return []string{m.renderMatchLabelLine(match, entry, width, selected, rowStyle, selectedStyle, detailStyle)}
	}
	primaryStyle := rowStyle
	if selected {
		primaryStyle = selectedStyle
	}
	query := strings.TrimSpace(m.input.Value())
	primaryMatchStyle := primaryStyle.Foreground(lipgloss.Color(m.theme.MatchFG)).Bold(true)
	lines := []string{}
	for index, part := range wrapDisplayWidth(label, contentWidth) {
		linePrefix := "  "
		if index == 0 {
			linePrefix = prefix
		}
		lines = append(lines, primaryStyle.Render(linePrefix)+renderFuzzyText(part, query, primaryStyle, primaryMatchStyle))
	}
	return lines
}

func (m PickerModel) renderMatchLabelLine(match notes.Match, entry notes.Entry, width int, selected bool, rowStyle, selectedStyle, detailStyle lipgloss.Style) string {
	prefix := "  "
	if selected {
		prefix = m.theme.SelectedMark + " "
	}
	rowWidth := maxInt(12, width)
	contentWidth := maxInt(8, rowWidth-ansi.StringWidth(prefix))
	labelText := strings.Join(strings.Fields(strings.TrimSpace(match.Label)), " ")

	detailText := match.Detail
	if m.options.ShowAddress {
		detailText = ""
	}
	if m.showInlinePreview() && selected && !entry.HasCmd() {
		preview := m.cachedPreview(entry)
		if selectedSnippet := m.inlinePreviewLine(preview, maxInt(12, width-4), m.activePreviewHit()); selectedSnippet != "" {
			detailText = selectedSnippet
		}
	}
	detailText = strings.Join(strings.Fields(strings.TrimSpace(detailText)), " ")

	primary := labelText
	if primary == "" {
		primary = detailText
	}
	secondary := ""
	if detailText != "" && detailText != labelText {
		secondary = detailText
	}
	combined := compactResultLine(primary, secondary, contentWidth)
	query := strings.TrimSpace(m.input.Value())
	primaryStyle := rowStyle
	if selected {
		primaryStyle = selectedStyle
	}

	if secondary == "" || combined == primary {
		matchStyle := primaryStyle.Foreground(lipgloss.Color(m.theme.MatchFG)).Bold(true)
		rendered := primaryStyle.Render(prefix) + renderFuzzyText(combined, query, primaryStyle, matchStyle)
		padding := rowWidth - ansi.StringWidth(prefix+combined)
		if padding > 0 {
			rendered += primaryStyle.Render(strings.Repeat(" ", padding))
		}
		return rendered
	}

	combinedRunes := []rune(combined)
	split := compactResultSplit(primary, secondary, contentWidth)
	if split > len(combinedRunes) {
		split = len(combinedRunes)
	}
	first := string(combinedRunes[:split])
	second := string(combinedRunes[split:])
	visible := ansi.StringWidth(prefix + first + second)
	if visible < rowWidth {
		second += strings.Repeat(" ", rowWidth-visible)
	}
	primaryMatchStyle := primaryStyle.Foreground(lipgloss.Color(m.theme.MatchFG)).Bold(true)
	detailMatchStyle := detailStyle.Foreground(lipgloss.Color(m.theme.MatchFG)).Bold(true)
	return primaryStyle.Render(prefix) +
		renderFuzzyText(first, query, primaryStyle, primaryMatchStyle) +
		renderFuzzyText(second, query, detailStyle, detailMatchStyle)
}

func renderFuzzyText(text, query string, baseStyle, matchStyle lipgloss.Style) string {
	indexes := fuzzyRuneIndexes(text, query)
	if len(indexes) == 0 {
		return baseStyle.Render(text)
	}
	var out strings.Builder
	for index, char := range []rune(text) {
		style := baseStyle
		if indexes[index] {
			style = matchStyle
		}
		out.WriteString(style.Render(string(char)))
	}
	return out.String()
}

func fuzzyRuneIndexes(text, query string) map[int]bool {
	textRunes := []rune(strings.ToLower(text))
	indexes := map[int]bool{}
	for _, term := range strings.Fields(strings.ToLower(query)) {
		termRunes := []rune(term)
		if len(termRunes) == 0 {
			continue
		}
		matched := make([]int, 0, len(termRunes))
		termIndex := 0
		for textIndex, char := range textRunes {
			if char != termRunes[termIndex] {
				continue
			}
			matched = append(matched, textIndex)
			termIndex++
			if termIndex == len(termRunes) {
				break
			}
		}
		if termIndex != len(termRunes) {
			continue
		}
		for _, index := range matched {
			indexes[index] = true
		}
	}
	return indexes
}

func compactResultLine(primary, secondary string, width int) string {
	primary = strings.TrimSpace(primary)
	secondary = strings.TrimSpace(secondary)
	if width <= 0 {
		return ""
	}
	if secondary == "" {
		return truncateRunes(primary, width)
	}

	const gap = 3
	const separator = " · "
	primaryWidth := ansi.StringWidth(primary)
	if primaryWidth >= width {
		return primary
	}
	availableSecondaryWidth := width - primaryWidth - gap
	if availableSecondaryWidth <= 0 {
		return primary
	}
	secondary = truncateMiddleRunes(secondary, availableSecondaryWidth)
	secondaryWidth := ansi.StringWidth(secondary)
	if secondaryWidth == 0 {
		return primary
	}
	return primary + separator + secondary
}

func compactResultSplit(primary, secondary string, width int) int {
	primary = strings.TrimSpace(primary)
	secondary = strings.TrimSpace(secondary)
	if width <= 0 || secondary == "" {
		return utf8.RuneCountInString(truncateRunes(primary, width))
	}

	const gap = 3
	primaryWidth := ansi.StringWidth(primary)
	if primaryWidth >= width {
		return utf8.RuneCountInString(primary)
	}
	availableSecondaryWidth := width - primaryWidth - gap
	if availableSecondaryWidth <= 0 {
		return utf8.RuneCountInString(primary)
	}
	secondary = truncateMiddleRunes(secondary, availableSecondaryWidth)
	secondaryWidth := ansi.StringWidth(secondary)
	if secondaryWidth == 0 {
		return utf8.RuneCountInString(primary)
	}
	return utf8.RuneCountInString(primary) + gap
}

func selectedCommandLines(command string, width int) []string {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil
	}
	width = maxInt(8, width-2)
	words := splitDisplayWords(command)
	if len(words) == 0 {
		return nil
	}
	lines := []string{}
	current := ""
	for _, word := range words {
		if current == "" && ansi.StringWidth(word) > width {
			wrapped := wrapDisplayWidth(word, width)
			lines = append(lines, wrapped[:len(wrapped)-1]...)
			current = wrapped[len(wrapped)-1]
			continue
		}
		candidate := word
		if current != "" {
			candidate = current + " " + word
		}
		if current == "" || ansi.StringWidth(candidate) <= width {
			current = candidate
			continue
		}
		lines = append(lines, current)
		if ansi.StringWidth(word) <= width {
			current = word
			continue
		}
		wrapped := wrapDisplayWidth(word, width)
		lines = append(lines, wrapped[:len(wrapped)-1]...)
		current = wrapped[len(wrapped)-1]
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

func fitSelectedCommandLines(command string, lines []string, slots, width int) []string {
	if slots <= 0 || len(lines) == 0 {
		return nil
	}
	if len(lines) <= slots {
		return lines
	}
	if slots == 1 {
		return []string{truncateMiddleRunes(strings.Join(lines, " "), width)}
	}
	words := splitDisplayWords(command)
	if len(words) > 0 {
		tail := wrapDisplayWidth("… "+words[len(words)-1], width)
		if len(tail) <= slots {
			prefixCount := slots - len(tail)
			out := append([]string(nil), lines[:prefixCount]...)
			return append(out, tail...)
		}
	}
	out := append([]string(nil), lines[:slots-1]...)
	last := "… " + truncateLeftWidth(lines[len(lines)-1], maxInt(1, width-2))
	out = append(out, last)
	return out
}

func splitDisplayWords(value string) []string {
	words := []string{}
	var word strings.Builder
	var quote rune
	escaped := false
	flush := func() {
		if word.Len() > 0 {
			words = append(words, word.String())
			word.Reset()
		}
	}
	for _, char := range value {
		if escaped {
			word.WriteRune(char)
			escaped = false
			continue
		}
		if char == '\\' {
			word.WriteRune(char)
			escaped = true
			continue
		}
		if quote != 0 {
			word.WriteRune(char)
			if char == quote {
				quote = 0
			}
			continue
		}
		if char == '\'' || char == '"' {
			quote = char
			word.WriteRune(char)
			continue
		}
		if char == ' ' || char == '	' || char == '\n' || char == '\r' {
			flush()
			continue
		}
		word.WriteRune(char)
	}
	flush()
	return words
}

func wrapDisplayWidthPreferSpaces(value string, width int) []string {
	if width <= 0 || ansi.StringWidth(value) <= width {
		return []string{value}
	}
	lines := []string{}
	remaining := value
	for ansi.StringWidth(remaining) > width {
		graphemes := uniseg.NewGraphemes(remaining)
		usedWidth := 0
		cut := 0
		spaceCut := 0
		for graphemes.Next() {
			cluster := graphemes.Str()
			clusterWidth := ansi.StringWidth(cluster)
			if cut > 0 && usedWidth+clusterWidth > width {
				break
			}
			usedWidth += clusterWidth
			_, end := graphemes.Positions()
			cut = end
			if strings.TrimSpace(cluster) == "" {
				spaceCut = end
			}
		}
		if spaceCut > 0 {
			cut = spaceCut
		}
		if cut <= 0 || cut >= len(remaining) {
			break
		}
		lines = append(lines, remaining[:cut])
		remaining = remaining[cut:]
	}
	lines = append(lines, remaining)
	return lines
}

func wrapDisplayWidth(value string, width int) []string {
	if width <= 0 {
		return []string{value}
	}
	graphemes := uniseg.NewGraphemes(value)
	lines := []string{}
	var line strings.Builder
	lineWidth := 0
	for graphemes.Next() {
		cluster := graphemes.Str()
		clusterWidth := ansi.StringWidth(cluster)
		if lineWidth > 0 && lineWidth+clusterWidth > width {
			lines = append(lines, line.String())
			line.Reset()
			lineWidth = 0
		}
		line.WriteString(cluster)
		lineWidth += clusterWidth
	}
	if line.Len() > 0 {
		lines = append(lines, line.String())
	}
	return lines
}

func (m *PickerModel) moveCursor(delta int) {
	if len(m.matches) == 0 || delta == 0 {
		return
	}
	target := m.cursor + delta
	if target < 0 || target >= len(m.matches) {
		return
	}
	m.cursor = target
	m.previewHit = 0
	m.refreshPreview()
}

func (m *PickerModel) cachedPreview(entry notes.Entry) notes.PreviewMatch {
	if m.previewCache == nil {
		m.previewCache = make(map[string]notes.PreviewMatch)
	}
	key := previewCacheKey(entry, m.input.Value())
	if preview, ok := m.previewCache[key]; ok {
		return preview
	}
	if len(m.previewCache) > 512 {
		m.previewCache = make(map[string]notes.PreviewMatch)
	}
	preview := notes.BuildPreview(entry, m.input.Value())
	m.previewCache[key] = preview
	return preview
}

func (m PickerModel) inlinePreviewLine(preview notes.PreviewMatch, width, activeIndex int) string {
	lines := previewPaneLines(preview, maxInt(12, width), 1, activeIndex, m.theme)
	if len(lines) == 0 {
		return ""
	}
	return lines[0]
}

func previewCacheKey(entry notes.Entry, query string) string {
	return entry.SourcePath + "|" + entry.DisplayName() + "|" + query
}

func (m PickerModel) detailLine(detail, hint string, width int, detailStyle, hintStyle lipgloss.Style) string {
	contentWidth := maxInt(12, width-4)
	detail = strings.Join(strings.Fields(strings.TrimSpace(detail)), " ")
	hint = strings.TrimSpace(hint)
	if hint == "" {
		return "    " + detailStyle.Render(truncateRunes(detail, contentWidth))
	}

	hintWidth := utf8.RuneCountInString(hint)
	detailWidth := contentWidth - hintWidth - 2
	if detailWidth < 1 {
		detailWidth = 1
	}
	left := truncateRunes(detail, detailWidth)
	padding := contentWidth - utf8.RuneCountInString(left) - hintWidth
	if padding < 1 {
		padding = 1
	}
	if utf8.RuneCountInString(left)+padding+hintWidth > contentWidth {
		hint = truncateRunes(hint, maxInt(1, contentWidth-utf8.RuneCountInString(left)-1))
		hintWidth = utf8.RuneCountInString(hint)
		padding = contentWidth - utf8.RuneCountInString(left) - hintWidth
		if padding < 1 {
			padding = 1
		}
	}
	return "    " + detailStyle.Render(left) + strings.Repeat(" ", padding) + hintStyle.Render(hint)
}

func (m PickerModel) enterHintText(entry notes.Entry) string {
	if action := entry.QuickAction(); action != nil {
		switch {
		case action.IsShow():
			if entry.IsRaw() {
				return "enter: print " + entry.SourceBadge()
			}
			return "enter: print note"
		case action.IsCmd():
			return "enter: run command"
		}
	}

	if entry.ActionCount() <= 1 {
		return ""
	}
	if entry.HasCmd() {
		if entry.HasShow() {
			return "enter: select action"
		}
		return "enter: select command"
	}
	if entry.HasShow() {
		return "enter: select note"
	}
	return "enter: select action"
}
