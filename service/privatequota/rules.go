package privatequota

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
)

const resetTolerance int64 = 60
const significantDrop = 200

type pendingChange struct {
	first      Sample
	eventIndex int
	canReset   bool
	ambiguous  bool
}

type windowState struct {
	last        Sample
	cycleIndex  int
	dropHigh    int
	lowStreak   int
	interrupted bool
	pending     *pendingChange
}

// BuildLedger从单个主体的完整观测和有效决定重放，不改输入、不访问外部资源。
// 调用方不能只截取月内样本：必须提供月前基线及有关边界，月度筛选交给SummarizeMonth。
func BuildLedger(samples []Sample, decisions []Decision) (Ledger, error) {
	ledger := Ledger{RulesVersion: RulesVersion}
	effective, err := resolveDecisions(decisions)
	if err != nil {
		return Ledger{}, err
	}
	ordered := slices.Clone(samples)
	slices.SortFunc(ordered, func(a, b Sample) int {
		if n := cmp.Compare(a.FetchedAt, b.FetchedAt); n != 0 {
			return n
		}
		return cmp.Compare(a.ID, b.ID)
	})
	states := make(map[int64]*windowState)
	ids := make(map[int64]Sample)
	batches := make(map[string]Sample)
	matched := make(map[string]bool)
	for _, sample := range ordered {
		if ledger.SubjectID != 0 && sample.SubjectID > 0 && ledger.SubjectID != sample.SubjectID {
			return Ledger{}, fmt.Errorf("quota samples contain multiple subjects")
		}
		if sample.SubjectID > 0 {
			ledger.SubjectID = sample.SubjectID
		}
		batchKey := fmt.Sprintf("%d/%s", sample.WindowSeconds, sample.BatchID)
		if previous, ok := ids[sample.ID]; ok && sample.ID > 0 {
			if !sameObservation(previous, sample) {
				return Ledger{}, fmt.Errorf("conflicting quota snapshot id %d", sample.ID)
			}
			continue
		}
		if previous, ok := batches[batchKey]; ok && sample.BatchID != "" {
			if !sameObservation(previous, sample) {
				return Ledger{}, fmt.Errorf("conflicting quota sample batch")
			}
			continue
		}
		ids[sample.ID], batches[batchKey] = sample, sample
		state := states[sample.WindowSeconds]
		if reason := sample.invalidReason(); reason != "" {
			ledger.Rejected = append(ledger.Rejected, RejectedSample{SampleID: sample.ID, FetchedAt: sample.FetchedAt, Reason: reason, WindowSeconds: sample.WindowSeconds})
			if state != nil {
				// 边界前的坏样本同样会破坏证据链，不能只检查候选生成后的失败。
				state.interrupted = true
				ledger.Cycles[state.cycleIndex].HasGap = true
				if state.pending != nil {
					state.pending.ambiguous = true
				}
			}
			continue
		}
		if state != nil && sample.FetchedAt <= state.last.FetchedAt {
			ledger.Rejected = append(ledger.Rejected, RejectedSample{SampleID: sample.ID, FetchedAt: sample.FetchedAt, Reason: "not_a_new_observation", WindowSeconds: sample.WindowSeconds})
			continue
		}
		if ledger.CoverageStart == 0 {
			ledger.CoverageStart = sample.FetchedAt
		}
		ledger.LastObservedAt = sample.FetchedAt
		if state == nil {
			states[sample.WindowSeconds] = &windowState{last: sample, dropHigh: *sample.UsedBasisPoints,
				cycleIndex: ledger.startCycle(sample, true, false)}
			continue
		}
		interrupted := state.interrupted
		state.interrupted = false
		interval := state.last.IntervalSeconds
		if interval == 0 {
			interval = 600
		}
		hasGap := sample.FetchedAt-state.last.FetchedAt > max(3*interval, 1800)
		if hasGap {
			ledger.Gaps = append(ledger.Gaps, ObservationGap{sample.WindowSeconds, state.last.FetchedAt, sample.FetchedAt})
			ledger.Cycles[state.cycleIndex].HasGap = true
			if state.pending != nil {
				state.pending.ambiguous = true
			}
		}
		if sample.SegmentID != state.last.SegmentID {
			if state.pending != nil {
				ledger.Events[state.pending.eventIndex].Status = EventInconclusive
			}
			ledger.Events = append(ledger.Events, boundaryEvent(state.last, sample, ChangeSegment, EventAccepted))
			ledger.Cycles[state.cycleIndex].EndedAt = sample.FetchedAt
			ledger.Cycles[state.cycleIndex].EndedBy = ChangeSegment
			state.cycleIndex = ledger.startCycle(sample, true, false)
			state.last, state.dropHigh, state.pending, state.lowStreak = sample, *sample.UsedBasisPoints, nil, 0
			continue
		}
		if state.pending != nil {
			pending := state.pending
			// 两种额度窗口在0%空闲时都可能返回“采样时刻+完整窗口”的滑动截止。
			// 只有完整空闲窗口的连续证据才能延续确认，不能泛化为任意周计划变化。
			idleContinuation := fullWindowIdle(pending.first) && followsIdleWindow(state.last, sample)
			if (!sameResetPlan(sample.ResetAt, pending.first.ResetAt) && !idleContinuation) ||
				*pending.first.UsedBasisPoints-*sample.UsedBasisPoints >= significantDrop {
				pending.ambiguous = true
			}
			if pending.canReset && !pending.ambiguous {
				ledger.acceptReset(state, pending.first, ledger.Events[pending.eventIndex].Kind)
				ledger.Events[pending.eventIndex].Status = EventAccepted
				ledger.Events[pending.eventIndex].ConfirmedAt = sample.FetchedAt
				ledger.advanceCycle(state.cycleIndex, sample)
				state.dropHigh = max(*pending.first.UsedBasisPoints, *sample.UsedBasisPoints)
				state.pending = nil
			}
			state.last = sample
			continue
		}
		// 空闲截止后移不是领取新额度；开始使用后截止固定，也仍属于同一轮。
		// 显式人工决定优先，不能因规则修正而丢失已有决定链。
		_, hasDecision := effective[boundaryEvent(state.last, sample, ChangePlan, EventPending).Key]
		if !hasGap && !interrupted && !hasDecision && followsIdleWindow(state.last, sample) {
			ledger.advanceCycle(state.cycleIndex, sample)
			state.last, state.dropHigh, state.lowStreak = sample, max(state.dropHigh, *sample.UsedBasisPoints), 0
			continue
		}
		if *sample.UsedBasisPoints < state.dropHigh {
			state.lowStreak++
		} else {
			state.lowStreak = 0
		}
		planChanged := !sameResetPlan(sample.ResetAt, state.last.ResetAt)
		dropped := state.dropHigh-*sample.UsedBasisPoints >= significantDrop
		if !planChanged && !dropped && state.lowStreak < 2 {
			ledger.advanceCycle(state.cycleIndex, sample)
			state.dropHigh = max(state.dropHigh, *sample.UsedBasisPoints)
			state.last = sample
			continue
		}
		kind := ChangeUsage
		if planChanged {
			kind = ChangePlan
		}
		// 只有向后延展的新计划且有回落证据，才自动候选重置；时间前移也可能只是校正。
		zeroExpired := state.dropHigh == 0 && *sample.UsedBasisPoints == 0 && sample.FetchedAt >= state.last.ResetAt
		// 周额度低用量也可能真实重置：到期后的任意回落，或提前归零，均进入双样本确认。
		// 非零的提前微小回落仍可能是校正，不扩大原有自动确认范围。
		weeklyLowReset := sample.WindowSeconds == SevenDaySeconds && *sample.UsedBasisPoints < state.dropHigh &&
			(sample.FetchedAt >= state.last.ResetAt || *sample.UsedBasisPoints == 0)
		canReset := sample.ResetAt > state.last.ResetAt+resetTolerance && (dropped || zeroExpired || weeklyLowReset)
		if canReset {
			switch {
			case sample.FetchedAt < state.last.ResetAt-resetTolerance:
				kind = ResetEarly
			case sample.FetchedAt >= state.last.ResetAt:
				kind = ResetExpired
			default:
				canReset = false
			}
		}
		event := boundaryEvent(state.last, sample, kind, EventPending)
		ledger.Events = append(ledger.Events, event)
		index := len(ledger.Events) - 1
		state.pending = &pendingChange{first: sample, eventIndex: index, canReset: canReset, ambiguous: hasGap || interrupted}
		if decision, ok := effective[event.Key]; ok {
			matched[event.Key] = true
			ledger.Events[index].DecisionID = decision.ID
			ledger.Events[index].ConfirmedAt = sample.FetchedAt
			switch decision.Kind {
			case ConfirmReset:
				ledger.acceptReset(state, sample, ResetManual)
				ledger.Events[index].Kind, ledger.Events[index].Status = ResetManual, EventAccepted
				ledger.Cycles[state.cycleIndex].HasGap = hasGap
			case PlanCorrection:
				ledger.Events[index].Status = EventCorrected
				ledger.Cycles[state.cycleIndex].HasCorrection = true
				ledger.advanceCycle(state.cycleIndex, sample)
			case SegmentChange:
				ledger.Events[index].Kind, ledger.Events[index].Status = ChangeSegment, EventAccepted
				ledger.Cycles[state.cycleIndex].EndedAt, ledger.Cycles[state.cycleIndex].EndedBy = sample.FetchedAt, ChangeSegment
				state.cycleIndex = ledger.startCycle(sample, true, false)
			}
			state.pending, state.dropHigh, state.lowStreak = nil, *sample.UsedBasisPoints, 0
		}
		state.last = sample
	}
	for key := range effective {
		if !matched[key] {
			return Ledger{}, fmt.Errorf("quota decision does not match an active boundary")
		}
	}
	return ledger, nil
}

func (sample Sample) invalidReason() string {
	switch {
	case sample.ID <= 0 || sample.SubjectID <= 0 || sample.BatchID == "" || sample.SegmentID == "" || sample.FetchedAt <= 0:
		return "invalid_identity_or_timestamp"
	case sample.WindowSeconds != FiveHourSeconds && sample.WindowSeconds != SevenDaySeconds:
		return "unknown_window"
	case sample.UsedBasisPoints == nil:
		return "missing_percent"
	case *sample.UsedBasisPoints < 0 || *sample.UsedBasisPoints > 10000:
		return "invalid_percent"
	case sample.ResetAt <= sample.FetchedAt || sample.ResetAt-sample.FetchedAt > sample.WindowSeconds+resetTolerance:
		return "invalid_reset_time"
	case sample.IntervalSeconds != 0 && (sample.IntervalSeconds < 120 || sample.IntervalSeconds > 86400):
		return "invalid_interval"
	default:
		return ""
	}
}

func sameObservation(a, b Sample) bool {
	if (a.UsedBasisPoints == nil) != (b.UsedBasisPoints == nil) {
		return false
	}
	if a.UsedBasisPoints != nil && *a.UsedBasisPoints != *b.UsedBasisPoints {
		return false
	}
	a.UsedBasisPoints, b.UsedBasisPoints = nil, nil
	return a == b
}

func sameResetPlan(a, b int64) bool {
	return a >= b-resetTolerance && a <= b+resetTolerance
}

// fullWindowIdle仅识别0%且截止等于采样时刻加完整窗口的记录，不推断非零或短窗口校正。
func fullWindowIdle(sample Sample) bool {
	return (sample.WindowSeconds == FiveHourSeconds || sample.WindowSeconds == SevenDaySeconds) &&
		sample.UsedBasisPoints != nil && *sample.UsedBasisPoints == 0 &&
		sameResetPlan(sample.ResetAt, sample.FetchedAt+sample.WindowSeconds)
}

func followsIdleWindow(before, after Sample) bool {
	return fullWindowIdle(before) && after.WindowSeconds == before.WindowSeconds &&
		after.FetchedAt < before.ResetAt-resetTolerance && after.ResetAt >= before.ResetAt-resetTolerance
}

func boundaryEvent(before, after Sample, kind EventKind, status EventStatus) Event {
	return Event{Key: fmt.Sprintf("%d/%d/%d", after.WindowSeconds, before.ID, after.ID),
		WindowSeconds: after.WindowSeconds, Kind: kind, Status: status,
		BeforeSnapshotID: before.ID, AfterSnapshotID: after.ID,
		ObservedFrom: before.FetchedAt, ObservedTo: after.FetchedAt,
		OldResetAt: before.ResetAt, NewResetAt: after.ResetAt}
}

func (ledger *Ledger) startCycle(sample Sample, partial, countInitial bool) int {
	ledger.Cycles = append(ledger.Cycles, Cycle{WindowSeconds: sample.WindowSeconds, SegmentID: sample.SegmentID,
		StartedAt: sample.FetchedAt, ExpectedResetAt: sample.ResetAt, Partial: partial})
	index := len(ledger.Cycles) - 1
	if countInitial {
		ledger.advanceCycle(index, sample)
	} else {
		ledger.Cycles[index].HighWaterBasisPoints = *sample.UsedBasisPoints
	}
	return index
}

func (ledger *Ledger) advanceCycle(index int, sample Sample) {
	cycle := &ledger.Cycles[index]
	if increase := *sample.UsedBasisPoints - cycle.HighWaterBasisPoints; increase > 0 {
		cycle.ObservedBasisPoints += int64(increase)
		cycle.HighWaterBasisPoints = *sample.UsedBasisPoints
		ledger.Deltas = append(ledger.Deltas, Delta{sample.WindowSeconds, sample.FetchedAt, int64(increase)})
	}
	cycle.ExpectedResetAt = sample.ResetAt
}

// acceptReset只结算观测值；旧轮最后一次观测之后的消耗不可由百分比推算补齐。
func (ledger *Ledger) acceptReset(state *windowState, first Sample, kind EventKind) {
	ledger.Cycles[state.cycleIndex].EndedAt = first.FetchedAt
	ledger.Cycles[state.cycleIndex].EndedBy = kind
	state.cycleIndex = ledger.startCycle(first, false, true)
	state.lowStreak = 0
}

// resolveDecisions要求每个事件是一条无分叉替代链，不能按数组顺序偷偷覆盖决定。
func resolveDecisions(decisions []Decision) (map[string]Decision, error) {
	ordered := slices.Clone(decisions)
	slices.SortFunc(ordered, func(a, b Decision) int { return cmp.Compare(a.ID, b.ID) })
	effective := make(map[string]Decision)
	ids := make(map[int64]bool)
	for _, decision := range ordered {
		if decision.ID <= 0 || ids[decision.ID] || decision.EventKey == "" || strings.TrimSpace(decision.Reason) == "" {
			return nil, fmt.Errorf("invalid quota decision identity or reason")
		}
		switch decision.Kind {
		case ConfirmReset, PlanCorrection, SegmentChange:
		default:
			return nil, fmt.Errorf("unknown quota decision kind")
		}
		previous, exists := effective[decision.EventKey]
		if (!exists && decision.SupersedesID != 0) || (exists && previous.ID != decision.SupersedesID) {
			return nil, fmt.Errorf("quota decisions must form a single replacement chain")
		}
		effective[decision.EventKey], ids[decision.ID] = decision, true
	}
	return effective, nil
}
