package ui

import (
	"strings"
	"testing"

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

func TestPickerViewOmitsFramesTabsPreviewAndHelp(t *testing.T) {
	m := NewPicker(nil, "prod", DefaultTheme(), Options{Height: 8})
	m.width = 100
	m.height = 8
	view := m.View()
	for _, forbidden := range []string{"enter ssh", "┌", "┐", "full command"} {
		if strings.Contains(view, forbidden) {
			t.Fatalf("fzf view contains obsolete chrome %q: %q", forbidden, view)
		}
	}
	if !strings.Contains(view, "> prod") || !strings.Contains(view, "all") {
		t.Fatalf("fzf view is missing query or compact status: %q", view)
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
	m := NewPicker(entries, "", DefaultTheme(), Options{Layout: "bottom", Height: 6})
	m.width, m.height = 80, 6
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
	m := NewPicker(entries, "", DefaultTheme(), Options{Layout: "bottom", Height: 6})
	m.width, m.height = 80, 6
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
	if got := len(strings.Split(plain, "\n")); got != 8 {
		t.Fatalf("legend changed picker height to %d lines, want 8", got)
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
