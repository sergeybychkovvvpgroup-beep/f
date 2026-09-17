package ui

// Theme is an internal fixed palette for the picker UI. It is intentionally not
// user-configurable: aoo should behave the same on every new machine.
type Theme struct {
	Name         string
	TitleFG      string
	TitleDimFG   string
	SelectedFG   string
	SelectedBG   string
	DetailFG     string
	HelpFG       string
	InputFG      string
	InputBG      string
	InputBorder  string
	InputPrompt  string
	RowFG        string
	DividerFG    string
	SelectedMark string
	StatusOKFG   string
	StatusWarnFG string
	StatusErrFG  string
	StatusRunFG  string
}

func DefaultTheme() Theme {
	return Theme{
		Name:         "default",
		TitleFG:      "#cdd6f4",
		TitleDimFG:   "#6c7086",
		SelectedFG:   "#1e1e2e",
		SelectedBG:   "#cdd6f4",
		DetailFG:     "#a6adc8",
		HelpFG:       "#6c7086",
		InputFG:      "#cdd6f4",
		InputBG:      "",
		InputBorder:  "#a6adc8",
		InputPrompt:  "#94e2d5",
		RowFG:        "#cdd6f4",
		DividerFG:    "#45475a",
		SelectedMark: "▸",
		StatusOKFG:   "#a6e3a1",
		StatusWarnFG: "#f9e2af",
		StatusErrFG:  "#f38ba8",
		StatusRunFG:  "#89b4fa",
	}
}
