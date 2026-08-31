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
import { afterEach, describe, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { getUsageSummary } from '../api'
import type {
  UsageSummaryData,
  UsageSummaryRange,
  UsageSummaryEnvelope,
} from '../types'

const range: UsageSummaryRange = {
  id: 'today',
  startTimestamp: 100,
  endTimestamp: 200,
  cacheKey: 'today:100',
}

const data: UsageSummaryData = {
  total_requests: 1,
  total_input_tokens: 10,
  total_output_tokens: 5,
  total_tokens: 15,
  total_quota: 100,
  items: [],
}

const successResponse: UsageSummaryEnvelope = {
  success: true,
  data,
}

afterEach(() => {
  vi.restoreAllMocks()
})

describe('usage summary API', () => {
  test('requests the administrator endpoint for the all scope', async () => {
    const get = vi.spyOn(api, 'get').mockResolvedValue({
      data: successResponse,
    } as never)

    await expect(getUsageSummary({ scope: 'all', range })).resolves.toEqual(
      data
    )

    expect(get).toHaveBeenCalledOnce()
    expect(get).toHaveBeenCalledWith('/api/log/usage-summary', {
      params: {
        start_timestamp: 100,
        end_timestamp: 200,
        include_trend: true,
      },
    })
  })

  test('requests the self endpoint for the self scope', async () => {
    const get = vi.spyOn(api, 'get').mockResolvedValue({
      data: successResponse,
    } as never)

    await getUsageSummary({ scope: 'self', range })

    expect(get).toHaveBeenCalledWith('/api/log/self/usage-summary', {
      params: {
        start_timestamp: 100,
        end_timestamp: 200,
        include_trend: true,
      },
    })
  })

  test('rejects a successful HTTP response with an invalid payload', async () => {
    vi.spyOn(api, 'get').mockResolvedValue({
      data: { success: true, data: { items: null } },
    } as never)

    await expect(getUsageSummary({ scope: 'self', range })).rejects.toThrow(
      'Invalid usage summary response'
    )
  })

  test('rejects an unsuccessful business response without falling back to logs', async () => {
    const get = vi.spyOn(api, 'get').mockResolvedValue({
      data: { success: false, message: 'not supported' },
    } as never)

    await expect(getUsageSummary({ scope: 'self', range })).rejects.toThrow(
      'not supported'
    )
    expect(get).toHaveBeenCalledOnce()
  })
})
