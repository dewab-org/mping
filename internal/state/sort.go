package state

import (
	"math"
	"strings"
	"time"
)

// less orders two snapshots for the given key and direction. The primary key
// is inverted for descending order, but ties always fall back to the display
// name ascending so equal rows keep a stable, predictable order.
func less(a, b HostSnapshot, key SortKey, dir SortDirection, now time.Time) bool {
	c := compareByKey(a, b, key, now)
	if c == 0 {
		return displayName(a.ResolvedName) < displayName(b.ResolvedName)
	}
	if dir == SortDesc {
		return c > 0
	}
	return c < 0
}

// compareByKey returns -1, 0, or 1 comparing a and b on the sort key. The
// caller supplies the clock reading so every comparison in one sort pass
// sees the same instant.
func compareByKey(a, b HostSnapshot, key SortKey, now time.Time) int {
	switch key {
	case SortRTT:
		return compareOrdered(a.LastRTT, b.LastRTT)
	case SortIP:
		return strings.Compare(a.IP, b.IP)
	case SortSuccess:
		return compareOrdered(a.SuccessCount, b.SuccessCount)
	case SortSuccessPct:
		aPct := pct(a.SuccessCount, a.SuccessCount+a.FailureCount)
		bPct := pct(b.SuccessCount, b.SuccessCount+b.FailureCount)
		return compareOrdered(aPct, bPct)
	case SortFailure:
		return compareOrdered(a.FailureCount, b.FailureCount)
	case SortLastOK:
		return compareOrdered(elapsedOrMax(now, a.LastOK), elapsedOrMax(now, b.LastOK))
	case SortError:
		return strings.Compare(a.LastError, b.LastError)
	default:
		return strings.Compare(displayName(a.ResolvedName), displayName(b.ResolvedName))
	}
}

func compareOrdered[T int64 | float64 | time.Duration](a, b T) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func elapsedOrMax(now time.Time, ts time.Time) time.Duration {
	if ts.IsZero() {
		return time.Duration(math.MaxInt64)
	}
	return now.Sub(ts)
}

func displayName(resolvedName string) string {
	name := strings.TrimSpace(resolvedName)
	if name == "" || strings.EqualFold(name, "N/A") {
		return "~n/a~" // tilde to sort after Z
	}
	return name
}

func pct(success, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return float64(success) / float64(total)
}
