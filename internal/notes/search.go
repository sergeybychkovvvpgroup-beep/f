package notes

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

const (
	SearchModeCommandOnly = "command-only"
	SearchModeNoteOnly    = "note-only"
)

type Match struct {
	Entry  Entry
	Score  int
	Label  string
	Detail string
}

func Filter(entries []Entry, query string) []Match {
	resolvedMode, normalizedQuery := resolveSearchMode(query)
	candidates := entries
	if resolvedMode == SearchModeCommandOnly {
		candidates = commandEntries(entries)
	}
	if resolvedMode == SearchModeNoteOnly {
		candidates = noteEntries(entries)
	}

	terms := queryTerms(normalizedQuery)
	if len(terms) == 0 {
		matches := make([]Match, 0, len(candidates))
		for _, entry := range candidates {
			matches = append(matches, Match{
				Entry:  entry,
				Score:  1,
				Detail: formatDetail(entry),
			})
		}
		return applyMatchLabels(matches, resolvedMode)
	}
	matches := make([]Match, 0, len(candidates))

	for _, entry := range candidates {
		score, ok := scoreEntry(entry, terms)
		if !ok {
			continue
		}

		matches = append(matches, Match{
			Entry:  entry,
			Score:  score,
			Detail: formatDetail(entry),
		})
	}

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Score == matches[j].Score {
			if matches[i].Entry.index == matches[j].Entry.index {
				return matches[i].Entry.DisplayName() < matches[j].Entry.DisplayName()
			}
			return matches[i].Entry.index < matches[j].Entry.index
		}
		return matches[i].Score > matches[j].Score
	})

	return applyMatchLabels(matches, resolvedMode)
}

func resolveSearchMode(query string) (string, string) {
	trimmedQuery := strings.TrimSpace(query)
	if trimmedQuery == "" {
		return SearchModeCommandOnly, ""
	}

	switch trimmedQuery[0] {
	case ':', '>':
		return SearchModeCommandOnly, strings.TrimSpace(trimmedQuery[1:])
	default:
		return SearchModeCommandOnly, trimmedQuery
	}
}

func queryTerms(query string) []string {
	rawTerms := strings.Fields(strings.TrimSpace(query))
	terms := make([]string, 0, len(rawTerms))
	for _, raw := range rawTerms {
		normalized := normalize(raw)
		if normalized == "" {
			continue
		}
		terms = append(terms, normalized)
	}
	return terms
}

func applyMatchLabels(matches []Match, mode string) []Match {
	if len(matches) == 0 {
		return matches
	}

	if mode == SearchModeCommandOnly {
		for i := range matches {
			label, detail := commandOnlyPresentation(matches[i].Entry)
			matches[i].Label = label
			matches[i].Detail = detail
		}
		return matches
	}

	counts := make(map[string]int, len(matches))
	for _, match := range matches {
		counts[normalize(match.Entry.DisplayName())]++
	}

	for i := range matches {
		matches[i].Label = matches[i].Entry.DisplayName()
		if counts[normalize(matches[i].Entry.DisplayName())] > 1 {
			matches[i].Label = matches[i].Entry.Title()
		}
	}

	return matches
}

func commandOnlyPresentation(entry Entry) (string, string) {
	action := entry.PrimaryAction()
	if action == nil {
		return entry.DisplayName(), formatDetail(entry)
	}

	detail := strings.TrimSpace(action.Desc)
	if detail == "" {
		detail = entry.DisplayName()
	}

	switch {
	case action.IsCmd():
		return entry.DisplayName(), oneLine(action.Cmd, 120)
	default:
		return entry.DisplayName(), action.DisplayValue()
	}
}

func scoreEntry(entry Entry, terms []string) (int, bool) {
	fields := entry.searchData
	if len(fields) == 0 {
		fields = weightedFields(entry)
	}
	total := 0

	for _, term := range terms {
		best := 0
		for _, field := range fields {
			score := matchScore(field, term)
			if score > best {
				best = score
			}
		}

		if best == 0 {
			return 0, false
		}
		total += best
	}

	if entry.HasCmd() {
		total += 4
	}
	if entry.IsGroup() {
		total += 6
	}

	return total, true
}

type weightedField struct {
	value  string
	weight int
}

func weightedFields(entry Entry) []weightedField {
	fields := []weightedField{
		{value: normalize(entry.DisplayName()), weight: 8},
		{value: normalize(strings.TrimSuffix(entry.SourceFile, filepathExt(entry.SourceFile))), weight: 7},
		{value: normalize(entry.Note), weight: 5},
	}
	if entry.IsGroup() {
		fields = append(fields, weightedField{value: normalize(entry.GroupSummary), weight: 6})
	}
	for _, action := range entry.ActionsList() {
		fields = append(fields,
			weightedField{value: normalize(action.Desc), weight: 6},
			weightedField{value: normalize(action.Cmd), weight: 5},
			weightedField{value: normalize(action.Text), weight: 3},
			weightedField{value: normalize(action.Banner), weight: 2},
		)
	}

	out := make([]weightedField, 0, len(fields))
	for _, field := range fields {
		if field.value != "" {
			out = append(out, field)
		}
	}
	return out
}

func groupEntries(entries []Entry) []Entry {
	type grouped struct {
		key     string
		entries []Entry
	}

	groups := make([]grouped, 0, len(entries))
	indexByKey := make(map[string]int, len(entries))
	for _, entry := range entries {
		key := groupKey(entry)
		if groupIndex, ok := indexByKey[key]; ok {
			groups[groupIndex].entries = append(groups[groupIndex].entries, entry)
			continue
		}
		indexByKey[key] = len(groups)
		groups = append(groups, grouped{
			key:     key,
			entries: []Entry{entry},
		})
	}

	out := make([]Entry, 0, len(groups))
	for _, group := range groups {
		out = append(out, buildEntryGroup(group.entries))
	}
	return out
}

func commandEntries(entries []Entry) []Entry {
	out := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		for _, action := range entry.ActionsList() {
			if !action.IsCmd() {
				continue
			}
			actionCopy := action
			desc := strings.TrimSpace(action.Desc)
			if desc == "" {
				desc = entry.DisplayName()
			}
			out = append(out, Entry{
				Desc:       entry.DisplayName(),
				Actions:    []Action{actionCopy},
				SourcePath: entry.SourcePath,
				SourceFile: entry.SourceFile,
				SourceLine: entry.SourceLine,
				index:      entry.index,
				searchData: []weightedField{
					{value: normalize(entry.DisplayName()), weight: 6},
					{value: normalize(desc), weight: 8},
					{value: normalize(actionCopy.Cmd), weight: 9},
				},
			})
		}
	}
	return out
}

func noteEntries(entries []Entry) []Entry {
	out := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		hasShow := false
		for _, action := range entry.ActionsList() {
			if !action.IsShow() {
				continue
			}
			hasShow = true
			actionCopy := action
			desc := strings.TrimSpace(action.Desc)
			if desc == "" || desc == "show" || desc == entry.DisplayName() {
				desc = entry.DisplayName()
			} else {
				desc = strings.TrimSpace(entry.DisplayName() + " " + desc)
			}
			out = append(out, Entry{
				Desc:       desc,
				Actions:    []Action{actionCopy},
				SourcePath: entry.SourcePath,
				SourceFile: entry.SourceFile,
				SourceLine: entry.SourceLine,
				SourceKind: entry.SourceKind,
				index:      entry.index,
				searchData: []weightedField{
					{value: normalize(desc), weight: 8},
					{value: normalize(entry.DisplayName()), weight: 7},
					{value: normalize(entry.SourceFile), weight: 6},
					{value: normalize(actionCopy.Desc), weight: 5},
					{value: normalize(actionCopy.Text), weight: 6},
				},
			})
		}
		if hasShow {
			continue
		}
		if entry.HasCmd() {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func groupKey(entry Entry) string {
	if strings.TrimSpace(entry.SourcePath) != "" {
		return entry.SourcePath
	}
	if strings.TrimSpace(entry.SourceFile) != "" {
		return entry.SourceFile
	}
	return fmt.Sprintf("%s#%d", entry.DisplayName(), entry.index)
}

func buildEntryGroup(entries []Entry) Entry {
	first := entries[0]
	group := Entry{
		Actions:      flattenGroupActions(entries),
		SourcePath:   first.SourcePath,
		SourceFile:   first.SourceFile,
		SourceLine:   first.SourceLine,
		GroupEntries: append([]Entry(nil), entries...),
		GroupSummary: buildGroupSummary(entries),
		index:        first.index,
	}
	group.searchData = weightedFields(group)
	return group
}

func buildGroupSummary(entries []Entry) string {
	if len(entries) == 0 {
		return ""
	}
	parts := make([]string, 0, minInt(2, len(entries)))
	for i, entry := range entries {
		if i >= 2 {
			break
		}
		parts = append(parts, entry.DisplayValue())
	}
	summary := strings.Join(parts, " | ")
	if len(entries) > 2 {
		summary = fmt.Sprintf("%s (+%d more)", summary, len(entries)-2)
	}
	return summary
}

func flattenGroupActions(entries []Entry) []Action {
	actions := make([]Action, 0, len(entries)*2)
	for _, entry := range entries {
		childActions := entry.ActionsList()
		for i, action := range childActions {
			actionCopy := action
			actionCopy.Desc = groupActionLabel(entry, action, len(childActions), i)
			actions = append(actions, actionCopy)
		}
	}
	return actions
}

func groupActionLabel(entry Entry, action Action, total, index int) string {
	base := entry.DisplayName()
	if total <= 1 {
		return base
	}

	suffix := strings.TrimSpace(action.Desc)
	if suffix == "" {
		switch {
		case action.IsCmd():
			suffix = fmt.Sprintf("run %d", index+1)
		case action.IsShow():
			suffix = "show"
		default:
			suffix = fmt.Sprintf("action %d", index+1)
		}
	}
	return base + " :: " + suffix
}

func matchScore(field weightedField, term string) int {
	if isDigits(term) {
		return matchNumericTerm(field, term)
	}

	if strings.Contains(field.value, term) {
		return 120 + field.weight*10
	}

	normalizedTerm := normalizeNumericToken(term)
	normalizedField := normalizeNumericToken(field.value)
	if strings.Contains(normalizedField, normalizedTerm) {
		return 119 + field.weight*10
	}

	bestWord := 0
	for _, word := range strings.Fields(field.value) {
		if normalizeNumericToken(word) == normalizedTerm {
			if len(normalizedTerm) >= 2 {
				return 118 + field.weight*10
			}
			return 112 + field.weight*10
		}
		if score := subsequenceScore(word, term); score > bestWord {
			bestWord = score
		}
	}
	if bestWord > 0 {
		return bestWord + field.weight*10
	}

	compactField := strings.ReplaceAll(field.value, " ", "")
	if len(term) <= 3 {
		return 0
	}
	if score := subsequenceScore(compactField, term); score > 0 {
		return score + field.weight*6
	}

	return 0
}

func matchNumericTerm(field weightedField, term string) int {
	normalizedTerm := normalizeNumericToken(term)
	best := 0

	for _, word := range strings.Fields(field.value) {
		if normalizeNumericToken(word) != normalizedTerm {
			continue
		}

		score := 118 + field.weight*10
		if isDigits(word) {
			score = 122 + field.weight*10
		}
		if score > best {
			best = score
		}
	}

	return best
}

func subsequenceScore(haystack, needle string) int {
	if needle == "" {
		return 1
	}

	hr := []rune(haystack)
	nr := []rune(needle)

	ni := 0
	first := -1
	last := -1

	for i, r := range hr {
		if ni < len(nr) && r == nr[ni] {
			if first == -1 {
				first = i
			}
			last = i
			ni++
			if ni == len(nr) {
				span := last - first + 1
				gaps := span - len(nr)
				if !isCloseSubsequence(span, len(nr), first) {
					return 0
				}
				return 100 - gaps*2
			}
		}
	}

	return 0
}

func isCloseSubsequence(span, needleLen, first int) bool {
	if needleLen <= 2 {
		return span == needleLen
	}

	maxExtra := 1
	if needleLen >= 5 {
		maxExtra = 2
	}
	if needleLen >= 8 {
		maxExtra = 3
	}

	if span > needleLen+maxExtra {
		return false
	}

	if first > maxExtra+1 {
		return false
	}

	return true
}

func normalize(value string) string {
	value = strings.ToLower(value)
	var b strings.Builder
	lastSpace := false

	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastSpace = false
			continue
		}

		if !lastSpace {
			b.WriteByte(' ')
			lastSpace = true
		}
	}

	return strings.TrimSpace(b.String())
}

func formatDetail(entry Entry) string {
	return entry.DisplayValue()
}

func normalizeNumericToken(value string) string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r))
	})
	if len(parts) == 0 {
		return value
	}

	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		if isDigits(part) {
			number, err := strconv.Atoi(part)
			if err == nil {
				normalized = append(normalized, strconv.Itoa(number))
				continue
			}
		}
		normalized = append(normalized, part)
	}

	return strings.Join(normalized, " ")
}

func isDigits(value string) bool {
	for _, r := range value {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return value != ""
}

func filepathExt(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			return path[i:]
		}
		if path[i] == '/' || path[i] == '\\' {
			break
		}
	}
	return ""
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
