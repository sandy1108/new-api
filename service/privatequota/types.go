// Package privatequota 只处理订阅额度观测，不参与模型Token计费或上游额度兑换。
package privatequota

const (
	FiveHourSeconds int64 = 18000
	SevenDaySeconds int64 = 604800
	RulesVersion          = 1
)

// Sample 是单一主体的不可变观测；百分比缺失必须保留nil，不能冒充0%。
// IntervalSeconds保存采样当时的间隔，避免修改频率后重算历史缺口失真。
type Sample struct {
	ID              int64
	SubjectID       int64
	BatchID         string
	WindowSeconds   int64
	SegmentID       string
	FetchedAt       int64
	UsedBasisPoints *int
	ResetAt         int64
	IntervalSeconds int64
}

type EventKind string
type EventStatus string
type DecisionKind string

const (
	ChangePlan    EventKind = "plan_change"
	ChangeUsage   EventKind = "usage_correction"
	ChangeSegment EventKind = "segment_change"
	ResetEarly    EventKind = "early_reset"
	ResetExpired  EventKind = "expired_reset"
	ResetManual   EventKind = "manual_reset"

	EventPending      EventStatus = "pending"
	EventAccepted     EventStatus = "accepted"
	EventCorrected    EventStatus = "corrected"
	EventInconclusive EventStatus = "inconclusive"

	ConfirmReset   DecisionKind = "confirm_reset"
	PlanCorrection DecisionKind = "plan_correction"
	SegmentChange  DecisionKind = "segment_change"
)

// Decision只改变本地判定，替代关系保留历史；操作者授权由后续服务端接口负责。
type Decision struct {
	ID           int64
	EventKey     string
	Kind         DecisionKind
	Reason       string
	SupersedesID int64
}

// Event保存两侧证据的观测区间，而不是编造一个精确重置时刻。
type Event struct {
	Key              string
	WindowSeconds    int64
	Kind             EventKind
	Status           EventStatus
	BeforeSnapshotID int64
	AfterSnapshotID  int64
	ObservedFrom     int64
	ObservedTo       int64
	ConfirmedAt      int64
	OldResetAt       int64
	NewResetAt       int64
	DecisionID       int64
}

// Cycle的ObservedBasisPoints是观测增量，HighWaterBasisPoints包括接入前基线。
type Cycle struct {
	WindowSeconds        int64
	SegmentID            string
	StartedAt            int64
	EndedAt              int64
	EndedBy              EventKind
	ExpectedResetAt      int64
	HighWaterBasisPoints int
	ObservedBasisPoints  int64
	Partial              bool
	HasGap               bool
	HasCorrection        bool
}

type Delta struct {
	WindowSeconds int64
	FetchedAt     int64
	BasisPoints   int64
}

type ObservationGap struct {
	WindowSeconds int64
	From          int64
	To            int64
}

type RejectedSample struct {
	SampleID  int64
	FetchedAt int64
	Reason    string
}

// Ledger是可重建结果，没有数据库、系统时钟或网络副作用；只接受一个主体。
type Ledger struct {
	SubjectID      int64
	RulesVersion   int
	CoverageStart  int64
	LastObservedAt int64
	Cycles         []Cycle
	Events         []Event
	Deltas         []Delta
	Gaps           []ObservationGap
	Rejected       []RejectedSample
}

type WindowSummary struct {
	ObservedBasisPoints int64
	UnusedBasisPoints   int64
	ResetCount          int
	EarlyResets         int
	ExpiredResets       int
	ManualResets        int
	PendingEvents       int
	PendingSince        int64
	PartialCycles       int
	ExcludedCycles      int
	EvaluatedCycles     int
	Current             *Cycle
}

// MonthSummary分开两种窗口单位；缺口按时间并集而非窗口数累计。
type MonthSummary struct {
	Month          string
	Timezone       string
	CoverageStart  int64
	RulesVersion   int
	SevenDay       WindowSummary
	FiveHour       WindowSummary
	GapSeconds     int64
	InvalidSamples int
}
