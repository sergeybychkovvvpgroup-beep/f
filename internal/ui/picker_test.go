package ui

import (
	"strings"
	"testing"
	"unicode/utf8"

	"f/internal/notes"
	"github.com/charmbracelet/bubbles/cursor"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestCompactPickerUsesAlternateScreenToRestoreTerminalOnExit(t *testing.T) {
	if got := len(pickerProgramOptions(Options{FullScreen: false})); got == 0 {
		t.Fatal("compact picker must use the alternate screen so its UI disappears on exit")
	}
}

func TestPickerUsesSingleLineResultsByDefault(t *testing.T) {
	m := PickerModel{}
	if got := m.resultRowHeight(); got != 1 {
		t.Fatalf("default picker row height = %d, want 1", got)
	}
}

func TestPickerShowsMutedAddressInlineAfterInterpunctWhenEnabled(t *testing.T) {
	entry := notes.Entry{
		Desc:    "gateway",
		Address: "operator@192.0.2.10:2222",
		Kind:    "host",
		Actions: []notes.Action{{Cmd: "ssh gateway"}},
	}
	m := NewPicker([]notes.Entry{entry}, "", DefaultTheme(), Options{Height: 8, ShowAddress: true})
	m.width, m.height = 100, 8
	if got := m.resultRowHeight(); got != 1 {
		t.Fatalf("address mode row height = %d, want 1", got)
	}
	plain := stripANSI(m.View())
	if !strings.Contains(plain, "│ gateway · operator@192.0.2.10:2222") {
		t.Fatalf("address is not rendered inline after an interpunct: %q", plain)
	}
	if strings.Contains(plain, "\n    operator@192.0.2.10:2222") {
		t.Fatalf("address is still rendered on a second line: %q", plain)
	}
}

func TestAddressModeKeepsFuzzyMatchHighlighting(t *testing.T) {
	previousProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previousProfile) })

	theme := DefaultTheme()
	entries := []notes.Entry{
		{
			Desc:    "gateway-one",
			Address: "operator@192.0.2.10:2222",
			Kind:    "host",
			Note:    "gateway-one operator@192.0.2.10:2222",
			Actions: []notes.Action{{Cmd: "ssh gateway-one"}},
		},
		{
			Desc:    "gateway-two",
			Address: "operator@192.0.2.11:2222",
			Kind:    "host",
			Note:    "gateway-two operator@192.0.2.11:2222",
			Actions: []notes.Action{{Cmd: "ssh gateway-two"}},
		},
	}
	m := NewPicker(entries, "gate", theme, Options{Height: 10, ShowAddress: true})
	m.width, m.height = 100, 10
	m.cursor = 1
	expected := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.MatchFG)).Bold(true).Render("g")
	if view := m.View(); !strings.Contains(view, expected) {
		t.Fatalf("address mode lost fuzzy-match highlighting: %q", view)
	}
}

func TestAddressModeFitsMinimumPickerHeight(t *testing.T) {
	entries := []notes.Entry{
		{Desc: "one", Address: "operator@192.0.2.1", Kind: "host", Actions: []notes.Action{{Cmd: "ssh one"}}},
		{Desc: "two", Address: "operator@192.0.2.2", Kind: "host", Actions: []notes.Action{{Cmd: "ssh two"}}},
	}
	m := NewPicker(entries, "", DefaultTheme(), Options{Height: 6, ShowAddress: true})
	m.width, m.height = 100, 6
	plain := stripANSI(m.View())
	if !strings.Contains(plain, "operator@192.0.2.1") || !strings.Contains(plain, "F1–F5 categories") || strings.Contains(plain, "…") {
		t.Fatalf("minimum-height address mode is clipped: %q", plain)
	}
}

func TestDefaultThemeUsesModernFZFAccents(t *testing.T) {
	theme := DefaultTheme()
	if theme.InputPrompt != "#f38ba8" || theme.MatchFG != "#f5c2e7" || theme.SelectedBG != "#313244" {
		t.Fatalf("unexpected fzf palette: %+v", theme)
	}
}

func TestFuzzyRuneIndexesHighlightSubsequence(t *testing.T) {
	got := fuzzyRuneIndexes("sergeyb@185.40.28.20", "sgb")
	want := map[int]bool{0: true, 3: true, 6: true}
	if len(got) != len(want) {
		t.Fatalf("highlighted indexes = %v, want %v", got, want)
	}
	for index := range want {
		if !got[index] {
			t.Fatalf("highlighted indexes = %v, missing %d", got, index)
		}
	}
}

func TestTruncateRunesIgnoresANSISequences(t *testing.T) {
	styled := "\x1b[38;2;243;139;168m>\x1b[0m chash"
	if got := truncateRunes(styled, 20); strings.Contains(got, "…") {
		t.Fatalf("short colored input was truncated: %q", got)
	}
}

func TestPickerInputLineDoesNotPaintBackground(t *testing.T) {
	previousProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previousProfile) })

	m := NewPicker(nil, "", DefaultTheme(), Options{Height: 8})
	m.width, m.height = 100, 8
	if view := m.View(); strings.Contains(view, "\x1b[48;2;32;32;39m") {
		t.Fatalf("input line paints a dark background block on transparent terminals: %q", view)
	}
}

func TestReturningFromDetailsKeepsInputBackgroundTransparent(t *testing.T) {
	previousProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previousProfile) })
	m := NewPicker([]notes.Entry{{Desc: "gateway", Actions: []notes.Action{{Cmd: "ssh gateway"}}}}, "", DefaultTheme(), Options{Height: 8})
	m.width, m.height = 100, 8
	opened, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	closed, _ := opened.(PickerModel).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if view := closed.(PickerModel).View(); strings.Contains(view, "\x1b[48;2;32;32;39m") {
		t.Fatalf("returning from details restored the dark input background: %q", view)
	}
}

func TestPickerHidesBlockCursorArtifact(t *testing.T) {
	m := NewPicker(nil, "", DefaultTheme(), Options{Height: 8})
	if got := m.input.Cursor.Mode(); got != cursor.CursorHide {
		t.Fatalf("input cursor mode = %s, want hidden to avoid a reverse-video block on transparent terminals", got)
	}
}

func TestPickerShowsListBeforeTyping(t *testing.T) {
	m := PickerModel{}
	if !m.shouldRenderResults() {
		t.Fatal("picker must show the complete list before typing, like fzf")
	}
}

func TestShortQueryDoesNotRenderEllipsis(t *testing.T) {
	m := NewPicker(nil, "chash", DefaultTheme(), Options{Height: 8})
	m.width, m.height = 100, 8
	firstLine := strings.Split(stripANSI(m.View()), "\n")[0]
	if strings.Contains(firstLine, "…") {
		t.Fatalf("short query rendered an overflow ellipsis: %q", firstLine)
	}
}

func TestPickerViewUsesMinimalBubbleTeaPanel(t *testing.T) {
	m := NewPicker(nil, "prod", DefaultTheme(), Options{Height: 8})
	m.width = 100
	m.height = 8
	view := stripANSI(m.View())
	for _, required := range []string{"SSH", "F1–F5 categories"} {
		if !strings.Contains(view, required) {
			t.Fatalf("minimal panel is missing %q: %q", required, view)
		}
	}
	for _, forbidden := range []string{"╭", "╰", "SSH NAVIGATOR", "enter ssh", "full command"} {
		if strings.Contains(view, forbidden) {
			t.Fatalf("minimal panel contains obsolete chrome %q: %q", forbidden, view)
		}
	}
	if !strings.Contains(view, "> prod") || !strings.Contains(view, "categories") {
		t.Fatalf("minimal panel is missing query or compact status: %q", view)
	}
}

func TestTabOpensDetailedEntryView(t *testing.T) {
	entry := notes.Entry{
		Desc:       "gateway",
		Address:    "operator@192.0.2.10:2222",
		Kind:       "cmd",
		Mode:       "commands jumps",
		SourcePath: "/home/operator/.ssh/config.d/f_hosts/f.conf",
		SourceLine: 42,
		Actions: []notes.Action{{
			Desc: "show remote routing table",
			Cmd:  "ssh -p 2222 -J operator@jump.example operator@192.0.2.10 show ip route table main",
		}},
	}
	m := NewPicker([]notes.Entry{entry}, "", DefaultTheme(), Options{Height: 18, ShowAddress: true})
	m.width, m.height = 100, 18
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	details := updated.(PickerModel)
	if !details.details {
		t.Fatal("Tab did not open the detailed entry view")
	}
	plain := stripANSI(details.View())
	for _, want := range []string{
		"DETAILS", "gateway", "operator@192.0.2.10:2222", "commands jumps",
		"show remote routing table", "ssh -p 2222 -J operator@jump.example operator@192.0.2.10 show ip route table main",
		"/home/operator/.ssh/config.d/f_hosts/f.conf:42", "Q/Tab back", "Enter run",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("detail view is missing %q: %q", want, plain)
		}
	}
}

func TestTabAndQReturnFromDetailedEntryView(t *testing.T) {
	entry := notes.Entry{Desc: "gateway", Address: "operator@192.0.2.10", Actions: []notes.Action{{Cmd: "ssh gateway"}}}
	m := NewPicker([]notes.Entry{entry}, "", DefaultTheme(), Options{Height: 10})
	opened, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	closedByTab, _ := opened.(PickerModel).Update(tea.KeyMsg{Type: tea.KeyTab})
	if closedByTab.(PickerModel).details {
		t.Fatal("second Tab did not return to the list")
	}
	opened, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	closedByQ, _ := opened.(PickerModel).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	got := closedByQ.(PickerModel)
	if got.details || got.cancelled {
		t.Fatal("q from details must return to the list without cancelling the picker")
	}
}

func TestEscapeDoesNotCloseDetailedEntryView(t *testing.T) {
	entry := notes.Entry{Desc: "gateway", Actions: []notes.Action{{Cmd: "ssh gateway"}}}
	m := NewPicker([]notes.Entry{entry}, "", DefaultTheme(), Options{Height: 10})
	opened, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	updated, cmd := opened.(PickerModel).Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(PickerModel)
	if !got.details || got.cancelled || cmd != nil {
		t.Fatalf("Escape changed details state: details=%v cancelled=%v cmd=%v", got.details, got.cancelled, cmd)
	}
}

func TestEnterRunsSelectedEntryFromDetails(t *testing.T) {
	entry := notes.Entry{Desc: "gateway", Actions: []notes.Action{{Cmd: "ssh gateway"}}}
	m := NewPicker([]notes.Entry{entry}, "", DefaultTheme(), Options{Height: 10})
	opened, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	updated, cmd := opened.(PickerModel).Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(PickerModel)
	if cmd == nil || got.selected == nil || got.selected.Desc != "gateway" {
		t.Fatalf("Enter from details did not select the entry: cmd=%v selected=%+v", cmd, got.selected)
	}
}

func TestDetailedCommandCanScrollToFinalLine(t *testing.T) {
	entry := notes.Entry{
		Desc: "gateway",
		Actions: []notes.Action{{
			Cmd: "ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null operator@192.0.2.10 alpha beta gamma delta epsilon final-token",
		}},
	}
	m := NewPicker([]notes.Entry{entry}, "", DefaultTheme(), Options{Height: 8})
	m.width, m.height = 42, 8
	opened, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	details := opened.(PickerModel)
	if strings.Contains(stripANSI(details.View()), "final-token") {
		t.Fatal("long command unexpectedly fits before scrolling")
	}
	for range 30 {
		updated, _ := details.Update(tea.KeyMsg{Type: tea.KeyDown})
		details = updated.(PickerModel)
	}
	if plain := stripANSI(details.View()); !strings.Contains(plain, "final-token") {
		t.Fatalf("scrolling details did not reveal the full command tail: %q", plain)
	}
}

func TestSelectedRowUsesSlimAccentMarker(t *testing.T) {
	m := NewPicker([]notes.Entry{{Desc: "server", Kind: "host", Actions: []notes.Action{{Cmd: "ssh server"}}}}, "", DefaultTheme(), Options{Height: 8})
	m.width, m.height = 100, 8
	if plain := stripANSI(m.View()); !strings.Contains(plain, "│ server") {
		t.Fatalf("selected row does not use a slim accent marker: %q", plain)
	}
}

func TestCategoryBadgeUsesHumanLabel(t *testing.T) {
	m := NewPicker([]notes.Entry{{Desc: "status", Kind: "cmd", Actions: []notes.Action{{Cmd: "ssh router status"}}}}, "", DefaultTheme(), Options{Height: 8})
	m.width, m.height = 100, 8
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyF3})
	plain := stripANSI(updated.(PickerModel).View())
	if !strings.Contains(plain, "COMMANDS") || strings.Contains(plain, "SSH NAVIGATOR") {
		t.Fatalf("category badge is not reference-style: %q", plain)
	}
}

func TestPickerPanelHasSmallLeftMargin(t *testing.T) {
	m := NewPicker(nil, "", DefaultTheme(), Options{Height: 10})
	m.width, m.height = 80, 10
	for _, line := range strings.Split(stripANSI(m.View()), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !strings.HasPrefix(line, "  ") {
			t.Fatalf("panel line has no two-cell margin: %q", line)
		}
	}
}

func TestPickerPanelStaysAtLeftAndWidthCappedOnWideTerminal(t *testing.T) {
	m := NewPicker([]notes.Entry{{Desc: "server", Kind: "host", Actions: []notes.Action{{Cmd: "ssh server"}}}}, "", DefaultTheme(), Options{Height: 14})
	m.width, m.height = 160, 14
	top := strings.Split(stripANSI(m.View()), "\n")[0]
	leftMargin := len(top) - len(strings.TrimLeft(top, " "))
	panelWidth := utf8.RuneCountInString(strings.TrimSpace(top))
	if leftMargin != 3 {
		t.Fatalf("wide terminal panel visible left margin=%d, want 3 including badge padding: %q", leftMargin, top)
	}
	if panelWidth > 96 {
		t.Fatalf("wide terminal panel width=%d, want at most 96", panelWidth)
	}
}

func TestCompactPickerShrinksToSmallFilteredResultSet(t *testing.T) {
	entries := []notes.Entry{
		{Desc: "one", Kind: "cmd", Actions: []notes.Action{{Cmd: "ssh one"}}},
		{Desc: "two", Kind: "cmd", Actions: []notes.Action{{Cmd: "ssh two"}}},
		{Desc: "three", Kind: "cmd", Actions: []notes.Action{{Cmd: "ssh three"}}},
		{Desc: "four", Kind: "cmd", Actions: []notes.Action{{Cmd: "ssh four"}}},
	}
	m := NewPicker(entries, "", DefaultTheme(), Options{Layout: "top", Height: 14})
	m.width, m.height = 160, 14
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyF3})
	lines := strings.Split(stripANSI(updated.(PickerModel).View()), "\n")
	if len(lines) != 7 {
		t.Fatalf("four-result compact panel has %d lines, want 7 without empty vertical space", len(lines))
	}
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			t.Fatalf("flat compact picker contains an empty gap: %q", strings.Join(lines, "\\n"))
		}
	}
}

func TestCompactTopPlacesQueryBeforeResults(t *testing.T) {
	entries := []notes.Entry{{Desc: "server", Cmd: "ssh server"}}
	m := NewPicker(entries, "", DefaultTheme(), Options{Layout: "top", Height: 6})
	m.width, m.height = 80, 6
	plain := stripANSI(m.View())
	if strings.Index(plain, "> ") > strings.Index(plain, "server") {
		t.Fatalf("top layout must place query before results: %q", plain)
	}
}

func TestCompactBottomPlacesQueryAfterResults(t *testing.T) {
	entries := []notes.Entry{{Desc: "server", Cmd: "ssh server"}}
	m := NewPicker(entries, "", DefaultTheme(), Options{Layout: "bottom", Height: 7})
	m.width, m.height = 80, 7
	plain := stripANSI(m.View())
	if strings.LastIndex(plain, "> ") < strings.Index(plain, "server") {
		t.Fatalf("bottom layout must place query after results: %q", plain)
	}
}

func TestCompactBottomDownMovesSelectionTopToBottom(t *testing.T) {
	entries := []notes.Entry{
		{Desc: "alpha", Cmd: "ssh alpha"},
		{Desc: "bravo", Cmd: "ssh bravo"},
	}
	m := NewPicker(entries, "", DefaultTheme(), Options{Layout: "bottom", Height: 6})
	if m.cursor != 0 {
		t.Fatalf("initial cursor = %d, want first result", m.cursor)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	got := updated.(PickerModel)
	if got.cursor != 1 {
		t.Fatalf("cursor after Down = %d, want second result", got.cursor)
	}
}

func TestCompactBottomRendersResultsTopToBottom(t *testing.T) {
	entries := []notes.Entry{
		{Desc: "alpha", Cmd: "ssh alpha"},
		{Desc: "bravo", Cmd: "ssh bravo"},
	}
	m := NewPicker(entries, "", DefaultTheme(), Options{Layout: "bottom", Height: 7})
	m.width, m.height = 80, 7
	plain := stripANSI(m.View())
	alpha := strings.Index(plain, "alpha")
	bravo := strings.Index(plain, "bravo")
	if alpha < 0 || bravo < 0 || alpha > bravo {
		t.Fatalf("compact bottom results must start at the top and continue downward: %q", plain)
	}
}

func TestFullScreenKeepsResultsInSearchOrder(t *testing.T) {
	entries := []notes.Entry{
		{Desc: "alpha", Cmd: "ssh alpha"},
		{Desc: "bravo", Cmd: "ssh bravo"},
	}
	m := NewPicker(entries, "", DefaultTheme(), Options{FullScreen: true, Layout: "bottom", Height: 8})
	m.width, m.height = 80, 8
	plain := stripANSI(m.View())
	alpha := strings.Index(plain, "alpha")
	bravo := strings.Index(plain, "bravo")
	if alpha < 0 || bravo < 0 || alpha > bravo {
		t.Fatalf("full-screen results must preserve search order: %q", plain)
	}
}

func TestPickerSearchesAllEntryKindsWithoutPrefix(t *testing.T) {
	entries := []notes.Entry{
		{Desc: "router login", Kind: "host", Actions: []notes.Action{{Cmd: "ssh router"}}},
		{Desc: "router status", Kind: "cmd", Actions: []notes.Action{{Cmd: "ssh router show version"}}},
		{Desc: "router admin", Kind: "fwd", Actions: []notes.Action{{Cmd: "ssh router-forward"}}},
		{Desc: "router production", Kind: "jump", Actions: []notes.Action{{Cmd: "ssh router-production"}}},
	}
	m := NewPicker(entries, "router", DefaultTheme(), Options{})
	if got := len(m.matches); got != len(entries) {
		t.Fatalf("plain query contains %d/%d kinds; want search across all categories", got, len(entries))
	}
}

func TestPickerDoesNotRenderKindPrefixes(t *testing.T) {
	entries := []notes.Entry{
		{Desc: "server", Kind: "host", Actions: []notes.Action{{Cmd: "ssh server"}}},
		{Desc: "status", Kind: "cmd", Actions: []notes.Action{{Cmd: "ssh router show version"}}},
		{Desc: "admin", Kind: "fwd", Actions: []notes.Action{{Cmd: "ssh admin-forward"}}},
		{Desc: "production", Kind: "jump", Actions: []notes.Action{{Cmd: "ssh production"}}},
	}
	m := NewPicker(entries, "", DefaultTheme(), Options{Height: 8})
	m.width, m.height = 100, 8
	plain := stripANSI(m.View())
	for _, prefix := range []string{"host:", "cmd:", "fwd:", "jump:"} {
		if strings.Contains(plain, prefix) {
			t.Fatalf("picker still renders obsolete %q prefix: %q", prefix, plain)
		}
	}
}

func TestFunctionKeysFilterCategoriesWithoutChangingQuery(t *testing.T) {
	entries := []notes.Entry{
		{Desc: "router login", Kind: "host", Actions: []notes.Action{{Cmd: "ssh router"}}},
		{Desc: "router status", Kind: "cmd", Actions: []notes.Action{{Cmd: "ssh router show version"}}},
		{Desc: "router admin", Kind: "fwd", Actions: []notes.Action{{Cmd: "ssh router-forward"}}},
		{Desc: "router production", Kind: "jump", Actions: []notes.Action{{Cmd: "ssh router-production"}}},
		{Desc: "router chained", Kind: "cmd/jump", Actions: []notes.Action{{Cmd: "ssh router-chained"}}},
	}
	tests := []struct {
		key  tea.KeyType
		kind string
		want int
	}{
		{tea.KeyF1, "all", 5},
		{tea.KeyF2, "host", 1},
		{tea.KeyF3, "cmd", 2},
		{tea.KeyF4, "fwd", 1},
		{tea.KeyF5, "jump", 2},
	}
	for _, tt := range tests {
		m := NewPicker(entries, "router", DefaultTheme(), Options{})
		updated, _ := m.Update(tea.KeyMsg{Type: tt.key})
		got := updated.(PickerModel)
		if got.activeKind != tt.kind {
			t.Fatalf("%s selected %q, want %q", tt.key, got.activeKind, tt.kind)
		}
		if len(got.matches) != tt.want {
			t.Fatalf("%s returned %d matches, want %d", tt.key, len(got.matches), tt.want)
		}
		if got.Query() != "router" {
			t.Fatalf("%s changed query to %q", tt.key, got.Query())
		}
	}
}

func TestPickerStatusUsesCompactCategoryHint(t *testing.T) {
	m := NewPicker(nil, "", DefaultTheme(), Options{Height: 8})
	m.width, m.height = 100, 8
	plain := stripANSI(m.View())
	for _, item := range []string{"F1–F5 categories", "Tab details"} {
		if !strings.Contains(plain, item) {
			t.Fatalf("compact status is missing %q: %q", item, plain)
		}
	}
	for _, noisy := range []string{"F1 all", "F2 hosts", "F3 commands", "F4 forwards", "F5 jumps"} {
		if strings.Contains(plain, noisy) {
			t.Fatalf("compact status still contains expanded hint %q: %q", noisy, plain)
		}
	}
	if got := len(strings.Split(plain, "\n")); got != 4 {
		t.Fatalf("empty compact picker has %d lines, want 4 without filler", got)
	}
}

func TestPickerHeaderHidesRedundantAllCountAndHealthySync(t *testing.T) {
	entries := []notes.Entry{
		{Desc: "one", Actions: []notes.Action{{Cmd: "ssh one"}}},
		{Desc: "two", Actions: []notes.Action{{Cmd: "ssh two"}}},
	}
	m := NewPicker(entries, "", DefaultTheme(), Options{Height: 8, InitialSync: SyncStatus{State: SyncStateOK}})
	m.width, m.height = 100, 8
	header := strings.Split(stripANSI(m.View()), "\n")[0]
	fields := strings.Fields(header)
	if len(fields) < 2 || fields[0] != "SSH" || fields[1] != "2" || strings.Contains(header, "2 / 2") || strings.Contains(header, "sync ok") {
		t.Fatalf("header still contains redundant healthy-state metadata: %q", header)
	}
}

func TestRunningSyncUsesBubblesSpinner(t *testing.T) {
	m := NewPicker(nil, "", DefaultTheme(), Options{InitialSync: SyncStatus{State: SyncStateRunning}})
	if got := stripANSI(m.renderSyncStatus()); !strings.Contains(got, "⠋") || !strings.Contains(got, "sync") {
		t.Fatalf("running sync status does not use spinner bubble: %q", got)
	}
}

func TestDetailCommandWrappingPreservesExactCommand(t *testing.T) {
	command := `ssh host "echo hello  world" | tee /tmp/result\ file`
	lines := detailCommandLines(command, 18)
	if got := strings.Join(lines, ""); got != command {
		t.Fatalf("wrapped command changed bytes:\n got %q\nwant %q", got, command)
	}
	if strings.Contains(strings.Join(lines, "\n"), "\\\n") {
		t.Fatalf("wrapped command inserted shell continuation characters: %q", lines)
	}
}

func TestDetailScrollStopsAtBottomAndMovesUpImmediately(t *testing.T) {
	entry := notes.Entry{Desc: "gateway", Actions: []notes.Action{{Cmd: strings.Repeat("0123456789", 20)}}}
	m := NewPicker([]notes.Entry{entry}, "", DefaultTheme(), Options{Height: 8})
	m.width, m.height = 36, 8
	opened, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	details := opened.(PickerModel)
	for range 30 {
		updated, _ := details.Update(tea.KeyMsg{Type: tea.KeyDown})
		details = updated.(PickerModel)
	}
	atBottom := stripANSI(details.View())
	updated, _ := details.Update(tea.KeyMsg{Type: tea.KeyUp})
	afterUp := stripANSI(updated.(PickerModel).View())
	if afterUp == atBottom {
		t.Fatal("Up did not move immediately after scrolling to the bottom")
	}
}

func TestCtrlCQuitsFromDetails(t *testing.T) {
	m := NewPicker([]notes.Entry{{Desc: "gateway", Actions: []notes.Action{{Cmd: "ssh gateway"}}}}, "", DefaultTheme(), Options{Height: 8})
	opened, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	updated, cmd := opened.(PickerModel).Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	got := updated.(PickerModel)
	if cmd == nil || !got.cancelled {
		t.Fatalf("Ctrl+C in details did not quit: cmd=%v cancelled=%t", cmd, got.cancelled)
	}
}

func TestCtrlYPrintsSelectedEntryFromDetails(t *testing.T) {
	m := NewPicker([]notes.Entry{{Desc: "gateway", Actions: []notes.Action{{Cmd: "ssh gateway"}}}}, "", DefaultTheme(), Options{Height: 8})
	opened, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	updated, cmd := opened.(PickerModel).Update(tea.KeyMsg{Type: tea.KeyCtrlY})
	got := updated.(PickerModel)
	if cmd == nil || got.selected == nil || !got.printOnly {
		t.Fatalf("Ctrl+Y in details did not select for printing: cmd=%v selected=%+v printOnly=%t", cmd, got.selected, got.printOnly)
	}
}

func TestDetailsReturnPreservesPickerState(t *testing.T) {
	entries := []notes.Entry{
		{Desc: "alpha", Kind: "host", Actions: []notes.Action{{Cmd: "ssh alpha"}}},
		{Desc: "beta", Kind: "host", Actions: []notes.Action{{Cmd: "ssh beta"}}},
	}
	m := NewPicker(entries, "a", DefaultTheme(), Options{Height: 8})
	m.cursor = 1
	m.activeKind = "host"
	opened, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	closed, _ := opened.(PickerModel).Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := closed.(PickerModel)
	if got.Query() != "a" || got.cursor != 1 || got.activeKind != "host" || got.cancelled {
		t.Fatalf("picker state changed after details: query=%q cursor=%d kind=%q cancelled=%t", got.Query(), got.cursor, got.activeKind, got.cancelled)
	}
}

func TestDetailViewFitsNarrowPanel(t *testing.T) {
	entry := notes.Entry{Desc: "gateway", Address: "operator@192.0.2.10:2222", Actions: []notes.Action{{Cmd: strings.Repeat("x", 100)}}}
	m := NewPicker([]notes.Entry{entry}, "", DefaultTheme(), Options{Height: 8})
	m.width, m.height = 42, 8
	opened, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	for _, line := range strings.Split(stripANSI(opened.(PickerModel).View()), "\n") {
		if utf8.RuneCountInString(strings.TrimRight(line, " ")) > 40 {
			t.Fatalf("detail line exceeds panel width: %q", line)
		}
	}
}

func stripANSI(value string) string {
	for {
		start := strings.IndexByte(value, '\x1b')
		if start < 0 {
			return value
		}
		end := strings.IndexByte(value[start:], 'm')
		if end < 0 {
			return value[:start]
		}
		value = value[:start] + value[start+end+1:]
	}
}
