package notes

import "testing"

func TestAnalyzeDuplicatesReportsKindsAndLocationsDeterministically(t *testing.T) {
	entries := []Entry{
		{Desc: " Router   Login ", Actions: []Action{{Cmd: "ssh router"}}, SourcePath: "/notes/b.yaml", SourceLine: 8},
		{Desc: "router login", Actions: []Action{{Cmd: " ssh router "}}, SourcePath: "/notes/a.yaml", SourceLine: 2},
		{Desc: "Other", Actions: []Action{{Text: "same text"}}, SourcePath: "/notes/c.yaml", SourceLine: 3},
		{Desc: "Different", Actions: []Action{{Text: " same   text "}}, SourcePath: "/notes/d.yaml", SourceLine: 4},
	}

	got := AnalyzeDuplicates(entries)
	if len(got) != 4 {
		t.Fatalf("expected exact, command, text and description duplicates, got %#v", got)
	}
	wantKinds := []DuplicateKind{DuplicateCommand, DuplicateDescription, DuplicateExact, DuplicateText}
	for i, want := range wantKinds {
		if got[i].Kind != want {
			t.Fatalf("duplicate %d kind = %q, want %q", i, got[i].Kind, want)
		}
		if len(got[i].Locations) != 2 || got[i].Locations[0].Path > got[i].Locations[1].Path {
			t.Fatalf("locations are not preserved and sorted: %#v", got[i].Locations)
		}
	}
}

func TestAnalyzeDuplicatesIgnoresBlankValues(t *testing.T) {
	got := AnalyzeDuplicates([]Entry{{SourcePath: "a", SourceLine: 1}, {SourcePath: "b", SourceLine: 2}})
	if len(got) != 0 {
		t.Fatalf("expected no blank duplicates, got %#v", got)
	}
}

func TestAnalyzeDuplicatesExactIncludesAllActionMetadata(t *testing.T) {
	entries := []Entry{
		{Desc: "router", Actions: []Action{{Desc: "primary", Cmd: "ssh router", Banner: "production"}}, SourcePath: "a", SourceLine: 1},
		{Desc: "router", Actions: []Action{{Desc: "backup", Cmd: "ssh router", Banner: "lab"}}, SourcePath: "b", SourceLine: 1},
	}

	for _, duplicate := range AnalyzeDuplicates(entries) {
		if duplicate.Kind == DuplicateExact {
			t.Fatalf("entries with different action metadata and notes are not exact duplicates: %#v", duplicate)
		}
	}
}

func TestAnalyzeDuplicatesPreservesCommandWhitespaceSemantics(t *testing.T) {
	entries := []Entry{
		{Desc: "one", Cmd: "printf '%s  %s' a b", SourcePath: "a", SourceLine: 1},
		{Desc: "two", Cmd: "printf '%s %s' a b", SourcePath: "b", SourceLine: 1},
	}
	for _, duplicate := range AnalyzeDuplicates(entries) {
		if duplicate.Kind == DuplicateCommand {
			t.Fatalf("semantically distinct commands were collapsed: %#v", duplicate)
		}
	}
}
