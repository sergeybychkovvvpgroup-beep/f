package ui

import "testing"

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
