package ui

import "testing"

func widthSum(l Layout) int {
	sum := 0
	for _, w := range l.ColumnWidths {
		sum += w
	}
	return sum
}

func TestPadToWidth(t *testing.T) {
	if got := PadToWidth("ab", 5); got != "ab   " {
		t.Errorf("pad short = %q", got)
	}
	if got := PadToWidth("abcdef", 4); got != "abcd" {
		t.Errorf("trim long = %q", got)
	}
	if got := PadToWidth("abc", 3); got != "abc" {
		t.Errorf("exact width = %q", got)
	}
	if got := PadToWidth("abc", 0); got != "abc" {
		t.Errorf("zero width should return input, got %q", got)
	}
	if got := PadToWidth("", 3); got != "   " {
		t.Errorf("empty input = %q", got)
	}
}

func TestPadToWidthMultibyte(t *testing.T) {
	// Truncation must not slice mid-rune, and padding must use display
	// width, not byte length ("bücher" is 6 cells but 7 bytes).
	if got := PadToWidth("bücher", 4); got != "büch" {
		t.Errorf("multibyte trim = %q, want büch", got)
	}
	if got := PadToWidth("bücher", 8); got != "bücher  " {
		t.Errorf("multibyte pad = %q, want two trailing spaces", got)
	}
	if got := PadToWidth("✔", 3); got != "✔  " {
		t.Errorf("symbol pad = %q, want two trailing spaces", got)
	}
	// Wide CJK runes occupy two cells each.
	if got := PadToWidth("日本語", 4); got != "日本" {
		t.Errorf("wide trim = %q, want 日本", got)
	}
}

func TestComputeLayoutFitsTerminal(t *testing.T) {
	for _, width := range []int{40, 60, 80, 100, 120, 170, 250} {
		l := ComputeLayout(width)
		if len(l.ColumnWidths) != 11 {
			t.Fatalf("width %d: columns = %d, want 11", width, len(l.ColumnWidths))
		}
		if got := widthSum(l); got > width {
			t.Errorf("width %d: columns sum to %d, overflowing the terminal", width, got)
		}
		for i, w := range l.ColumnWidths {
			if w <= 0 {
				t.Errorf("width %d: column %d width = %d, want positive", width, i, w)
			}
		}
	}
}

func TestComputeLayoutWideTerminalGivesSlackToError(t *testing.T) {
	l := ComputeLayout(250)
	// The budget excludes one separator cell between each column pair plus
	// the scrollbar column, so columns sum to the width minus len(columns).
	if want := 250 - len(l.ColumnWidths); widthSum(l) != want {
		t.Errorf("wide layout sums to %d, want exactly %d", widthSum(l), want)
	}
	errCol := l.ColumnWidths[len(l.ColumnWidths)-1]
	if errCol < 50 {
		t.Errorf("error column = %d, want the leftover slack", errCol)
	}
}

func TestComputeLayoutDefaults(t *testing.T) {
	l := ComputeLayout(0)
	if l.Width != 80 {
		t.Errorf("Width for 0 input = %d, want default 80", l.Width)
	}
	if got := widthSum(l); got > 80 {
		t.Errorf("default layout sums to %d, overflowing 80 columns", got)
	}
}
