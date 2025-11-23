package ui

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"mping/internal/state"
	"mping/internal/theme"
)

// ModalBuilder assembles themed modal dialogs.
type ModalBuilder struct {
	Theme theme.Theme
}

func (m ModalBuilder) AddHostsModal(submit func(string), cancel func()) tview.Primitive {
	form := tview.NewForm()
	form.AddTextArea("Hosts", "", 0, 5, 0, nil)
	form.AddButton("Add", func() {
		text := form.GetFormItemByLabel("Hosts").(*tview.TextArea).GetText()
		submit(text)
	})
	form.AddButton("Cancel", cancel)
	form.SetCancelFunc(cancel)
	form.SetBorder(true).SetTitle("Add Hosts")
	form.SetTitleColor(m.Theme.ModalBorderForeground)
	form.SetBorderColor(m.Theme.ModalBorderBackground)
	applyButtonStyles(form, m.Theme)
	return center(form, 90, 12)
}

func (m ModalBuilder) IntervalModal(current string, submit func(string), cancel func()) tview.Primitive {
	return m.valueModal("Set Interval (seconds)", current, submit, cancel)
}

func (m ModalBuilder) TimeoutModal(current string, submit func(string), cancel func()) tview.Primitive {
	return m.valueModal("Set Timeout (seconds)", current, submit, cancel)
}

func (m ModalBuilder) valueModal(title, current string, submit func(string), cancel func()) tview.Primitive {
	form := tview.NewForm()
	form.AddInputField(title, current, 20, nil, nil)
	form.AddButton("OK", func() {
		val := form.GetFormItemByLabel(title).(*tview.InputField).GetText()
		submit(val)
	})
	form.AddButton("Cancel", cancel)
	form.SetCancelFunc(cancel)
	form.SetBorder(true).SetTitle(title)
	form.SetTitleColor(m.Theme.ModalBorderForeground)
	form.SetBorderColor(m.Theme.ModalBorderBackground)
	applyButtonStyles(form, m.Theme)
	return center(form, 60, 7)
}

func (m ModalBuilder) SortModal(current state.SortKey, dir state.SortDirection, submit func(state.SortKey, state.SortDirection), cancel func()) tview.Primitive {
	keys := []state.SortKey{state.SortHost, state.SortIP, state.SortRTT, state.SortSuccessPct, state.SortSuccess, state.SortFailure, state.SortLastOK, state.SortError}
	keyLabels := []string{"Hostname", "IP", "RTT", "Success%", "Success", "Failure", "Last OK", "Error"}
	keyIdx := 0
	for i, k := range keys {
		if k == current {
			keyIdx = i
			break
		}
	}
	dirIdx := 0
	if dir == state.SortDesc {
		dirIdx = 1
	}

	form := tview.NewForm()
	form.AddDropDown("Sort Key", keyLabels, keyIdx, nil)
	form.AddDropDown("Direction", []string{"Ascending", "Descending"}, dirIdx, nil)
	form.AddButton("Apply", func() {
		_, keyChoice := form.GetFormItem(0).(*tview.DropDown).GetCurrentOption()
		var selectedKey state.SortKey = state.SortHost
		switch keyChoice {
		case "RTT":
			selectedKey = state.SortRTT
		case "IP":
			selectedKey = state.SortIP
		case "Success%":
			selectedKey = state.SortSuccessPct
		case "Success":
			selectedKey = state.SortSuccess
		case "Failure":
			selectedKey = state.SortFailure
		case "Last OK":
			selectedKey = state.SortLastOK
		case "Error":
			selectedKey = state.SortError
		default:
			selectedKey = state.SortHost
		}

		_, dirChoice := form.GetFormItem(1).(*tview.DropDown).GetCurrentOption()
		selectedDir := state.SortAsc
		if dirChoice == "Descending" {
			selectedDir = state.SortDesc
		}
		submit(selectedKey, selectedDir)
	})
	form.AddButton("Cancel", cancel)
	form.SetCancelFunc(cancel)
	form.SetBorder(true).SetTitle("Sort Options")
	form.SetTitleColor(m.Theme.ModalBorderForeground)
	form.SetBorderColor(m.Theme.ModalBorderBackground)
	applyButtonStyles(form, m.Theme)
	return center(form, 60, 9)
}

func (m ModalBuilder) HelpModal(cancel func()) tview.Primitive {
	helpText := strings.Join([]string{
		"Keybindings:",
		"Up/Down: Move selection",
		"PageUp/PageDown: Scroll page",
		"a: Add hosts",
		"d: Delete selected host",
		"o: Sort options",
		"s: Settings",
		"r: Reverse sort direction",
		"i: Set interval",
		"t: Set timeout",
		"h or ?: Help",
		"q or Ctrl+C: Quit",
	}, "\n")

	text := tview.NewTextView().
		SetText(helpText).
		SetDynamicColors(true).
		SetWrap(true)
	text.SetBorder(true).SetTitle("Help")
	text.SetBorderColor(m.Theme.ModalBorderBackground)
	text.SetTitleColor(m.Theme.ModalBorderForeground)

	frame := tview.NewFrame(text).
		AddText("", true, tview.AlignLeft, m.Theme.ModalBorderForeground).
		AddText("[Cancel] Esc", false, tview.AlignRight, m.Theme.ModalBorderForeground)
	frame.SetBorders(0, 0, 0, 0, 0, 0)

	box := center(frame, 70, 15)
	box.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			cancel()
			return nil
		}
		if event.Rune() == 'h' || event.Rune() == '?' || event.Rune() == 'H' {
			cancel()
			return nil
		}
		return event
	})
	return box
}

func (m ModalBuilder) SettingsModal(
	current state.SortKey,
	dir state.SortDirection,
	interval,
	timeout,
	refresh string,
	backend string,
	sysArgs string,
	themes []string,
	currentTheme string,
	submit func(state.SortKey, state.SortDirection, string, string, string, string, string, string),
	cancel func(),
) tview.Primitive {
	form := tview.NewForm()
	form.SetItemPadding(0)

	form.AddTextView("Ping", "[::b]Ping[::-]", 0, 1, true, false)
	backendDrop := tview.NewDropDown().SetLabel("Backend").SetOptions([]string{"system", "native"}, nil)
	if backend == "native" {
		backendDrop.SetCurrentOption(1)
	} else {
		backendDrop.SetCurrentOption(0)
	}
	form.AddFormItem(backendDrop)
	form.AddInputField("Interval (s)", interval, 3, nil, nil)
	form.AddInputField("Timeout (s)", timeout, 3, nil, nil)
	form.AddInputField("System Args", sysArgs, 30, nil, nil)
	form.AddTextView("", "", 0, 1, true, false)

	form.AddTextView("Sort", "[::b]Sort[::-]", 0, 1, true, false)
	keyLabels := []string{"Hostname", "IP", "RTT", "Success%", "Success", "Failure", "Last OK", "Error"}
	keyDrop := tview.NewDropDown().SetLabel("Sort Key").SetOptions(keyLabels, nil)
	keyDrop.SetFieldWidth(12)
	keyIdx := 0
	for i, lbl := range keyLabels {
		if toSortKey(lbl) == current {
			keyIdx = i
			break
		}
	}
	keyDrop.SetCurrentOption(keyIdx)
	dirDrop := tview.NewDropDown().SetLabel("Dir").SetOptions([]string{"Asc", "Desc"}, nil)
	dirDrop.SetFieldWidth(6)
	if dir == state.SortDesc {
		dirDrop.SetCurrentOption(1)
	} else {
		dirDrop.SetCurrentOption(0)
	}
	form.AddFormItem(keyDrop)
	form.AddFormItem(dirDrop)
	form.AddTextView("", "", 0, 1, true, false)

	form.AddTextView("Display", "[::b]Display[::-]", 0, 1, true, false)

	themeDrop := tview.NewDropDown().SetLabel("Theme").SetOptions(themes, nil)
	themeIdx := 0
	for i, t := range themes {
		if t == currentTheme {
			themeIdx = i
			break
		}
	}
	themeDrop.SetCurrentOption(themeIdx)

	form.AddFormItem(themeDrop)
	form.AddInputField("Refresh (s)", refresh, 10, nil, nil)
	form.AddTextView("", "", 0, 1, true, false)
	form.AddButton("Apply", func() {
		_, keyChoice := keyDrop.GetCurrentOption()
		selectedKey := toSortKey(keyChoice)
		_, dirChoice := dirDrop.GetCurrentOption()
		selectedDir := state.SortAsc
		if dirChoice == "Desc" {
			selectedDir = state.SortDesc
		}

		intervalVal := form.GetFormItemByLabel("Interval (s)").(*tview.InputField).GetText()
		timeoutVal := form.GetFormItemByLabel("Timeout (s)").(*tview.InputField).GetText()

		selectedTheme := themes[themeIdx]
		if idx, option := themeDrop.GetCurrentOption(); idx >= 0 {
			selectedTheme = option
		}

		refreshVal := form.GetFormItemByLabel("Refresh (s)").(*tview.InputField).GetText()
		argsVal := form.GetFormItemByLabel("System Args").(*tview.InputField).GetText()
		selectedBackend := "system"
		if idx, option := backendDrop.GetCurrentOption(); idx >= 0 {
			selectedBackend = option
		}

		submit(selectedKey, selectedDir, intervalVal, timeoutVal, refreshVal, selectedTheme, selectedBackend, argsVal)
	})
	form.AddButton("Cancel", cancel)
	form.SetCancelFunc(cancel)
	form.SetBorder(true).SetTitle("Settings")
	form.SetTitleColor(m.Theme.ModalBorderForeground)
	form.SetBorderColor(m.Theme.ModalBorderBackground)
	applyButtonStyles(form, m.Theme)
	height := form.GetFormItemCount() + form.GetButtonCount() + 4
	if height < 10 {
		height = 10
	}
	return center(form, 60, height)
}

func toSortKey(option string) state.SortKey {
	switch option {
	case "IP":
		return state.SortIP
	case "RTT":
		return state.SortRTT
	case "Success%":
		return state.SortSuccessPct
	case "Success":
		return state.SortSuccess
	case "Failure":
		return state.SortFailure
	case "Last OK":
		return state.SortLastOK
	case "Error":
		return state.SortError
	default:
		return state.SortHost
	}
}

func applyButtonStyles(form *tview.Form, th theme.Theme) {
	for i := 0; i < form.GetButtonCount(); i++ {
		btn := form.GetButton(i)
		label := strings.ToLower(btn.GetLabel())
		switch label {
		case "ok", "add", "apply", "save":
			btn.SetLabelColor(th.ButtonOKForeground)
			btn.SetLabelColorActivated(th.ButtonOKForeground)
			btn.SetBackgroundColor(th.ButtonOKBackground)
			btn.SetBackgroundColorActivated(th.ButtonOKBackground)
		case "cancel", "close":
			btn.SetLabelColor(th.ButtonCancelForeground)
			btn.SetLabelColorActivated(th.ButtonCancelForeground)
			btn.SetBackgroundColor(th.ButtonCancelBackground)
			btn.SetBackgroundColorActivated(th.ButtonCancelBackground)
		default:
			btn.SetLabelColor(th.ModalBorderForeground)
			btn.SetLabelColorActivated(th.ModalBorderForeground)
			btn.SetBackgroundColor(th.ModalBorderBackground)
			btn.SetBackgroundColorActivated(th.ModalBorderBackground)
		}
	}
}
func center(content tview.Primitive, width, height int) *tview.Grid {
	grid := tview.NewGrid().
		SetRows(0, height, 0).
		SetColumns(0, width, 0).
		AddItem(content, 1, 1, 1, 1, 0, 0, true)
	return grid
}
