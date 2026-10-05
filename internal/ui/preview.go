package ui

import (
	"strings"
	"unicode"

	"f/internal/notes"
	"github.com/charmbracelet/lipgloss"
)

type previewLine struct {
	text string
	hits []notes.Occurrence
}

func previewExcerpt(snippet notes.PreviewSnippet, width, height int) []previewLine {
	text := strings.TrimSpace(snippet.Text)
	if text == "" {
		return []previewLine{{text: ""}}
	}

	lines, lineOffsets := splitPreviewLines(text)
	if snippet.Occurrence.End <= snippet.Occurrence.Start {
		return plainPreviewLines(lines, width, height)
	}

	activeLine := lineForOffset(lineOffsets, snippet.Occurrence.Start)
	startLine := activeLine
	endLine := startLine + 2
	if endLine > len(lines) {
		endLine = len(lines)
	}

	activeLineStart := lineOffsets[activeLine]
	activeLineText := lines[activeLine]
	activeHits := lineHits([]notes.Occurrence{snippet.Occurrence}, activeLineStart, len([]rune(activeLineText)))
	horizontalStart := previewHorizontalStart(activeLineText, activeHits, width)

	result := make([]previewLine, 0, endLine-startLine)
	for lineIndex := startLine; lineIndex < endLine; lineIndex++ {
		lineStart := lineOffsets[lineIndex]
		lineText := lines[lineIndex]
		localHits := lineHits([]notes.Occurrence{snippet.Occurrence}, lineStart, len([]rune(lineText)))
		croppedText, croppedHits := cropPreviewLineWithStart(lineText, localHits, width, horizontalStart)
		result = append(result, previewLine{text: croppedText, hits: croppedHits})
	}

	return result
}

func previewContextExcerpt(snippet notes.PreviewSnippet, width, before, after int) []previewLine {
	text := strings.TrimSpace(snippet.Text)
	if text == "" {
		return []previewLine{{text: ""}}
	}

	lines, lineOffsets := splitPreviewLines(text)
	if snippet.Occurrence.End <= snippet.Occurrence.Start {
		return plainPreviewLines(lines, width, before+after+1)
	}

	activeLine := lineForOffset(lineOffsets, snippet.Occurrence.Start)
	startLine := activeLine - before
	if startLine < 0 {
		startLine = 0
	}
	endLine := activeLine + after + 1
	if endLine > len(lines) {
		endLine = len(lines)
	}

	activeLineStart := lineOffsets[activeLine]
	activeLineText := lines[activeLine]
	activeHits := lineHits([]notes.Occurrence{snippet.Occurrence}, activeLineStart, len([]rune(activeLineText)))
	horizontalStart := previewHorizontalStart(activeLineText, activeHits, width)

	result := make([]previewLine, 0, endLine-startLine)
	for lineIndex := startLine; lineIndex < endLine; lineIndex++ {
		lineStart := lineOffsets[lineIndex]
		lineText := lines[lineIndex]
		localHits := lineHits([]notes.Occurrence{snippet.Occurrence}, lineStart, len([]rune(lineText)))
		croppedText, croppedHits := cropPreviewLineWithStart(lineText, localHits, width, horizontalStart)
		result = append(result, previewLine{text: croppedText, hits: croppedHits})
	}
	return result
}

func splitPreviewLines(text string) ([]string, []int) {
	rawLines := strings.Split(text, "\n")
	lines := make([]string, 0, len(rawLines))
	offsets := make([]int, 0, len(rawLines))
	offset := 0

	for _, line := range rawLines {
		lines = append(lines, strings.TrimRight(line, " "))
		offsets = append(offsets, offset)
		offset += len([]rune(line)) + 1
	}
	return lines, offsets
}

func lineForOffset(offsets []int, target int) int {
	for i := len(offsets) - 1; i >= 0; i-- {
		if target >= offsets[i] {
			return i
		}
	}
	return 0
}

func lineHits(occurrences []notes.Occurrence, lineStart, lineLen int) []notes.Occurrence {
	lineEnd := lineStart + lineLen
	hits := make([]notes.Occurrence, 0, 2)
	for _, occurrence := range occurrences {
		if occurrence.End <= lineStart || occurrence.Start >= lineEnd {
			continue
		}
		start := maxInt(0, occurrence.Start-lineStart)
		end := occurrence.End - lineStart
		if end > lineLen {
			end = lineLen
		}
		hits = append(hits, notes.Occurrence{Start: start, End: end})
	}
	return hits
}

func plainPreviewLines(lines []string, width, height int) []previewLine {
	out := make([]previewLine, 0, minInt(len(lines), maxInt(1, height)))
	for _, line := range excerptLines(lines, maxInt(1, height)) {
		cropped, hits := cropPreviewLine(line, nil, width)
		out = append(out, previewLine{text: cropped, hits: hits})
	}
	return out
}

func cropPreviewLine(line string, hits []notes.Occurrence, width int) (string, []notes.Occurrence) {
	return cropPreviewLineWithStart(line, hits, width, previewHorizontalStart(line, hits, width))
}

func previewHorizontalStart(line string, hits []notes.Occurrence, width int) int {
	runes := []rune(line)
	if width <= 0 || len(runes) <= width {
		return 0
	}

	if width <= 0 {
		return 0
	}
	start := 0
	if len(hits) > 0 {
		before := 2
		if width > 40 {
			before = 4
		}
		start = hits[0].Start - before
	}
	if start < 0 {
		start = 0
	}
	if start > len(runes)-width {
		start = len(runes) - width
	}
	return start
}

func cropPreviewLineWithStart(line string, hits []notes.Occurrence, width, start int) (string, []notes.Occurrence) {
	if width <= 0 {
		return "", nil
	}
	runes := []rune(line)
	if len(runes) <= width {
		return line, hits
	}

	if start < 0 {
		start = 0
	}
	if start > len(runes)-width {
		start = len(runes) - width
	}
	end := start + width

	text := string(runes[start:end])
	if start > 0 && width > 1 {
		text = "…" + string(runes[start+1:end])
	}
	if end < len(runes) && width > 1 {
		text = string([]rune(text)[:width-1]) + "…"
	}

	outHits := make([]notes.Occurrence, 0, len(hits))
	for _, hit := range hits {
		localStart := hit.Start - start
		localEnd := hit.End - start
		if localEnd <= 0 || localStart >= width {
			continue
		}
		if localStart < 0 {
			localStart = 0
		}
		if localEnd > width {
			localEnd = width
		}
		outHits = append(outHits, notes.Occurrence{Start: localStart, End: localEnd})
	}
	return text, outHits
}

func renderPreviewLine(line previewLine, section string, theme Theme) string {
	if strings.TrimSpace(line.text) == "" {
		return ""
	}

	baseStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.DetailFG))
	keywordStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.TitleFG))
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.RowFG))
	commentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.TitleDimFG))
	matchStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.SelectedFG)).
		Background(lipgloss.Color(theme.SelectedBG))

	runes := []rune(line.text)
	styleIDs := make([]int, len(runes))
	for i := range runes {
		styleIDs[i] = 0
	}

	applySyntaxStyles(styleIDs, runes, section)
	for _, hit := range line.hits {
		for i := hit.Start; i < hit.End && i < len(styleIDs); i++ {
			styleIDs[i] = 3
		}
	}

	styles := []lipgloss.Style{baseStyle, keywordStyle, valueStyle, matchStyle, commentStyle}
	var b strings.Builder
	if len(runes) == 0 {
		return ""
	}
	start := 0
	current := styleIDs[0]
	for i := 1; i <= len(runes); i++ {
		if i == len(runes) || styleIDs[i] != current {
			b.WriteString(styles[current].Render(string(runes[start:i])))
			if i < len(runes) {
				start = i
				current = styleIDs[i]
			}
		}
	}
	return b.String()
}

func applySyntaxStyles(styleIDs []int, runes []rune, section string) {
	line := string(runes)
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return
	}
	if strings.HasPrefix(trimmed, "#") {
		for i := range styleIDs {
			styleIDs[i] = 4
		}
		return
	}

	if idx := strings.IndexRune(line, ':'); idx > 0 && idx < len(runes)-1 && isLikelyYAMLKey(line[:idx]) {
		for i := 0; i < idx; i++ {
			styleIDs[i] = 1
		}
		for i := idx + 1; i < len(styleIDs); i++ {
			styleIDs[i] = 2
		}
	}

	if section == "command" || looksLikeShell(trimmed) || looksLikeConfig(trimmed) {
		markWords(styleIDs, runes, 1, map[string]struct{}{
			"set": {}, "delete": {}, "show": {}, "edit": {}, "commit": {}, "save": {}, "run": {},
			"ssh": {}, "sudo": {}, "curl": {}, "docker": {}, "systemctl": {}, "ip": {}, "ping": {},
		})
	}
}

func markWords(styleIDs []int, runes []rune, styleID int, keywords map[string]struct{}) {
	start := -1
	for i, r := range runes {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' {
			if start == -1 {
				start = i
			}
			continue
		}
		if start != -1 {
			word := strings.ToLower(string(runes[start:i]))
			if _, ok := keywords[word]; ok {
				for j := start; j < i; j++ {
					styleIDs[j] = styleID
				}
			}
			start = -1
		}
	}
	if start != -1 {
		word := strings.ToLower(string(runes[start:]))
		if _, ok := keywords[word]; ok {
			for j := start; j < len(runes); j++ {
				styleIDs[j] = styleID
			}
		}
	}
}

func isLikelyYAMLKey(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && !strings.ContainsAny(value, " {}[]")
}

func looksLikeShell(value string) bool {
	return strings.Contains(value, "$") || strings.Contains(value, "{{") || strings.Contains(value, "sudo ")
}

func looksLikeConfig(value string) bool {
	return strings.HasPrefix(value, "set ") || strings.HasPrefix(value, "delete ") || strings.HasPrefix(value, "show ")
}

func previewPaneLines(preview notes.PreviewMatch, width, height, activeIndex int, theme Theme) []string {
	if height <= 0 {
		height = 1
	}
	if len(preview.Snippets) == 0 {
		lines := plainPreviewLines([]string{preview.Text}, width, height)
		out := make([]string, 0, len(lines))
		for _, line := range lines {
			out = append(out, renderPreviewLine(line, preview.Section, theme))
		}
		return out
	}

	if activeIndex < 0 || activeIndex >= len(preview.Snippets) {
		activeIndex = 0
	}
	snippet := preview.Snippets[activeIndex]
	lines := previewExcerpt(snippet, width, height)
	if height == 1 && len(lines) > 1 {
		lines = lines[:1]
	}
	rendered := make([]string, 0, len(lines))
	for _, line := range lines {
		rendered = append(rendered, renderPreviewLine(line, snippet.Section, theme))
	}
	return clipLines(rendered, height)
}

func popupPreviewLines(preview notes.PreviewMatch, width, height, activeIndex int, theme Theme) []string {
	if height <= 0 {
		height = 1
	}
	if len(preview.Snippets) == 0 {
		lines := plainPreviewLines(strings.Split(preview.Text, "\n"), width, height)
		out := make([]string, 0, len(lines))
		for _, line := range lines {
			out = append(out, renderPreviewLine(line, preview.Section, theme))
		}
		return clipLines(out, height)
	}

	if activeIndex < 0 || activeIndex >= len(preview.Snippets) {
		activeIndex = 0
	}
	snippet := preview.Snippets[activeIndex]
	lines := previewContextExcerpt(snippet, width, 1, 1)
	rendered := make([]string, 0, len(lines))
	for _, line := range lines {
		rendered = append(rendered, renderPreviewLine(line, snippet.Section, theme))
	}
	return clipLines(rendered, height)
}
