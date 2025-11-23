package ui

import "strings"

type Layout struct {
	ColumnWidths []int
	Width        int
}

// ComputeLayout calculates column widths based on terminal width.
func ComputeLayout(totalWidth int) Layout {
	if totalWidth <= 0 {
		totalWidth = 80
	}

	// Column order: Host, IP, RTT, OK, Success%, Success, Fail, LastOK, Error
	hostWidth := totalWidth / 2
	if hostWidth < 12 {
		hostWidth = 12
	}

	mins := []int{
		hostWidth, // Host (takes 50% of width)
		15,        // IP
		8,         // RTT
		4,         // OK
		10,        // Success%
		8,         // Success
		6,         // Fail
		10,        // LastOK
	}

	used := 0
	for _, w := range mins {
		used += w
	}

	remaining := totalWidth - used
	if remaining < 10 {
		remaining = 10
	}

	widths := append([]int{}, mins...)
	widths = append(widths, remaining) // Error column takes the rest

	return Layout{
		ColumnWidths: widths,
		Width:        totalWidth,
	}
}

// PadToWidth pads or trims a string to the specified width.
func PadToWidth(s string, width int) string {
	if width <= 0 {
		return s
	}
	if len(s) > width {
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
}
