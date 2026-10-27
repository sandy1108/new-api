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

import { api } from '@/lib/api'

import type {
  UsageSummaryData,
  UsageSummaryEnvelope,
  UsageSummaryItem,
  UsageSummaryRange,
  UsageSummaryScope,
  UsageSummaryTrendPoint,
} from './types'

export interface GetUsageSummaryParams {
  scope: UsageSummaryScope
  range: UsageSummaryRange
  observer?: boolean
  signal?: AbortSignal
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === 'object'
}

function isFiniteNumber(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value)
}

function isUsageSummaryItem(value: unknown): value is UsageSummaryItem {
  if (!isRecord(value)) return false

  const requiredNumberFields = [
    'user_id',
    'token_id',
    'channel_id',
    'requests',
    'input_tokens',
    'output_tokens',
    'total_tokens',
    'quota',
  ]
  if (!requiredNumberFields.every((field) => isFiniteNumber(value[field]))) {
    return false
  }

  const requiredStringFields = ['username', 'token_name', 'model_name']
  if (
    !requiredStringFields.every((field) => typeof value[field] === 'string')
  ) {
    return false
  }

  return (
    value.channel_name === undefined || typeof value.channel_name === 'string'
  )
}

function isUsageSummaryData(value: unknown): value is UsageSummaryData {
  if (!isRecord(value) || !Array.isArray(value.items)) return false

  const totalFields = [
    'total_requests',
    'total_input_tokens',
    'total_output_tokens',
    'total_tokens',
    'total_quota',
  ]
  const trend = value.trend
  const validTrend =
    trend === undefined ||
    (Array.isArray(trend) && trend.every(isUsageSummaryTrendPoint))

  return (
    totalFields.every((field) => isFiniteNumber(value[field])) &&
    value.items.every(isUsageSummaryItem) &&
    validTrend
  )
}

function isUsageSummaryTrendPoint(
  value: unknown
): value is UsageSummaryTrendPoint {
  if (!isRecord(value)) return false
  return [
    'timestamp',
    'requests',
    'input_tokens',
    'output_tokens',
    'total_tokens',
    'quota',
  ].every((field) => isFiniteNumber(value[field]))
}

function parseUsageSummaryResponse(value: unknown): UsageSummaryData {
  if (!isRecord(value)) throw new Error('Invalid usage summary response')

  const envelope = value as unknown as UsageSummaryEnvelope
  if (envelope.success !== true) {
    throw new Error(
      typeof envelope.message === 'string' && envelope.message
        ? envelope.message
        : 'Usage summary request failed'
    )
  }

  if (!isUsageSummaryData(envelope.data)) {
    throw new Error('Invalid usage summary response')
  }
  return envelope.data
}

export async function getUsageSummary(
  params: GetUsageSummaryParams
): Promise<UsageSummaryData> {
  let endpoint = '/api/log/self/usage-summary'
  if (params.observer) endpoint = '/api/pong-reports/usage-summary'
  else if (params.scope === 'all') endpoint = '/api/log/usage-summary'
  const response = await api.get<UsageSummaryEnvelope>(endpoint, {
    signal: params.signal,
    params: {
      ...(params.observer ? { scope: params.scope } : {}),
      start_timestamp: params.range.startTimestamp,
      end_timestamp: params.range.endTimestamp,
      include_trend: true,
    },
  })
  return parseUsageSummaryResponse(response.data)
}
