package privatequota

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 小于2个百分点的真实到期重置不能让周账本永久停在pending。
func TestBuildLedgerWeeklyLowUsageResetContinuesAccounting(t *testing.T) {
	const at int64 = 1800000000
	for _, tc := range []struct {
		name       string
		oldReset   int64
		newUsage   int
		wantStatus EventStatus
	}{
		{"到期后1%归零", at + 300, 0, EventAccepted},
		{"提前重置1%归零", at + 3600, 0, EventAccepted},
		{"到期后微小非零回落", at + 300, 50, EventAccepted},
		{"提前微小非零回落仍待人工", at + 3600, 50, EventPending},
	} {
		t.Run(tc.name, func(t *testing.T) {
			samples := []Sample{
				quotaSample(1, at, 100, tc.oldReset),
				quotaSample(2, at+600, tc.newUsage, at+605400),
				quotaSample(3, at+1200, 100, at+605400),
				quotaSample(4, at+1800, 300, at+605400),
			}
			pending, err := BuildLedger(samples[:2], nil)
			require.NoError(t, err)
			require.Len(t, pending.Events, 1)
			assert.Equal(t, EventPending, pending.Events[0].Status)
			ledger, err := BuildLedger(samples, nil)
			require.NoError(t, err)
			require.Len(t, ledger.Events, 1)
			assert.Equal(t, tc.wantStatus, ledger.Events[0].Status)
			if tc.wantStatus == EventPending {
				require.Len(t, ledger.Cycles, 1)
				assert.Zero(t, ledger.Cycles[0].ObservedBasisPoints)
				return
			}
			require.Len(t, ledger.Cycles, 2)
			assert.EqualValues(t, 300, ledger.Cycles[1].ObservedBasisPoints)
		})
	}
}

func TestSummarizeMonthWeeklyGrowthIsLiveAndResetDoesNotAddAUnit(t *testing.T) {
	const at int64 = 1790812800 // 北京时间2026-10-01 08:00。
	samples := []Sample{
		quotaSample(1, at, 0, at+3600),
		quotaSample(2, at+600, 3000, at+3600),
		quotaSample(3, at+1200, 8000, at+3600),
		quotaSample(4, at+1800, 1000, at+606000),
		quotaSample(5, at+2400, 1000, at+606000),
	}
	location := time.FixedZone("Asia/Shanghai", 8*3600)
	for _, tc := range []struct {
		count  int
		want   int64
		resets int
	}{
		{2, 3000, 0}, {3, 8000, 0}, {4, 8000, 0}, {5, 9000, 1},
	} {
		ledger, err := BuildLedger(samples[:tc.count], nil)
		require.NoError(t, err)
		summary, err := SummarizeMonth(ledger, "2026-10", location)
		require.NoError(t, err)
		assert.Equal(t, tc.want, summary.SevenDay.ObservedBasisPoints)
		assert.Equal(t, tc.resets, summary.SevenDay.ResetCount)
	}
}

func TestSummarizeMonthFiveHourQualityDoesNotPolluteWeeklyLedger(t *testing.T) {
	const at int64 = 1790812800
	bad := idleSample(5, at+4200, 0, at+18000)
	bad.UsedBasisPoints = nil
	ledger, err := BuildLedger([]Sample{
		quotaSample(1, at, 0, at+604800),
		quotaSample(2, at+600, 100, at+604800),
		idleSample(3, at, 0, at+18000),
		idleSample(4, at+3600, 0, at+18000), bad,
	}, nil)
	require.NoError(t, err)
	summary, err := SummarizeMonth(ledger, "2026-10", time.FixedZone("Asia/Shanghai", 8*3600))
	require.NoError(t, err)
	assert.EqualValues(t, 100, summary.SevenDay.ObservedBasisPoints)
	assert.Zero(t, summary.SevenDay.GapSeconds)
	assert.Zero(t, summary.SevenDay.InvalidSamples)
	assert.EqualValues(t, 3600, summary.FiveHour.GapSeconds)
	assert.Equal(t, 1, summary.FiveHour.InvalidSamples)
	assert.EqualValues(t, 3600, summary.GapSeconds)
	assert.Equal(t, 1, summary.InvalidSamples)
}

// 此夹具只含时间/百分比，不包含真实渠道或OAuth；复现空闲时reset_at随查询滑动。
func idleSample(id, at int64, used int, reset int64) Sample {
	return Sample{ID: id, SubjectID: 1, BatchID: string(rune('a' + id)), SegmentID: "idle-test",
		FetchedAt: at, WindowSeconds: FiveHourSeconds, UsedBasisPoints: &used, ResetAt: reset, IntervalSeconds: 600}
}

func TestBuildLedgerFiveHourIdleResetResumesUsage(t *testing.T) {
	const at int64 = 1800000000
	samples := []Sample{idleSample(1, at, 8100, at+900),
		idleSample(2, at+1200, 0, at+19200),
		idleSample(3, at+1800, 0, at+19800),
		idleSample(4, at+2400, 0, at+20400),
		// 开始使用时先有不足1%的0读数，reset_at从滑动截止变为固定截止。
		idleSample(5, at+3000, 0, at+20800),
		idleSample(6, at+3600, 100, at+20800),
		idleSample(7, at+4200, 300, at+20800)}
	ledger, err := BuildLedger(samples, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(ledger.Events) != 1 || ledger.Events[0].Status != EventAccepted || ledger.Events[0].Kind != ResetExpired {
		t.Fatalf("want one confirmed expired reset, got %+v", ledger.Events)
	}
	if len(ledger.Cycles) != 2 || ledger.Cycles[1].ObservedBasisPoints != 300 {
		t.Fatalf("want new cycle to accumulate 3%%, got %+v", ledger.Cycles)
	}
	// 旧事件键保持可重放；人工校正不能因新算法识别重置而丢失。
	corrected, err := BuildLedger(samples, []Decision{{ID: 1, EventKey: ledger.Events[0].Key, Kind: PlanCorrection, Reason: "人工复核"}})
	if err != nil || len(corrected.Events) == 0 || corrected.Events[0].Status != EventCorrected {
		t.Fatalf("manual decision lost: %v %+v", err, corrected.Events)
	}
}

func TestBuildLedgerFiveHourIdleDoesNotInventResets(t *testing.T) {
	const at int64 = 1800000000
	var samples []Sample
	for i := int64(0); i < 40; i++ {
		samples = append(samples, idleSample(i+1, at+i*600, 0, at+i*600+18000))
	}
	samples = append(samples, idleSample(41, at+24000, 300, at+41800))
	ledger, err := BuildLedger(samples, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(ledger.Events) != 0 || len(ledger.Cycles) != 1 || ledger.Cycles[0].ObservedBasisPoints != 300 {
		t.Fatalf("idle sliding must not create a reset or block usage: %+v", ledger)
	}
}

func TestBuildLedgerIdleExceptionKeepsAmbiguousEvidencePending(t *testing.T) {
	const at int64 = 1800000000
	for _, name := range []string{"weekly", "nonzero", "not_full_window", "backward_plan", "gap_after", "gap_before", "invalid_between"} {
		t.Run(name, func(t *testing.T) {
			samples := []Sample{idleSample(1, at, 8100, at+900), idleSample(2, at+1200, 0, at+19200), idleSample(3, at+1800, 0, at+19800)}
			switch name {
			case "weekly":
				for i := range samples {
					samples[i].WindowSeconds = SevenDaySeconds
				}
			case "nonzero":
				*samples[1].UsedBasisPoints, *samples[2].UsedBasisPoints = 100, 100
			case "not_full_window":
				samples[1].ResetAt, samples[2].ResetAt = at+18000, at+18600
			case "backward_plan":
				samples[2].ResetAt = at + 18000
			case "gap_after":
				samples[2].FetchedAt, samples[2].ResetAt = at+3600, at+21600
			case "gap_before":
				samples[1].FetchedAt, samples[1].ResetAt = at+3600, at+21600
				samples[2].FetchedAt, samples[2].ResetAt = at+4200, at+22200
			case "invalid_between":
				bad := idleSample(4, at+1500, 0, at+19500)
				bad.UsedBasisPoints = nil
				samples = append(samples, bad)
			}
			ledger, err := BuildLedger(samples, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(ledger.Events) != 1 || ledger.Events[0].Status != EventPending || len(ledger.Cycles) != 1 {
				t.Fatalf("must not turn ambiguous evidence into a reset: %+v", ledger)
			}
		})
	}
}

func TestBuildLedgerIdleToActiveDirectlyAccumulates(t *testing.T) {
	const at int64 = 1800000000
	ledger, err := BuildLedger([]Sample{idleSample(1, at, 0, at+18000), idleSample(2, at+600, 100, at+18400), idleSample(3, at+1200, 300, at+18400)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(ledger.Events) != 0 || ledger.Cycles[0].ObservedBasisPoints != 300 {
		t.Fatalf("active usage after idle lost: %+v", ledger)
	}
}

func TestBuildLedgerIdlePlanDecisionSurvivesReplay(t *testing.T) {
	const at int64 = 1800000000
	samples := []Sample{idleSample(1, at, 0, at+18000), idleSample(2, at+600, 0, at+18600), idleSample(3, at+1200, 0, at+19200)}
	ledger, err := BuildLedger(samples, []Decision{{ID: 1, EventKey: "18000/1/2", Kind: PlanCorrection, Reason: "保留旧版人工校正"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(ledger.Events) != 1 || ledger.Events[0].Status != EventCorrected {
		t.Fatalf("explicit decision discarded: %+v", ledger)
	}
}
