package ui

import "testing"

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

func TestComputeLayout(t *testing.T) {
	l := ComputeLayout(200)
	if l.Width != 200 {
		t.Errorf("Width = %d, want 200", l.Width)
	}
	if len(l.ColumnWidths) != 11 {
		t.Fatalf("columns = %d, want 11", len(l.ColumnWidths))
	}
	if l.ColumnWidths[0] != 100 {
		t.Errorf("host column = %d, want half of total", l.ColumnWidths[0])
	}
	for i, w := range l.ColumnWidths {
		if w <= 0 {
			t.Errorf("column %d width = %d, want positive", i, w)
		}
	}
}

func TestComputeLayoutDefaultsAndMinimums(t *testing.T) {
	l := ComputeLayout(0)
	if l.Width != 80 {
		t.Errorf("Width for 0 input = %d, want default 80", l.Width)
	}
	narrow := ComputeLayout(20)
	if narrow.ColumnWidths[0] != 12 {
		t.Errorf("host column floor = %d, want 12", narrow.ColumnWidths[0])
	}
	if errCol := narrow.ColumnWidths[len(narrow.ColumnWidths)-1]; errCol < 10 {
		t.Errorf("error column = %d, want >= 10", errCol)
	}
}
