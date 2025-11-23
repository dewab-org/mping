package state

import (
	"math"
	"strings"
	"time"
)

func less(hosts map[string]*HostState, a, b string, key SortKey, dir SortDirection) bool {
	ha := hosts[a]
	hb := hosts[b]
	var result bool

	nameA := displayName(ha)
	nameB := displayName(hb)

	switch key {
	case SortRTT:
		if ha.LastRTT == hb.LastRTT {
			result = strings.Compare(nameA, nameB) < 0
		} else {
			result = ha.LastRTT < hb.LastRTT
		}
	case SortIP:
		if ha.IP == hb.IP {
			result = strings.Compare(nameA, nameB) < 0
		} else {
			result = strings.Compare(ha.IP, hb.IP) < 0
		}
	case SortSuccess:
		if ha.SuccessCount == hb.SuccessCount {
			result = strings.Compare(nameA, nameB) < 0
		} else {
			result = ha.SuccessCount < hb.SuccessCount
		}
	case SortSuccessPct:
		aTotal := ha.SuccessCount + ha.FailureCount
		bTotal := hb.SuccessCount + hb.FailureCount
		aPct := pct(ha.SuccessCount, aTotal)
		bPct := pct(hb.SuccessCount, bTotal)
		if aPct == bPct {
			result = strings.Compare(nameA, nameB) < 0
		} else {
			result = aPct < bPct
		}
	case SortFailure:
		if ha.FailureCount == hb.FailureCount {
			result = strings.Compare(nameA, nameB) < 0
		} else {
			result = ha.FailureCount < hb.FailureCount
		}
	case SortLastOK:
		now := time.Now()
		aElapsed := elapsedOrMax(now, ha.LastOK)
		bElapsed := elapsedOrMax(now, hb.LastOK)
		if aElapsed == bElapsed {
			result = strings.Compare(nameA, nameB) < 0
		} else {
			result = aElapsed < bElapsed
		}
	case SortError:
		if ha.LastError == hb.LastError {
			result = strings.Compare(nameA, nameB) < 0
		} else {
			result = strings.Compare(ha.LastError, hb.LastError) < 0
		}
	default:
		result = strings.Compare(nameA, nameB) < 0
	}

	if dir == SortDesc {
		return !result
	}
	return result
}

func elapsedOrMax(now time.Time, ts time.Time) time.Duration {
	if ts.IsZero() {
		return time.Duration(math.MaxInt64)
	}
	return now.Sub(ts)
}

func displayName(h *HostState) string {
	name := strings.TrimSpace(h.ResolvedName)
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
