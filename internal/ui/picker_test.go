package ui

import (
	"strings"
	"testing"

	"aoo/internal/notes"
	tea "github.com/charmbracelet/bubbletea"
)

func TestSplitPaneUsesAvailableCompactWidth(t *testing.T) {
	m := PickerModel{options: Options{ShowListOnStart: true}}
	if !m.useRightPreview(minSplitPaneWidth) {
		t.Fatalf("split pane must be enabled at width %d", minSplitPaneWidth)
	}
	if m.useRightPreview(minSplitPaneWidth - 1) {
		t.Fatalf("split pane must stay disabled below width %d", minSplitPaneWidth)
	}
}

func TestSplitPaneWaitsUntilResultsAreVisible(t *testing.T) {
	m := PickerModel{}
	if m.useRightPreview(minSplitPaneWidth) {
		t.Fatal("split pane must stay hidden when the result list is hidden")
	}
}

func TestLightModeNeverUsesSplitPane(t *testing.T) {
	m := PickerModel{options: Options{ShowListOnStart: true, LightMode: true}}
	if m.useRightPreview(200) {
		t.Fatal("light mode must remain a single fzf-like list at wide terminal widths")
	}
}

func TestLightModeAlwaysUsesSingleLineResults(t *testing.T) {
	m := PickerModel{options: Options{LightMode: true}}
	if got := m.resultRowHeight(); got != 1 {
		t.Fatalf("light mode row height = %d, want 1", got)
	}
}

func TestLightModeShowsListBeforeTyping(t *testing.T) {
	m := PickerModel{options: Options{LightMode: true, ShowListOnStart: false}}
	if !m.shouldRenderResults() {
		t.Fatal("light mode must show the complete list before typing, like fzf")
	}
}

func TestLightModeViewOmitsFullUIChrome(t *testing.T) {
	m := NewPicker(nil, "prod", DefaultTheme(), Options{LightMode: true, ShowListOnStart: true, Height: 8})
	m.width = 100
	m.height = 8
	view := m.View()
	if strings.Contains(view, "enter ssh") || strings.Contains(view, "┌") {
		t.Fatalf("light mode rendered full UI chrome: %q", view)
	}
	if !strings.Contains(view, "> prod") || !strings.Contains(view, "all") {
		t.Fatalf("light mode is missing query or compact status: %q", view)
	}
}

func TestPickerSearchesAllEntryKindsTogether(t *testing.T) {
	entries := []notes.Entry{
		{Desc: "login", Cmd: "ssh login", Mode: "general"},
		{Desc: "jump", Cmd: "ssh jump", Mode: "jumps"},
		{Desc: "forward", Cmd: "ssh forward", Mode: "forwards"},
		{Desc: "command", Cmd: "ssh command", Mode: "commands"},
	}
	m := NewPicker(entries, "", DefaultTheme(), Options{LightMode: true})
	if got := len(m.matches); got != len(entries) {
		t.Fatalf("picker contains %d/%d entries; all entry kinds must share one search", got, len(entries))
	}
	if strings.Contains(m.statusLine(), "F1") {
		t.Fatalf("unified status still exposes mode tabs: %q", m.statusLine())
	}
}

func TestFunctionKeysDoNotFilterUnifiedSearch(t *testing.T) {
	entries := []notes.Entry{
		{Desc: "login", Cmd: "ssh login", Mode: "general"},
		{Desc: "forward", Cmd: "ssh forward", Mode: "forwards"},
	}
	m := NewPicker(entries, "", DefaultTheme(), Options{LightMode: true})
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyF2})
	got := updated.(PickerModel)
	if len(got.matches) != len(entries) {
		t.Fatalf("F2 filtered unified search to %d/%d entries", len(got.matches), len(entries))
	}
}
