package ui

import (
	"testing"
	"time"

	"github.com/rivo/tview"

	"mping/internal/config"
	"mping/internal/ping"
	"mping/internal/state"
	"mping/internal/theme"
)

func newTestUI(t *testing.T, st *state.SharedState) (*UI, *string) {
	t.Helper()
	var deleted string
	cb := Callbacks{
		DeleteHost: func(key string) { deleted = key },
	}
	ui := NewUI(tview.NewApplication(), st, theme.Theme{}, config.Settings{}, cb, nil)
	return ui, &deleted
}

// deleteSelected must target the host that was rendered at the selected row,
// even if the underlying sort order has changed since the last render.
func TestDeleteSelectedUsesRenderedOrder(t *testing.T) {
	st := state.NewSharedState(0)
	for _, h := range []string{"alpha", "bravo"} {
		if err := st.AddHost(h, time.Second, time.Second); err != nil {
			t.Fatalf("AddHost: %v", err)
		}
	}
	st.SetSort(state.SortRTT, state.SortAsc)
	st.ApplyResult("alpha", ping.PingResult{Success: true, RTT: 10 * time.Millisecond}, nil)
	st.ApplyResult("bravo", ping.PingResult{Success: true, RTT: 500 * time.Millisecond}, nil)

	ui, deleted := newTestUI(t, st)
	ui.Refresh() // renders [alpha bravo]
	ui.Table.Select(1, 0)

	// A late result reorders state *after* the render: bravo is now fastest.
	st.ApplyResult("bravo", ping.PingResult{Success: true, RTT: time.Millisecond}, nil)

	ui.deleteSelected()
	if *deleted != "alpha" {
		t.Errorf("deleted %q, want alpha (the host rendered at the selected row)", *deleted)
	}
}

func TestDeleteSelectedOutOfRange(t *testing.T) {
	st := state.NewSharedState(0)
	ui, deleted := newTestUI(t, st)
	ui.Refresh()
	ui.deleteSelected() // empty table: header row selected, no host keys
	if *deleted != "" {
		t.Errorf("deleted %q, want no deletion on empty table", *deleted)
	}
}

func TestRenderTablePopulatesRowKeysInSortedOrder(t *testing.T) {
	st := state.NewSharedState(0)
	for _, h := range []string{"bravo", "alpha"} {
		if err := st.AddHost(h, time.Second, time.Second); err != nil {
			t.Fatalf("AddHost: %v", err)
		}
	}
	ui, _ := newTestUI(t, st)
	ui.Refresh()
	if len(ui.rowKeys) != 2 || ui.rowKeys[0] != "alpha" || ui.rowKeys[1] != "bravo" {
		t.Errorf("rowKeys = %v, want [alpha bravo]", ui.rowKeys)
	}
	st.DeleteHost("alpha")
	ui.Refresh()
	if len(ui.rowKeys) != 1 || ui.rowKeys[0] != "bravo" {
		t.Errorf("rowKeys after delete = %v, want [bravo]", ui.rowKeys)
	}
}
