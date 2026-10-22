export type Settings = {
  enabled: boolean
  interval_minutes: number
  version: number
  updated_at: number
}
export type Subject = {
  id: number
  name: string
  source_channel_id: number
  enabled: boolean
  version: number
  next_due_at: number
  last_attempt_at: number
  last_success_at: number
  error_code: string
}
export type Channel = { id: number; name: string; type: number; bound: boolean }
export type Page<T> = { items: T[]; total: number; ledger_revision?: number }
export type Cycle = {
  window_seconds: number
  started_at: number
  ended_at: number
  ended_by: string
  expected_reset_at: number
  high_water_basis_points: number
  observed_basis_points: number
  partial: boolean
  has_gap: boolean
  has_correction: boolean
}
export type WindowSummary = {
  gap_seconds: number
  invalid_samples: number
  observed_units: number
  unused_units: number
  reset_count: number
  early_resets: number
  expired_resets: number
  manual_resets: number
  pending_events: number
  pending_since: number
  partial_cycles: number
  excluded_cycles: number
  evaluated_cycles: number
  current: Cycle | null
}
export type MonthSummary = {
  month: string
  timezone: string
  coverage_start: number
  rules_version: number
  seven_day: WindowSummary
  five_hour: WindowSummary
  gap_seconds: number
  invalid_samples: number
}
export type Current = {
  used_basis_points: number | null
  reset_at: number
  fetched_at: number
  quality_code: string
  stale: boolean
}
export type Summary = {
  collection_enabled: boolean
  subject: Subject
  ledger_revision: number
  summary: MonthSummary
  current: Record<string, Current>
  last_observed_at: number
}
export type Trend = {
  date: string
  observed_units: number | null
  has_gap: boolean
}
export type Decision = 'confirm_reset' | 'plan_correction' | 'segment_change'
export type QuotaEvent = {
  id: number
  kind: string
  status: string
  version: number
  observed_from: number
  observed_to: number
  old_reset_at: number
  new_reset_at: number
  previews: Partial<Record<Decision, MonthSummary>>
  decisions: {
    id: number
    decision: Decision
    reason: string
    actor_id: number
    created_at: number
    supersedes_id: number
  }[]
}
export type Filters = {
  subject_id: number
  month: string
  window_seconds: number
}
export type View = 'overview' | 'history'
