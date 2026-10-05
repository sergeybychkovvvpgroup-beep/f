package ui

import (
	"strings"
	"testing"
	"unicode/utf8"

	"f/internal/notes"
	tea "github.com/charmbracelet/bubbletea"
)

func TestPickerAlwaysUsesSingleLineResults(t *testing.T) {
	m := PickerModel{}
	if got := m.resultRowHeight(); got != 1 {
		t.Fatalf("picker row height = %d, want 1", got)
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
	for _, required := range []string{"SSH", "F1 all"} {
		if !strings.Contains(view, required) {
			t.Fatalf("minimal panel is missing %q: %q", required, view)
		}
	}
	for _, forbidden := range []string{"╭", "╰", "SSH NAVIGATOR", "enter ssh", "full command"} {
		if strings.Contains(view, forbidden) {
			t.Fatalf("minimal panel contains obsolete chrome %q: %q", forbidden, view)
		}
	}
	if !strings.Contains(view, "> prod") || !strings.Contains(view, "all") {
		t.Fatalf("minimal panel is missing query or compact status: %q", view)
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

func TestPickerStatusShowsFunctionKeyCategoryLegend(t *testing.T) {
	m := NewPicker(nil, "", DefaultTheme(), Options{Height: 8})
	m.width, m.height = 100, 8
	plain := stripANSI(m.View())
	for _, item := range []string{"F1 all", "F2 hosts", "F3 commands", "F4 forwards", "F5 jumps"} {
		if !strings.Contains(plain, item) {
			t.Fatalf("category legend is missing %q: %q", item, plain)
		}
	}
	if got := len(strings.Split(plain, "\n")); got != 4 {
		t.Fatalf("empty compact picker has %d lines, want 4 without filler", got)
	}
}

func TestRunningSyncUsesBubblesSpinner(t *testing.T) {
	m := NewPicker(nil, "", DefaultTheme(), Options{InitialSync: SyncStatus{State: SyncStateRunning}})
	if got := stripANSI(m.renderSyncStatus()); !strings.Contains(got, "⠋") || !strings.Contains(got, "sync") {
		t.Fatalf("running sync status does not use spinner bubble: %q", got)
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
