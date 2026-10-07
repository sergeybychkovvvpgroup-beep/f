package ui

import (
	"strings"
	"testing"
	"unicode/utf8"

	"f/internal/notes"
	"github.com/charmbracelet/bubbles/cursor"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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

func TestPickerShowsOnlySelectedCommandBelowName(t *testing.T) {
	entry := notes.Entry{
		Desc:    "gateway-production-primary",
		Address: "operator@192.0.2.10:2222",
		Command: "ssh -p 2222 operator@192.0.2.10",
		Kind:    "host",
		Actions: []notes.Action{{Cmd: "ssh gateway"}},
	}
	m := NewPicker([]notes.Entry{entry}, "", DefaultTheme(), Options{Height: 8, ShowAddress: true})
	m.width, m.height = 100, 8
	plain := stripANSI(m.View())
	if !strings.Contains(plain, "│ gateway-production-primary") {
		t.Fatalf("selected row lost its full name: %q", plain)
	}
	if strings.Contains(plain, "· operator@192.0.2.10:2222") {
		t.Fatalf("selected row still shows the obsolete short target preview: %q", plain)
	}
	if !strings.Contains(plain, "ssh -p 2222 operator@192.0.2.10") {
		t.Fatalf("selected row does not show its full command below: %q", plain)
	}
}

func TestUnselectedRowsShowOnlyNames(t *testing.T) {
	entries := []notes.Entry{
		{Desc: "selected", Address: "root@192.0.2.1", Command: "ssh -o ProxyJump=jump root@192.0.2.1", Actions: []notes.Action{{Cmd: "ssh selected"}}},
		{Desc: "unselected-long-name-that-must-stay-complete", Address: "admin@192.0.2.2", Command: "ssh -o ProxyJump=jump admin@192.0.2.2", Actions: []notes.Action{{Cmd: "ssh unselected"}}},
	}
	m := NewPicker(entries, "", DefaultTheme(), Options{Height: 10, ShowAddress: true})
	m.width, m.height = 100, 10
	plain := stripANSI(m.View())
	if !strings.Contains(plain, "unselected-long-name-that-must-stay-complete") {
		t.Fatalf("unselected row lost its full name: %q", plain)
	}
	if strings.Contains(plain, "admin@192.0.2.2") {
		t.Fatalf("unselected row shows an address or command preview: %q", plain)
	}
	if strings.Count(plain, "ssh -o ProxyJump=jump") != 1 {
		t.Fatalf("full command should appear only for the selected row: %q", plain)
	}
}

func TestResultLineNeverTruncatesEntryName(t *testing.T) {
	name := "omada-chashnikovo-production-controller"
	target := "operator@192.0.2.10"
	got := compactResultLine(name, target, 30)
	if !strings.Contains(got, name) || strings.Contains(strings.Split(got, " · ")[0], "…") {
		t.Fatalf("entry name was truncated: %q", got)
	}
}

func TestNarrowLongNameAndTargetUseExplicitViewportSafeLines(t *testing.T) {
	name := strings.Repeat("long-name-", 8)
	target := "operator@192.0.2.10"
	entry := notes.Entry{Desc: name, Address: target, Command: "ssh " + target, Actions: []notes.Action{{Cmd: "ssh long"}}}
	m := NewPicker([]notes.Entry{entry}, "", DefaultTheme(), Options{Height: 10, ShowAddress: true})
	m.width, m.height = 42, 10
	plain := stripANSI(m.View())
	compact := strings.NewReplacer(" ", "", "	", "", "\n", "", "\r", "").Replace(plain)
	if !strings.Contains(compact, strings.ReplaceAll(name, " ", "")) || !strings.Contains(compact, strings.ReplaceAll(target, " ", "")) {
		t.Fatalf("wrapped row lost name or target: %q", plain)
	}
	for _, line := range strings.Split(plain, "\n") {
		if width := ansi.StringWidth(strings.TrimRight(line, " ")); width > 42 {
			t.Fatalf("rendered line width = %d, want <= 42: %q", width, line)
		}
	}
}

func TestUnselectedCommandOnlyEntryDoesNotRepeatCommand(t *testing.T) {
	entries := []notes.Entry{
		{Desc: "one", Command: "ssh one", Actions: []notes.Action{{Cmd: "ssh one"}}},
		{Desc: "two", Command: "ssh two", Actions: []notes.Action{{Cmd: "ssh two"}}},
	}
	m := NewPicker(entries, "", DefaultTheme(), Options{Height: 10, ShowAddress: true})
	m.width, m.height = 80, 10
	plain := stripANSI(m.View())
	if strings.Contains(plain, "two · ssh two") {
		t.Fatalf("unselected command-only entry repeated its command: %q", plain)
	}
}

func TestSelectedCommandTailIsNotClippedByCompactHeight(t *testing.T) {
	command := "ssh " + strings.Repeat("-o ServerAliveInterval=30 ", 12) + "operator@192.0.2.10 final-token"
	entry := notes.Entry{Desc: "gateway", Address: "operator@192.0.2.10", Command: command, Actions: []notes.Action{{Cmd: "ssh gateway"}}}
	m := NewPicker([]notes.Entry{entry}, "", DefaultTheme(), Options{Height: 8, ShowAddress: true})
	m.width, m.height = 52, 8
	plain := stripANSI(m.View())
	if !strings.Contains(plain, "final-token") || !strings.Contains(plain, "…") {
		t.Fatalf("oversized selected command did not preserve its tail with an explicit omission marker: %q", plain)
	}
	if lines := strings.Count(plain, "\n") + 1; lines > 8 {
		t.Fatalf("picker rendered %d lines into an 8-line viewport: %q", lines, plain)
	}
}

func TestSelectedCommandWrapsSingleLongTokenByDisplayWidth(t *testing.T) {
	command := strings.Repeat("界", 30)
	for _, line := range selectedCommandLines(command, 24) {
		if width := ansi.StringWidth(line); width > 22 {
			t.Fatalf("selected command line width = %d, want <= 22: %q", width, line)
		}
	}
}

func TestLongAddresslessNameWrapsWhenHeightAllows(t *testing.T) {
	name := strings.Repeat("ordinary-name-", 6)
	entry := notes.Entry{Desc: name, Command: "ssh alias", Actions: []notes.Action{{Cmd: "ssh alias"}}}
	m := NewPicker([]notes.Entry{entry}, "", DefaultTheme(), Options{Height: 20, ShowAddress: true})
	m.width, m.height = 42, 20
	plain := stripANSI(m.View())
	compact := strings.NewReplacer(" ", "", "\t", "", "\n", "", "\r", "").Replace(plain)
	if !strings.Contains(compact, name) {
		t.Fatalf("long addressless name was truncated instead of wrapped: %q", plain)
	}
}

func TestMinimumViewportPreservesCompleteWrappedTarget(t *testing.T) {
	target := "user@final-target.example"
	entry := notes.Entry{Desc: strings.Repeat("selected-name-", 8), Address: target, Command: "ssh " + target, Actions: []notes.Action{{Cmd: "ssh alias"}}}
	m := NewPicker([]notes.Entry{entry}, "", DefaultTheme(), Options{Height: 6, ShowAddress: true})
	m.width, m.height = 20, 6
	plain := stripANSI(m.View())
	compact := strings.NewReplacer(" ", "", "\t", "", "\n", "", "\r", "").Replace(plain)
	if !strings.Contains(compact, "selected-nam") || !strings.Contains(compact, target) || !strings.Contains(plain, "…") {
		t.Fatalf("minimum viewport lost the selected name, omission marker, or part of the wrapped target: %q", plain)
	}
}

func TestRowsStayContiguousAroundCursor(t *testing.T) {
	entries := []notes.Entry{
		{Desc: "zero", Address: "u@0", Command: "ssh u@0", Actions: []notes.Action{{Cmd: "ssh zero"}}},
		{Desc: strings.Repeat("huge-", 25), Address: "u@1", Command: "ssh u@1", Actions: []notes.Action{{Cmd: "ssh one"}}},
		{Desc: "two", Address: "u@2", Command: "ssh u@2", Actions: []notes.Action{{Cmd: "ssh two"}}},
		{Desc: "three", Address: "u@3", Command: "ssh u@3", Actions: []notes.Action{{Cmd: "ssh three"}}},
		{Desc: "four", Address: "u@4", Command: "ssh u@4", Actions: []notes.Action{{Cmd: "ssh four"}}},
	}
	m := NewPicker(entries, "", DefaultTheme(), Options{Height: 10, ShowAddress: true})
	m.width, m.height, m.cursor = 42, 10, 2
	plain := stripANSI(m.View())
	if strings.Contains(plain, "zero") && !strings.Contains(plain, "huge-") {
		t.Fatalf("viewport skipped an intervening row and showed a farther row: %q", plain)
	}
}

func TestOversizedSelectedRowUsesExplicitBoundedOmission(t *testing.T) {
	name := strings.Repeat("very-long-name-", 30)
	target := "operator@192.0.2.10"
	entry := notes.Entry{Desc: name, Address: target, Command: "ssh " + target, Actions: []notes.Action{{Cmd: "ssh alias"}}}
	m := NewPicker([]notes.Entry{entry}, "", DefaultTheme(), Options{Height: 8, ShowAddress: true})
	m.width, m.height = 42, 8
	plain := stripANSI(m.View())
	if !strings.Contains(plain, "…") || !strings.Contains(plain, target) {
		t.Fatalf("oversized row lacks an explicit omission marker or target: %q", plain)
	}
	if lines := strings.Count(plain, "\n") + 1; lines > 8 {
		t.Fatalf("oversized row escaped viewport: %q", plain)
	}
}

func TestCompactStatusAdvertisesEditHotkey(t *testing.T) {
	m := NewPicker([]notes.Entry{{Desc: "gateway", Editable: true, Actions: []notes.Action{{Cmd: "ssh gateway"}}}}, "", DefaultTheme(), Options{Height: 8})
	m.width, m.height = 100, 8
	plain := stripANSI(m.View())
	if !strings.Contains(plain, "Ctrl+E edit") {
		t.Fatalf("compact picker does not advertise Ctrl+E: %q", plain)
	}
}

func TestNarrowStatusKeepsEditHotkeyVisible(t *testing.T) {
	m := NewPicker([]notes.Entry{{Desc: "gateway", Editable: true, Actions: []notes.Action{{Cmd: "ssh gateway"}}}}, "", DefaultTheme(), Options{Height: 8})
	m.width, m.height = 40, 8
	plain := stripANSI(m.View())
	if !strings.Contains(plain, "Ctrl+E edit") {
		t.Fatalf("narrow picker hid Ctrl+E: %q", plain)
	}
}

func TestEditHotkeyDoesNotExitForNonEditableEntry(t *testing.T) {
	m := NewPicker([]notes.Entry{{Desc: "command", Actions: []notes.Action{{Cmd: "docker ps"}}}}, "", DefaultTheme(), Options{Height: 8})
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlE})
	got := updated.(PickerModel)
	if cmd != nil || got.EditRequested() || got.Selected() != nil {
		t.Fatal("Ctrl+E must be ignored for entries without an SSH config source")
	}
}

func TestWideResultUsesAvailableSpaceForFullCommand(t *testing.T) {
	command := "ssh -p 2222 -J jump.example -i ~/.ssh/id_ed25519 operator@192.0.2.10 show route table main"
	got := compactResultLine("gateway", command, 140)
	if !strings.Contains(got, command) || strings.Contains(got, "…") {
		t.Fatalf("wide result truncated the full command: %q", got)
	}
}

func TestNarrowResultKeepsCommandStartAndTarget(t *testing.T) {
	command := "ssh -o HostKeyAlgorithms=+ssh-rsa -o PubkeyAcceptedAlgorithms=+ssh-rsa root@192.0.2.10"
	got := compactResultLine("legacy-router", command, 64)
	if !strings.Contains(got, "ssh -o") || !strings.Contains(got, "root@192.0.2.10") || !strings.Contains(got, "…") {
		t.Fatalf("middle truncation did not preserve command start and target: %q", got)
	}
	if utf8.RuneCountInString(got) > 64 {
		t.Fatalf("result width = %d, want <= 64: %q", utf8.RuneCountInString(got), got)
	}
}

func TestMiddleTruncationRespectsTerminalWidthForWideRunes(t *testing.T) {
	command := "ssh " + strings.Repeat("界", 30) + " operator@192.0.2.10"
	got := compactResultLine("東京-router", command, 64)
	if width := ansi.StringWidth(got); width > 64 {
		t.Fatalf("terminal width = %d, want <= 64: %q", width, got)
	}
	if !strings.Contains(got, "operator@192.0.2.10") {
		t.Fatalf("middle truncation lost target: %q", got)
	}
}

func TestRenderedAddressRowRespectsTerminalWidthForWideRunes(t *testing.T) {
	theme := DefaultTheme()
	entry := notes.Entry{Desc: "東京-router", Command: "ssh " + strings.Repeat("界", 30) + " operator@192.0.2.10"}
	match := notes.Match{Entry: entry, Label: entry.Desc, Detail: entry.Command}
	m := PickerModel{options: Options{ShowAddress: true}, theme: theme}
	style := lipgloss.NewStyle()
	got := m.renderMatchLabelLine(match, entry, 64, false, style, style, style)
	if width := ansi.StringWidth(got); width > 64 {
		t.Fatalf("rendered terminal width = %d, want <= 64: %q", width, got)
	}
}

func TestTruncateLeftWidthPreservesGraphemeClusters(t *testing.T) {
	if got := truncateLeftWidth("prefix👩‍💻", 2); got != "👩‍💻" {
		t.Fatalf("ZWJ emoji was split: %q", got)
	}
	if got := truncateLeftWidth("prefix🇺🇸", 1); got != "" {
		t.Fatalf("flag grapheme was split: %q", got)
	}
}

func TestSelectedCommandWrapsAtArgumentBoundaries(t *testing.T) {
	command := "ssh -o ProxyJump=access.example -o 'IdentityFile=~/.ssh/id_ed25519' operator@192.0.2.10"
	lines := selectedCommandLines(command, 52)
	if len(lines) < 2 {
		t.Fatalf("command was not wrapped: %q", lines)
	}
	for _, line := range lines {
		if strings.Count(line, "'")%2 != 0 {
			t.Fatalf("command line split a quoted argument: %q", lines)
		}
	}
	if got := strings.Join(lines, " "); got != command {
		t.Fatalf("wrapped command changed text: %q", got)
	}
}

func TestCompactHeightIncludesSelectedCommandAndOtherRows(t *testing.T) {
	entries := []notes.Entry{
		{Desc: "one", Address: "root@192.0.2.1", Command: "ssh -o ProxyJump=access.example -o 'IdentityFile=~/.ssh/id_ed25519' root@192.0.2.1", Actions: []notes.Action{{Cmd: "ssh one"}}},
		{Desc: "two", Address: "root@192.0.2.2", Command: "ssh root@192.0.2.2", Actions: []notes.Action{{Cmd: "ssh two"}}},
		{Desc: "three", Address: "root@192.0.2.3", Command: "ssh root@192.0.2.3", Actions: []notes.Action{{Cmd: "ssh three"}}},
	}
	m := NewPicker(entries, "", DefaultTheme(), Options{Height: 12, ShowAddress: true})
	m.width, m.height = 80, 12
	plain := stripANSI(m.View())
	if !strings.Contains(plain, "two") || !strings.Contains(plain, "three") {
		t.Fatalf("selected command preview hid ordinary rows: %q", plain)
	}
	if strings.Contains(plain, "root@192.0.2.2") || strings.Contains(plain, "root@192.0.2.3") {
		t.Fatalf("ordinary rows still show obsolete short target previews: %q", plain)
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

func TestSelectedCommandModeFitsMinimumPickerHeight(t *testing.T) {
	entries := []notes.Entry{
		{Desc: "one", Address: "operator@192.0.2.1", Kind: "host", Actions: []notes.Action{{Cmd: "ssh one"}}},
		{Desc: "two", Address: "operator@192.0.2.2", Kind: "host", Actions: []notes.Action{{Cmd: "ssh two"}}},
	}
	m := NewPicker(entries, "", DefaultTheme(), Options{Height: 6, ShowAddress: true})
	m.width, m.height = 100, 6
	plain := stripANSI(m.View())
	if !strings.Contains(plain, "one") || !strings.Contains(plain, "two") || !strings.Contains(plain, "F1–F5 categories") || strings.Contains(plain, "operator@") || strings.Contains(plain, "…") {
		t.Fatalf("minimum-height selected-command mode is clipped or shows an obsolete target preview: %q", plain)
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

func TestTabShowsSSHConfigBlockWithoutServiceMetadata(t *testing.T) {
	block := "Host spb-omada\n  HostName 195.218.230.91\n  User sergeyb\n  LocalForward 8443 192.168.121.5:443"
	entry := notes.Entry{
		Desc:           "spb-omada [remote 192.168.121.5:443]",
		Address:        "sergeyb@195.218.230.91",
		Kind:           "fwd",
		Mode:           "forwards",
		SSHConfigBlock: block,
		Actions:        []notes.Action{{Desc: "sergeyb@195.218.230.91", Cmd: "ssh spb-omada"}},
	}
	m := NewPicker([]notes.Entry{entry}, "", DefaultTheme(), Options{Height: 12, ShowAddress: true})
	m.width, m.height = 100, 12
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	plain := stripANSI(updated.(PickerModel).View())
	for _, want := range []string{
		"Host spb-omada", "HostName 195.218.230.91", "User sergeyb", "LocalForward 8443 192.168.121.5:443",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("details do not show SSH config directive %q: %q", want, plain)
		}
	}
	for _, unwanted := range []string{"DETAILS", "Name  ", "Address  ", "Type  ", "Mode  ", "About  ", "Command\n"} {
		if strings.Contains(plain, unwanted) {
			t.Fatalf("details contain service metadata %q: %q", unwanted, plain)
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

func TestDetailCommandWrappingUsesTerminalDisplayWidth(t *testing.T) {
	block := "Host alias\n  HostName " + strings.Repeat("界", 20)
	entry := notes.Entry{Desc: "alias", SSHConfigBlock: block, Actions: []notes.Action{{Cmd: "ssh alias"}}}
	m := NewPicker([]notes.Entry{entry}, "", DefaultTheme(), Options{Height: 8})
	m.width, m.height = 30, 8
	opened, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	plain := stripANSI(opened.(PickerModel).View())
	for _, line := range strings.Split(plain, "\n") {
		if width := ansi.StringWidth(strings.TrimRight(line, " ")); width > 30 {
			t.Fatalf("detail line width = %d, want <= 30: %q", width, line)
		}
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
