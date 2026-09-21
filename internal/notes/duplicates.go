package notes

import (
	"sort"
	"strings"
)

type DuplicateKind string

const (
	DuplicateExact       DuplicateKind = "exact_entry"
	DuplicateCommand     DuplicateKind = "command"
	DuplicateText        DuplicateKind = "text"
	DuplicateDescription DuplicateKind = "description"
)

type Location struct {
	Path string `json:"path"`
	Line int    `json:"line"`
}

type Duplicate struct {
	Kind      DuplicateKind `json:"kind"`
	Value     string        `json:"value"`
	Locations []Location    `json:"locations"`
}

func AnalyzeDuplicates(entries []Entry) []Duplicate {
	groups := map[DuplicateKind]map[string][]Location{
		DuplicateExact: {}, DuplicateCommand: {}, DuplicateText: {}, DuplicateDescription: {},
	}
	values := map[DuplicateKind]map[string]string{
		DuplicateExact: {}, DuplicateCommand: {}, DuplicateText: {}, DuplicateDescription: {},
	}
	add := func(kind DuplicateKind, key, value string, entry Entry) {
		if key == "" {
			return
		}
		groups[kind][key] = append(groups[kind][key], Location{Path: entry.SourcePath, Line: entry.SourceLine})
		values[kind][key] = value
	}
	for _, entry := range entries {
		desc := normalizeDuplicateText(entry.Desc, true)
		add(DuplicateDescription, desc, desc, entry)
		exact := []string{desc, "mode:" + normalizeDuplicateText(entry.Mode, true)}
		for _, action := range entry.ActionsList() {
			exact = append(exact,
				"action-desc:"+normalizeDuplicateText(action.Desc, true),
				"banner:"+normalizeDuplicateText(action.Banner, false),
			)
			if command := normalizeDuplicateCommand(action.Cmd); command != "" {
				add(DuplicateCommand, command, command, entry)
				exact = append(exact, "cmd:"+command)
			} else {
				exact = append(exact, "cmd:")
			}
			if text := normalizeDuplicateText(action.Text, false); text != "" {
				add(DuplicateText, text, text, entry)
				exact = append(exact, "text:"+text)
			} else {
				exact = append(exact, "text:")
			}
		}
		if len(entry.ActionsList()) > 0 {
			add(DuplicateExact, strings.Join(exact, "\x00"), entry.DisplayName(), entry)
		}
	}
	var out []Duplicate
	for kind, byValue := range groups {
		for key, locations := range byValue {
			if len(locations) < 2 {
				continue
			}
			sort.Slice(locations, func(i, j int) bool {
				if locations[i].Path == locations[j].Path {
					return locations[i].Line < locations[j].Line
				}
				return locations[i].Path < locations[j].Path
			})
			out = append(out, Duplicate{Kind: kind, Value: values[kind][key], Locations: locations})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind == out[j].Kind {
			return out[i].Value < out[j].Value
		}
		return out[i].Kind < out[j].Kind
	})
	return out
}

func normalizeDuplicateText(value string, foldCase bool) string {
	value = strings.Join(strings.Fields(value), " ")
	if foldCase {
		value = strings.ToLower(value)
	}
	return value
}

func normalizeDuplicateCommand(value string) string {
	return strings.TrimSpace(strings.ReplaceAll(value, "\r\n", "\n"))
}
