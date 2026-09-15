package ui

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"aoo/internal/notes"
	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type PickerModel struct {
	input        textinput.Model
	entries      []notes.Entry
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
	input.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.TitleDimFG))
	input.PlaceholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.HelpFG))
	input.Cursor.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.RowFG))
	input.Cursor.SetMode(cursor.CursorStatic)

	m := PickerModel{
		input:        input,
		entries:      entries,
		theme:        theme,
		options:      options,
		previewCache: make(map[string]notes.PreviewMatch),
		syncStatus:   options.InitialSync,
		syncStream:   options.SyncStatusStream,
	}
	m.refresh()
	return m
}

func (m PickerModel) Init() tea.Cmd {
	cmds := []tea.Cmd{}
	if m.syncStream != nil {
		cmds = append(cmds, m.pollSyncStatus())
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
	case tea.KeyMsg:
		switch msg.String() {
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
		case "f3":
			m.moveCursor(1)
			return m, nil
		case "shift+f3":
			m.moveCursor(-1)
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.refresh()
	return m, cmd
}

func (m PickerModel) View() string {
	bg := lipgloss.Color(m.theme.InputBG)
	rowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.RowFG)).Background(bg)
	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.theme.SelectedFG)).
		Background(lipgloss.Color(m.theme.SelectedBG))
	detailStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.DetailFG)).Background(bg)
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.HelpFG)).Background(bg)
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.TitleFG)).Background(bg)
	containerStyle := lipgloss.NewStyle().Width(m.contentWidth()).MaxWidth(m.contentWidth()).Background(bg)
	contentWidth := m.contentWidth()
	effectiveHeight := m.effectiveHeight()

	input := m.input
	input.Width = m.inputWidth()
	inputLine := truncateRunes(input.View(), contentWidth)
	if m.useRightPreview(contentWidth) {
		return m.shelfView(contentWidth, effectiveHeight, inputLine, rowStyle, selectedStyle, detailStyle, helpStyle, titleStyle)
	}
	lines := make([]string, 0, maxInt(6, effectiveHeight))

	if m.isBottomLayout() {
		resultBlock := []string{}
		if m.shouldRenderResults() {
			bodyHeight := maxInt(4, effectiveHeight-4)
			resultBlock = append(resultBlock, m.resultBlock(contentWidth, bodyHeight, rowStyle, selectedStyle, detailStyle, helpStyle)...)
			resultBlock = append(resultBlock, "")
		}
		lines = append(lines, resultBlock...)
		lines = append(lines, truncateRunes(m.renderStatusBar(titleStyle), contentWidth))
		if !m.options.FocusMode {
			lines = append(lines, helpStyle.Render(truncateRunes(pickerHelpText(), contentWidth)))
		}
		lines = append(lines, inputLine)
		if effectiveHeight > 0 && len(lines) < effectiveHeight {
			padding := make([]string, 0, effectiveHeight-len(lines))
			for len(lines)+len(padding) < effectiveHeight {
				padding = append(padding, "")
			}
			lines = append(padding, lines...)
		}
	} else {
		lines = append(lines, inputLine)
		if m.shouldRenderResults() {
			lines = append(lines, "")
			bodyHeight := maxInt(4, effectiveHeight-5)
			lines = append(lines, m.resultBlock(contentWidth, bodyHeight, rowStyle, selectedStyle, detailStyle, helpStyle)...)
		}
		lines = append(lines, "")
		lines = append(lines, truncateRunes(m.renderStatusBar(titleStyle), contentWidth))
		if !m.options.FocusMode {
			lines = append(lines, helpStyle.Render(truncateRunes(pickerHelpText(), contentWidth)))
		}
		if effectiveHeight > 0 && len(lines) < effectiveHeight {
			fillerAt := len(lines) - 1
			padding := make([]string, 0, effectiveHeight-len(lines))
			for len(lines)+len(padding) < effectiveHeight {
				padding = append(padding, "")
			}
			lines = append(lines[:fillerAt], append(padding, lines[fillerAt:]...)...)
		}
	}

	lines = normalizeRenderedLines(lines, contentWidth)
	mainView := containerStyle.
		Height(maxInt(4, effectiveHeight)).
		MaxHeight(maxInt(4, effectiveHeight)).
		Render(strings.Join(lines, "\n"))
	return mainView
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
	if strings.TrimSpace(m.input.Value()) != "" {
		return true
	}
	return m.options.ShowListOnStart
}

func (m *PickerModel) refresh() {
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
	available := m.effectiveHeight() - 4
	if available < 6 {
		available = 6
	}
	maxItems := available / m.resultRowHeight()
	if maxItems < 2 {
		maxItems = 2
	}
	return maxItems
}

func RunPicker(entries []notes.Entry, initialQuery string, themeName string, options Options) (*notes.Entry, int, string, bool, bool, bool, string, error) {
	theme, err := ResolveTheme(themeName)
	if err != nil {
		return nil, 0, initialQuery, false, false, false, "", err
	}

	model := NewPicker(entries, initialQuery, theme, options)
	programOptions := []tea.ProgramOption{}
	if options.FullScreen {
		programOptions = append(programOptions, tea.WithAltScreen())
	}
	program := tea.NewProgram(model, programOptions...)
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
	if m.width <= 0 {
		return 80
	}
	if m.width < 20 {
		return 20
	}
	return m.width - 4
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

func (m PickerModel) inputWidth() int {
	width := m.contentWidth() - lipgloss.Width(m.input.Prompt)
	if width < 1 {
		return 1
	}
	return width
}

func (m PickerModel) isBottomLayout() bool {
	return strings.EqualFold(strings.TrimSpace(m.options.Layout), "bottom")
}

func (m PickerModel) showInlinePreview() bool {
	return m.options.ShowMatchContext
}

func (m PickerModel) resultRowHeight() int {
	if !m.twoLineResults() {
		return 1
	}
	return 2
}

func (m PickerModel) twoLineResults() bool {
	return !m.options.SingleLineResults
}

func (m PickerModel) statusLine() string {
	status := fmt.Sprintf("%d/%d", len(m.matches), len(m.entries))
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
	line := baseStyle.Render(m.statusLine())
	if sync := m.renderSyncStatus(); sync != "" {
		line += baseStyle.Render("  ·  ") + sync
	}
	return line
}

func (m PickerModel) renderSyncStatus() string {
	label := strings.TrimSpace(m.syncStatusLabel())
	if label == "" {
		return ""
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
	return string([]rune{0x2191, 0x2193}) + " select  enter ssh  ctrl+n add new login  alt+enter/ctrl+y print  esc quit"
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
	if utf8.RuneCountInString(value) <= limit {
		return value
	}
	if limit <= 1 {
		return "…"
	}

	runes := []rune(value)
	return string(runes[:limit-1]) + "…"
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

func (m PickerModel) shelfView(width, height int, inputLine string, rowStyle, selectedStyle, detailStyle, helpStyle, titleStyle lipgloss.Style) string {
	if height < 8 {
		height = 8
	}
	chrome := lipgloss.Color(m.theme.InputBorder)
	bg := lipgloss.Color(m.theme.InputBG)
	queryText := titleStyle.Render("aoo  "+m.statusLine()+"  ") + inputLine
	queryBox := lipgloss.NewStyle().
		Width(maxInt(1, width-2)).
		Height(1).
		Border(lipgloss.NormalBorder()).
		BorderForeground(chrome).
		Background(bg).
		Render(queryText)

	bodyHeight := maxInt(4, height-4)
	compact := m
	compact.options.SingleLineResults = true
	compact.options.Layout = "top"
	body := compact.resultBlock(width, bodyHeight, rowStyle, selectedStyle, detailStyle, helpStyle)

	help := helpStyle.Width(width).Render(truncateRunes(pickerHelpText(), width))
	lines := append([]string{queryBox}, body...)
	lines = append(lines, help)
	lines = paintBackground(lines, width, lipgloss.NewStyle().Background(bg))
	return lipgloss.NewStyle().Width(width).Background(bg).Render(strings.Join(lines, "\n"))
}

func (m PickerModel) resultBlock(width, height int, rowStyle, selectedStyle, detailStyle, hintStyle lipgloss.Style) []string {
	if !m.useRightPreview(width) {
		return m.resultLines(width, rowStyle, selectedStyle, detailStyle, hintStyle)
	}

	gap := 1
	previewWidth := width / 2
	if previewWidth < 42 {
		previewWidth = 42
	}
	if previewWidth > 84 {
		previewWidth = 84
	}
	listWidth := width - previewWidth - gap
	if listWidth < 32 {
		return m.resultLines(width, rowStyle, selectedStyle, detailStyle, hintStyle)
	}

	left := m.resultLines(listWidth-2, rowStyle, selectedStyle, detailStyle, hintStyle)
	right := m.previewLines(previewWidth-2, height-2, detailStyle, hintStyle)
	bg := lipgloss.Color(m.theme.InputBG)
	boxStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(m.theme.InputBorder)).Background(bg)
	leftBox := boxStyle.Width(maxInt(1, listWidth-2)).Height(maxInt(1, height-2)).Render(strings.Join(fillLines(left, height-2, listWidth-2), "\n"))
	rightBox := boxStyle.Width(maxInt(1, previewWidth-2)).Height(maxInt(1, height-2)).Render(strings.Join(fillLines(right, height-2, previewWidth-2), "\n"))
	gapText := lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", gap))
	joined := strings.Split(lipgloss.JoinHorizontal(lipgloss.Top, leftBox, gapText, rightBox), "\n")
	return paintBackground(joined, width, lipgloss.NewStyle().Background(bg))
}

func (m PickerModel) useRightPreview(width int) bool {
	return width >= 100 && m.shouldRenderResults()
}

func (m PickerModel) previewLines(width, height int, detailStyle, hintStyle lipgloss.Style) []string {
	if len(m.matches) == 0 {
		return []string{detailStyle.Render("No selection")}
	}
	entry := m.matches[m.cursor].Entry
	action := entry.QuickAction()
	lines := []string{hintStyle.Render(truncateRunes(entry.DisplayName(), width))}
	if action != nil {
		if strings.TrimSpace(action.Desc) != "" && strings.TrimSpace(action.Desc) != "ssh" {
			lines = append(lines, detailStyle.Render(truncateRunes(action.Desc, width)))
		}
		if strings.TrimSpace(action.Cmd) != "" {
			lines = append(lines, "")
			lines = append(lines, hintStyle.Render("command"))
			for _, line := range wrapText(action.Cmd, width) {
				lines = append(lines, detailStyle.Render(line))
			}
		}
		if strings.TrimSpace(action.Banner) != "" {
			lines = append(lines, "")
			lines = append(lines, hintStyle.Render("banner"))
			for _, line := range wrapText(action.Banner, width) {
				lines = append(lines, detailStyle.Render(line))
			}
		}
	}
	if strings.TrimSpace(entry.Note) != "" {
		lines = append(lines, "")
		lines = append(lines, hintStyle.Render("search"))
		for _, line := range wrapText(entry.Note, width) {
			lines = append(lines, detailStyle.Render(line))
		}
	}
	return clipLines(lines, height)
}

func fillLines(lines []string, height, width int) []string {
	out := clipLines(lines, height)
	for len(out) < height {
		out = append(out, strings.Repeat(" ", maxInt(0, width)))
	}
	for i := range out {
		visible := lipgloss.Width(out[i])
		if visible < width {
			out[i] += strings.Repeat(" ", width-visible)
		}
	}
	return out
}

func paintBackground(lines []string, width int, style lipgloss.Style) []string {
	out := make([]string, len(lines))
	for i, line := range lines {
		visible := lipgloss.Width(line)
		if visible < width {
			line += strings.Repeat(" ", width-visible)
		}
		out[i] = style.Render(line)
	}
	return out
}

func (m PickerModel) resultLines(width int, rowStyle, selectedStyle, detailStyle, hintStyle lipgloss.Style) []string {
	if len(m.matches) == 0 {
		return []string{detailStyle.Render("No matches")}
	}

	visible := m.visibleMatches()
	type renderedRow struct {
		lines []string
	}
	rows := make([]renderedRow, 0, len(visible))
	for i, match := range visible {
		index := i + m.offset()
		entry := match.Entry
		selected := index == m.cursor
		rowLines := []string{m.renderMatchLabelLine(match, entry, width, selected, rowStyle, selectedStyle, detailStyle)}

		snippet := match.Detail
		if m.showInlinePreview() && index == m.cursor && !entry.HasCmd() {
			preview := m.cachedPreview(entry)
			if selectedSnippet := m.inlinePreviewLine(preview, width-4, m.activePreviewHit()); selectedSnippet != "" {
				snippet = selectedSnippet
			}
		}
		if m.twoLineResults() {
			rowLines = append(rowLines, m.detailLine(snippet, m.enterHintText(entry), width, detailStyle, hintStyle))
		}
		rows = append(rows, renderedRow{lines: rowLines})
	}

	lines := make([]string, 0, len(visible)*maxInt(2, m.resultRowHeight()))
	if m.isBottomLayout() {
		for i := len(rows) - 1; i >= 0; i-- {
			lines = append(lines, rows[i].lines...)
		}
		return lines
	}
	for _, row := range rows {
		lines = append(lines, row.lines...)
	}
	return lines
}

func (m PickerModel) renderMatchLabelLine(match notes.Match, entry notes.Entry, width int, selected bool, rowStyle, selectedStyle, detailStyle lipgloss.Style) string {
	prefix := "  "
	if selected {
		prefix = m.theme.SelectedMark + " "
	}
	contentWidth := maxInt(12, width-3)
	labelText := strings.Join(strings.Fields(strings.TrimSpace(match.Label)), " ")
	if m.twoLineResults() {
		labelText = truncateRunes(labelText, contentWidth)
		if selected {
			return prefix + selectedStyle.Render(padRight(labelText, contentWidth))
		}
		return prefix + rowStyle.Render(labelText)
	}

	detailText := match.Detail
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
	if selected {
		return prefix + selectedStyle.Render(padRight(combined, contentWidth))
	}

	if secondary == "" || combined == primary {
		return prefix + rowStyle.Render(combined)
	}

	combinedRunes := []rune(combined)
	split := compactResultSplit(primary, secondary, contentWidth)
	if split > len(combinedRunes) {
		split = len(combinedRunes)
	}
	return prefix + rowStyle.Render(string(combinedRunes[:split])) + detailStyle.Render(string(combinedRunes[split:]))
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
	const minPrimaryWidth = 18
	const separator = " · "
	if width <= minPrimaryWidth {
		return truncateRunes(primary, width)
	}

	maxSecondaryWidth := width / 2
	if maxSecondaryWidth < 16 {
		maxSecondaryWidth = 16
	}
	if maxSecondaryWidth > 48 {
		maxSecondaryWidth = 48
	}

	availableSecondaryWidth := width - minPrimaryWidth - gap
	if availableSecondaryWidth < 0 {
		availableSecondaryWidth = 0
	}
	if maxSecondaryWidth > availableSecondaryWidth {
		maxSecondaryWidth = availableSecondaryWidth
	}

	secondary = truncateRunes(secondary, maxSecondaryWidth)
	secondaryWidth := utf8.RuneCountInString(secondary)
	if secondaryWidth == 0 {
		return truncateRunes(primary, width)
	}

	primaryWidth := width - secondaryWidth - gap
	if primaryWidth < 1 {
		return truncateRunes(primary, width)
	}
	return truncateRunes(primary, primaryWidth) + separator + secondary
}

func compactResultSplit(primary, secondary string, width int) int {
	primary = strings.TrimSpace(primary)
	secondary = strings.TrimSpace(secondary)
	if width <= 0 || secondary == "" {
		return utf8.RuneCountInString(truncateRunes(primary, width))
	}

	const gap = 3
	const minPrimaryWidth = 18
	if width <= minPrimaryWidth {
		return utf8.RuneCountInString(truncateRunes(primary, width))
	}

	maxSecondaryWidth := width / 2
	if maxSecondaryWidth < 16 {
		maxSecondaryWidth = 16
	}
	if maxSecondaryWidth > 48 {
		maxSecondaryWidth = 48
	}

	availableSecondaryWidth := width - minPrimaryWidth - gap
	if availableSecondaryWidth < 0 {
		availableSecondaryWidth = 0
	}
	if maxSecondaryWidth > availableSecondaryWidth {
		maxSecondaryWidth = availableSecondaryWidth
	}

	secondary = truncateRunes(secondary, maxSecondaryWidth)
	secondaryWidth := utf8.RuneCountInString(secondary)
	if secondaryWidth == 0 {
		return utf8.RuneCountInString(truncateRunes(primary, width))
	}

	primaryWidth := width - secondaryWidth - gap
	if primaryWidth < 1 {
		return utf8.RuneCountInString(truncateRunes(primary, width))
	}
	left := truncateRunes(primary, primaryWidth)
	return utf8.RuneCountInString(left) + gap
}

func (m *PickerModel) moveCursor(delta int) {
	if len(m.matches) == 0 || delta == 0 {
		return
	}
	target := m.cursor + delta
	if m.isBottomLayout() {
		target = m.cursor - delta
	}
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
