package privatequota

import (
	"cmp"
	"fmt"
	"slices"
	"time"
)

// SummarizeMonth按北京时间自然月归属增量；不把整轮消耗全放到结束月份。
func SummarizeMonth(ledger Ledger, month string, location *time.Location) (MonthSummary, error) {
	if location == nil || location.String() != "Asia/Shanghai" {
		return MonthSummary{}, fmt.Errorf("quota ledger requires Asia/Shanghai timezone")
	}
	start, err := time.ParseInLocation("2006-01", month, location)
	if err != nil || start.Format("2006-01") != month {
		return MonthSummary{}, fmt.Errorf("invalid quota ledger month")
	}
	from, to := start.Unix(), start.AddDate(0, 1, 0).Unix()
	summary := MonthSummary{Month: month, Timezone: location.String(), RulesVersion: ledger.RulesVersion,
		CoverageStart: ledger.CoverageStart}
	for _, delta := range ledger.Deltas {
		if delta.FetchedAt >= from && delta.FetchedAt < to {
			if window := summary.window(delta.WindowSeconds); window != nil {
				window.ObservedBasisPoints += delta.BasisPoints
			}
		}
	}
	for _, cycle := range ledger.Cycles {
		window := summary.window(cycle.WindowSeconds)
		if window == nil {
			continue
		}
		if cycle.StartedAt < to && (cycle.EndedAt == 0 || cycle.EndedAt > from) && ledger.LastObservedAt >= from {
			if cycle.Partial {
				window.PartialCycles++
			}
			if cycle.EndedAt == 0 {
				current := cycle
				window.Current = &current
			}
		}
		// 未用估算按结束后首个样本所在月归属，与增量的自然月口径不强行相加。
		if cycle.EndedAt >= from && cycle.EndedAt < to {
			if cycle.EndedBy == ChangeSegment || cycle.Partial || cycle.HasGap || cycle.HasCorrection {
				window.ExcludedCycles++
				continue
			}
			window.EvaluatedCycles++
			window.UnusedBasisPoints += int64(max(0, 10000-cycle.HighWaterBasisPoints))
		}
	}
	for _, event := range ledger.Events {
		window := summary.window(event.WindowSeconds)
		if window == nil {
			continue
		}
		if event.Status == EventPending && event.ObservedTo < to && ledger.LastObservedAt >= from {
			window.PendingEvents++
			if window.PendingSince == 0 || event.ObservedTo < window.PendingSince {
				window.PendingSince = event.ObservedTo
			}
		}
		if event.Status != EventAccepted || event.ObservedTo < from || event.ObservedTo >= to {
			continue
		}
		switch event.Kind {
		case ResetEarly:
			window.ResetCount++
			window.EarlyResets++
		case ResetExpired:
			window.ResetCount++
			window.ExpiredResets++
		case ResetManual:
			window.ResetCount++
			window.ManualResets++
		}
	}
	for _, rejected := range ledger.Rejected {
		if rejected.FetchedAt >= from && rejected.FetchedAt < to {
			summary.InvalidSamples++
			if window := summary.window(rejected.WindowSeconds); window != nil {
				window.InvalidSamples++
			}
		}
	}
	var gaps []ObservationGap
	for _, gap := range ledger.Gaps {
		left, right := max(from, gap.From), min(to, gap.To)
		if left < right {
			// 单一窗口缺口由相邻观测生成，互不重叠；全局仍保留原来的时间并集契约。
			if window := summary.window(gap.WindowSeconds); window != nil {
				window.GapSeconds += right - left
			}
			gaps = append(gaps, ObservationGap{From: left, To: right})
		}
	}
	slices.SortFunc(gaps, func(a, b ObservationGap) int { return cmp.Compare(a.From, b.From) })
	var mergedFrom, mergedTo int64
	for _, gap := range gaps {
		if mergedTo == 0 {
			mergedFrom, mergedTo = gap.From, gap.To
		} else if gap.From <= mergedTo {
			mergedTo = max(mergedTo, gap.To)
		} else {
			summary.GapSeconds += mergedTo - mergedFrom
			mergedFrom, mergedTo = gap.From, gap.To
		}
	}
	summary.GapSeconds += mergedTo - mergedFrom
	return summary, nil
}

func (summary *MonthSummary) window(seconds int64) *WindowSummary {
	switch seconds {
	case SevenDaySeconds:
		return &summary.SevenDay
	case FiveHourSeconds:
		return &summary.FiveHour
	default:
		return nil
	}
}
