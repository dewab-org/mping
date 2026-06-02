package ui

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"mping/internal/config"
	"mping/internal/state"
	"mping/internal/theme"
)

type Callbacks struct {
	AddHosts           func(string)
	DeleteHost         func(string)
	SetInterval        func(time.Duration)
	SetTimeout         func(time.Duration)
	SetRefreshInterval func(time.Duration)
	SetSort            func(state.SortKey, state.SortDirection)
	ReverseSort        func()
	SetTheme           func(string)
	SetBackend         func(string, string, string, int)
	Quit               func()
}

// UI glues together the tview widgets and shared state.
type UI struct {
	App          *tview.Application
	Pages        *tview.Pages
	Table        *tview.Table
	ScrollBar    *tview.TextView
	TitleBar     *tview.TextView
	StatusBar    *tview.TextView
	Layout       *tview.Flex
	State        *state.SharedState
	Theme        theme.Theme
	Builder      ModalBuilder
	Config       config.Settings
	Callbacks    Callbacks
	Themes       []string
	ThemeName    string
	layoutInfo   Layout
	lastWidth    int
	lastRowCount int
}

func NewUI(app *tview.Application, st *state.SharedState, th theme.Theme, cfg config.Settings, cb Callbacks, themes []string) *UI {
	title := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	title.SetBackgroundColor(th.TitleBackground)
	title.SetTextColor(th.TitleForeground)

	table := tview.NewTable().
		SetFixed(1, 0).
		SetSelectable(true, false)
	table.SetBorder(false)

	scroll := tview.NewTextView().
		SetDynamicColors(false).
		SetTextAlign(tview.AlignCenter)
	scroll.SetBorder(false)
	scroll.SetBackgroundColor(th.StatusBackground)
	scroll.SetTextColor(th.StatusForeground)

	status := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	status.SetBackgroundColor(th.StatusBackground)
	status.SetTextColor(th.StatusForeground)

	pages := tview.NewPages()

	tableRow := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(table, 0, 1, true).
		AddItem(scroll, 1, 0, false)

	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(title, 1, 0, false).
		AddItem(tableRow, 0, 1, true).
		AddItem(status, 1, 0, false)
	pages.AddPage("main", layout, true, true)

	ui := &UI{
		App:       app,
		Pages:     pages,
		Table:     table,
		ScrollBar: scroll,
		TitleBar:  title,
		StatusBar: status,
		Layout:    layout,
		State:     st,
		Theme:     th,
		Builder:   ModalBuilder{Theme: th},
		Config:    cfg,
		Callbacks: cb,
		Themes:    themes,
		ThemeName: cfg.ThemeName,
	}

	ui.bindKeys()
	return ui
}

// Refresh rebuilds the title, status, and table rows.
func (u *UI) Refresh() {
	width := u.lastWidth
	if width <= 0 {
		width = 80
	}
	u.layoutInfo = ComputeLayout(width)
	u.renderTitle()
	u.renderStatus()
	u.renderTable()
}

func (u *UI) RefreshWithScreen(screen tcell.Screen) {
	w, _ := screen.Size()
	if w > 0 {
		u.lastWidth = w
	}
	u.Refresh()
}

func (u *UI) renderTitle() {
	key, dir := u.State.SortConfig()
	dirSymbol := "↑"
	if dir == state.SortDesc {
		dirSymbol = "↓"
	}
	mode := u.Config.Protocol
	if mode == "tcp" {
		mode = fmt.Sprintf("tcp:%d", u.Config.TCPPort)
	} else if mode == "icmp" {
		mode = fmt.Sprintf("icmp/%s", u.Config.Backend)
	}
	title := fmt.Sprintf(" mping | hosts: %d | mode: %s | sort: %s%s | workers: %d | interval: %s | timeout: %s | refresh: %s | theme: %s | config: %s ",
		u.State.Count(),
		mode,
		key, dirSymbol,
		u.Config.MaxConcurrentPings,
		u.Config.Interval,
		u.Config.Timeout,
		u.Config.RefreshInterval,
		u.ThemeName,
		statusPath(u.Config.ConfigPath))
	u.TitleBar.SetText(PadToWidth(title, u.layoutInfo.Width))
}

func (u *UI) renderStatus() {
	status := " keys: a add, d del, o sort cycle, s settings, r reverse, i interval, t timeout, h/? help, q quit "
	u.StatusBar.SetText(PadToWidth(status, u.layoutInfo.Width))
}

func statusPath(path string) string {
	if path == "" {
		return "default"
	}
	return path
}

func (u *UI) renderTable() {
	u.Table.Clear()
	snap := u.State.Snapshot()
	u.lastRowCount = len(snap) + 1
	header := []string{"Hostname", "Mode", "IP", "RTT", "Status", "OK", "Success%", "Success", "Fail", "Last OK", "Error"}
	for c, h := range header {
		cell := tview.NewTableCell(h).
			SetTextColor(u.Theme.HeaderForeground).
			SetBackgroundColor(u.Theme.HeaderBackground).
			SetSelectable(false)
		u.Table.SetCell(0, c, cell)
	}

	for i, host := range snap {
		row := i + 1
		okText := "✖"
		okColor := u.Theme.OKTextFailure
		success := false
		if host.LastOK.After(time.Time{}) && host.LastError == "" {
			success = true
		}
		if success {
			okText = "✔"
			okColor = u.Theme.OKTextSuccess
		}

		successPct := 0.0
		total := host.SuccessCount + host.FailureCount
		if total > 0 {
			successPct = float64(host.SuccessCount) / float64(total) * 100
		}

		lastOK := "-"
		if !host.LastOK.IsZero() {
			lastOK = humanSince(host.LastOK)
		}

		resolved := host.ResolvedName
		if strings.TrimSpace(resolved) == "" || strings.EqualFold(resolved, "N/A") {
			resolved = "-n/a-"
		}

		values := []string{
			resolved,
			modeLabel(host.Protocol, host.TCPPort),
			host.IP,
			fmt.Sprintf("%.2fs", host.LastRTT.Seconds()),
			statusLabel(host.LastStatus),
			okText,
			fmt.Sprintf("%.1f%%", successPct),
			strconv.FormatInt(host.SuccessCount, 10),
			strconv.FormatInt(host.FailureCount, 10),
			lastOK,
			host.LastError,
		}

		for c, v := range values {
			cellText := PadToWidth(v, u.layoutInfo.ColumnWidths[c])
			cell := tview.NewTableCell(cellText).
				SetTextColor(u.Theme.RowForeground)
			if c == len(values)-1 { // Error column
				cell.SetMaxWidth(u.layoutInfo.ColumnWidths[c])
				cell.SetExpansion(0)
			}
			if c == 5 { // OK column
				cell.SetTextColor(okColor)
			}
			if row%2 == 0 {
				cell.SetBackgroundColor(tcell.ColorReset)
			}
			u.Table.SetCell(row, c, cell)
		}
	}
	if rowOffset, _ := u.Table.GetOffset(); true {
		u.Table.SetOffset(rowOffset, 0)
	}
	u.updateScrollBar()
}

func modeLabel(protocol string, tcpPort int) string {
	if protocol == "tcp" {
		return fmt.Sprintf("tcp:%d", tcpPort)
	}
	return protocol
}

func statusLabel(status string) string {
	if strings.TrimSpace(status) == "" {
		return "-"
	}
	return status
}

func humanSince(t time.Time) string {
	d := time.Since(t)
	if d < time.Minute {
		secs := int(d.Seconds())
		if secs < 0 {
			secs = 0
		}
		return fmt.Sprintf("%ds", secs)
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh", int(d.Hours()))
}

func (u *UI) bindKeys() {
	u.Table.SetSelectionChangedFunc(func(row, column int) {
		u.updateScrollBar()
	})
	u.Table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'a':
			u.showAddHosts()
			return nil
		case 'd':
			u.deleteSelected()
			return nil
		case 'o':
			u.cycleSort()
			return nil
		case 's':
			u.showSettings()
			return nil
		case 'r':
			if u.Callbacks.ReverseSort != nil {
				u.Callbacks.ReverseSort()
				u.Refresh()
			}
			return nil
		case 'i':
			u.showInterval()
			return nil
		case 't':
			u.showTimeout()
			return nil
		case 'h':
			u.showHelp()
			return nil
		case '?':
			u.showHelp()
			return nil
		case 'q':
			if u.Callbacks.Quit != nil {
				u.Callbacks.Quit()
			}
			return nil
		}
		switch event.Key() {
		case tcell.KeyCtrlC:
			if u.Callbacks.Quit != nil {
				u.Callbacks.Quit()
			}
			return nil
		case tcell.KeyPgUp:
			u.Table.ScrollToBeginning()
			return nil
		case tcell.KeyPgDn:
			u.Table.ScrollToEnd()
			return nil
		}
		return event
	})
}

func (u *UI) showAddHosts() {
	modal := u.Builder.AddHostsModal(func(text string) {
		if u.Callbacks.AddHosts != nil {
			u.Callbacks.AddHosts(text)
		}
		u.closeModal()
	}, func() {
		u.closeModal()
	})
	u.Pages.AddPage("add", modal, true, true)
	u.App.SetFocus(modal)
}

func (u *UI) showInterval() {
	modal := u.Builder.IntervalModal(fmt.Sprintf("%.0f", u.Config.Interval.Seconds()), func(text string) {
		if val, err := strconv.Atoi(text); err == nil && u.Callbacks.SetInterval != nil {
			u.Callbacks.SetInterval(time.Duration(val) * time.Second)
		}
		u.closeModal()
	}, u.closeModal)
	u.Pages.AddPage("interval", modal, true, true)
	u.App.SetFocus(modal)
}

func (u *UI) showTimeout() {
	modal := u.Builder.TimeoutModal(fmt.Sprintf("%.0f", u.Config.Timeout.Seconds()), func(text string) {
		if val, err := strconv.Atoi(text); err == nil && u.Callbacks.SetTimeout != nil {
			u.Callbacks.SetTimeout(time.Duration(val) * time.Second)
		}
		u.closeModal()
	}, u.closeModal)
	u.Pages.AddPage("timeout", modal, true, true)
	u.App.SetFocus(modal)
}

func (u *UI) showSort() {
	key, dir := u.State.SortConfig()
	modal := u.Builder.SortModal(key, dir, func(k state.SortKey, d state.SortDirection) {
		if u.Callbacks.SetSort != nil {
			u.Callbacks.SetSort(k, d)
		}
		u.closeModal()
	}, u.closeModal)
	u.Pages.AddPage("sort", modal, true, true)
	u.App.SetFocus(modal)
}

func (u *UI) showSettings() {
	key, dir := u.State.SortConfig()
	modal := u.Builder.SettingsModal(
		key,
		dir,
		fmt.Sprintf("%.0f", u.Config.Interval.Seconds()),
		fmt.Sprintf("%.0f", u.Config.Timeout.Seconds()),
		fmt.Sprintf("%.0f", u.Config.RefreshInterval.Seconds()),
		u.Config.Protocol,
		fmt.Sprintf("%d", u.Config.TCPPort),
		u.Config.Backend,
		strings.Join(u.Config.SystemArgs, " "),
		u.Themes,
		u.ThemeName,
		func(k state.SortKey, d state.SortDirection, intervalVal, timeoutVal, refreshVal, themeVal, protocolVal, tcpPortVal, backendVal, argsVal string) {
			if secs, err := strconv.Atoi(intervalVal); err == nil && u.Callbacks.SetInterval != nil {
				u.Callbacks.SetInterval(time.Duration(secs) * time.Second)
			}
			if secs, err := strconv.Atoi(timeoutVal); err == nil && u.Callbacks.SetTimeout != nil {
				u.Callbacks.SetTimeout(time.Duration(secs) * time.Second)
			}
			if secs, err := strconv.Atoi(refreshVal); err == nil && u.Callbacks.SetRefreshInterval != nil {
				u.Callbacks.SetRefreshInterval(time.Duration(secs) * time.Second)
			}
			if u.Callbacks.SetTheme != nil {
				u.Callbacks.SetTheme(themeVal)
				u.ThemeName = themeVal
			}
			if u.Callbacks.SetBackend != nil {
				tcpPort, _ := strconv.Atoi(tcpPortVal)
				u.Callbacks.SetBackend(backendVal, argsVal, protocolVal, tcpPort)
			}
			if u.Callbacks.SetSort != nil {
				u.Callbacks.SetSort(k, d)
			}
			u.closeModal()
		},
		u.closeModal,
	)
	u.Pages.AddPage("settings", modal, true, true)
	u.App.SetFocus(modal)
}

func (u *UI) showHelp() {
	modal := u.Builder.HelpModal(u.closeModal)
	u.Pages.AddPage("help", modal, true, true)
	u.App.SetFocus(modal)
}

func (u *UI) closeModal() {
	u.Pages.RemovePage("add")
	u.Pages.RemovePage("interval")
	u.Pages.RemovePage("timeout")
	u.Pages.RemovePage("sort")
	u.Pages.RemovePage("settings")
	u.Pages.RemovePage("help")
	u.App.SetFocus(u.Table)
}

// UpdateTheme swaps the active theme and updates component colors.
func (u *UI) UpdateTheme(th theme.Theme, name string) {
	u.Theme = th
	u.ThemeName = name
	u.Builder.Theme = th
	u.TitleBar.SetBackgroundColor(th.TitleBackground)
	u.TitleBar.SetTextColor(th.TitleForeground)
	u.StatusBar.SetBackgroundColor(th.StatusBackground)
	u.StatusBar.SetTextColor(th.StatusForeground)
	u.Refresh()
}

func (u *UI) deleteSelected() {
	row, _ := u.Table.GetSelection()
	if row <= 0 {
		return
	}
	snap := u.State.Snapshot()
	idx := row - 1
	if idx < 0 || idx >= len(snap) {
		return
	}
	if u.Callbacks.DeleteHost != nil {
		u.Callbacks.DeleteHost(snap[idx].Key)
	}
}

func (u *UI) cycleSort() {
	key, dir := u.State.SortConfig()
	keys := []state.SortKey{state.SortHost, state.SortIP, state.SortRTT, state.SortSuccessPct, state.SortSuccess, state.SortFailure, state.SortLastOK, state.SortError}
	idx := 0
	for i, k := range keys {
		if k == key {
			idx = i
			break
		}
	}
	idx = (idx + 1) % len(keys)
	u.State.SetSort(keys[idx], dir)
	u.Refresh()
}

func (u *UI) updateScrollBar() {
	rowOffset, _ := u.Table.GetOffset()
	_, _, _, innerHeight := u.Table.GetInnerRect()
	if innerHeight <= 0 {
		innerHeight = u.lastRowCount
	}
	total := u.lastRowCount
	if total <= 0 {
		u.ScrollBar.SetText("")
		return
	}
	visible := innerHeight
	if visible > total {
		visible = total
	}
	barHeight := visible
	if barHeight < 1 {
		barHeight = 1
	}
	block := int(math.Max(1, float64(barHeight)*float64(visible)/float64(total)))
	var start int
	if total > visible {
		start = int(float64(rowOffset) / float64(total-visible) * float64(barHeight-block))
	}
	var b strings.Builder
	for i := 0; i < barHeight; i++ {
		if i >= start && i < start+block {
			b.WriteRune('█')
		} else {
			b.WriteRune('│')
		}
		b.WriteRune('\n')
	}
	u.ScrollBar.SetText(b.String())
}
