package ui

import "github.com/mattn/go-runewidth"

type Layout struct {
	ColumnWidths []int
	Width        int
}

// minColumnWidth is the floor a column can be squeezed to on narrow terminals.
const minColumnWidth = 3

// ComputeLayout calculates column widths based on terminal width. tview draws
// one separator cell between adjacent columns and the scroll bar takes the
// last cell, so the columns' budget is totalWidth minus that overhead. The
// widths always sum to at most the budget (down to one cell per column), so
// every column stays on screen.
func ComputeLayout(totalWidth int) Layout {
	if totalWidth <= 0 {
		totalWidth = 80
	}

	// Column order: Host, Mode, IP, RTT, Status, OK, Success%, Success, Fail, LastOK, Error
	host := totalWidth * 3 / 10
	if host < 12 {
		host = 12
	}
	widths := []int{
		host,
		10, // Mode
		15, // IP
		9,  // RTT
		12, // Status
		4,  // OK
		9,  // Success%
		8,  // Success
		6,  // Fail
		8,  // LastOK
		8,  // Error (grows to fill leftover space)
	}

	// len(widths)-1 inter-column separators plus the scrollbar column.
	budget := totalWidth - len(widths)
	if budget < len(widths) {
		budget = len(widths)
	}

	sum := 0
	for _, w := range widths {
		sum += w
	}

	if sum <= budget {
		widths[len(widths)-1] += budget - sum
		return Layout{ColumnWidths: widths, Width: totalWidth}
	}

	// Too narrow for the preferred widths: shrink proportionally, then trim
	// any overshoot left by the per-column floor, widest columns first.
	scaled := 0
	for i, w := range widths {
		nw := w * budget / sum
		if nw < minColumnWidth {
			nw = minColumnWidth
		}
		widths[i] = nw
		scaled += nw
	}
	for scaled > budget {
		widest := 0
		for i, w := range widths {
			if w > widths[widest] {
				widest = i
			}
		}
		if widths[widest] <= 1 {
			break
		}
		widths[widest]--
		scaled--
	}

	return Layout{ColumnWidths: widths, Width: totalWidth}
}

// PadToWidth pads or trims a string to the specified display width,
// accounting for multi-byte and wide characters.
func PadToWidth(s string, width int) string {
	if width <= 0 {
		return s
	}
	if runewidth.StringWidth(s) > width {
		s = runewidth.Truncate(s, width, "")
	}
	return runewidth.FillRight(s, width)
}
