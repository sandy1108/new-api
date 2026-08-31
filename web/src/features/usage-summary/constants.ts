/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.
*/
import type { UsageSummaryRangeId, UsageSummaryScope } from './types'

export const USAGE_SUMMARY_STALE_TIME = 10 * 60 * 1000

export const DEFAULT_USAGE_SUMMARY_RANGE: UsageSummaryRangeId = 'today'

export const DEFAULT_USAGE_SUMMARY_SCOPE: UsageSummaryScope = 'all'

export const USAGE_SUMMARY_RANGES = [
  { id: 'today', labelKey: 'Today' },
  { id: 'yesterday', labelKey: 'Yesterday' },
  { id: 'week', labelKey: 'This Week' },
  { id: 'last-week', labelKey: 'Last Week' },
  { id: 'month', labelKey: 'This Month' },
  { id: 'last-month', labelKey: 'Last Month' },
  { id: 'quarter', labelKey: 'This Quarter' },
] as const satisfies ReadonlyArray<{
  id: UsageSummaryRangeId
  labelKey: string
}>

export type UsageSummaryRangeOption = (typeof USAGE_SUMMARY_RANGES)[number]
