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

export type UsageSummaryScope = 'all' | 'self'

export type UsageSummaryRangeId =
  | 'today'
  | 'yesterday'
  | 'week'
  | 'last-week'
  | 'month'
  | 'last-month'
  | 'quarter'

export interface UsageSummaryItem {
  user_id: number
  username: string
  token_id: number
  token_name: string
  channel_id: number
  channel_name?: string
  model_name: string
  requests: number
  input_tokens: number
  output_tokens: number
  total_tokens: number
  quota: number
}

export interface UsageSummaryTrendPoint {
  timestamp: number
  requests: number
  input_tokens: number
  output_tokens: number
  total_tokens: number
  quota: number
}

export interface UsageSummaryData {
  total_requests: number
  total_input_tokens: number
  total_output_tokens: number
  total_tokens: number
  total_quota: number
  items: UsageSummaryItem[]
  trend?: UsageSummaryTrendPoint[]
}

export interface UsageSummaryEnvelope {
  success: boolean
  message?: string
  data?: UsageSummaryData
}

export interface UsageSummaryRange {
  id: UsageSummaryRangeId
  startTimestamp: number
  endTimestamp: number
  cacheKey: string
}

export interface UsageMetrics {
  requests: number
  inputTokens: number
  outputTokens: number
  totalTokens: number
  quota: number
}

export interface UsageModelBucket extends UsageMetrics {
  name: string
}

export interface UsageChannelBucket extends UsageMetrics {
  id: number
  name: string
  key: string
  models: UsageModelBucket[]
}

export interface UsageTokenBucket extends UsageMetrics {
  id: number
  name: string
  key: string
  userId: number
  username: string
  channels: UsageChannelBucket[]
}

export interface UsageSummaryViewModel {
  totals: UsageMetrics
  tokens: UsageTokenBucket[]
}
