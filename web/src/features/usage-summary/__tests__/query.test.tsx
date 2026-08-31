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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { createElement, type ReactNode } from 'react'

import { useAuthStore } from '@/stores/auth-store'

import { getUsageSummary } from '../api'
import { useUsageSummary } from '../hooks/use-usage-summary'
import type { UsageSummaryData, UsageSummaryRange } from '../types'

vi.mock('../api', () => ({
  getUsageSummary: vi.fn(),
}))

const mockedGetUsageSummary = vi.mocked(getUsageSummary)
const range: UsageSummaryRange = {
  id: 'today',
  startTimestamp: 100,
  endTimestamp: 200,
  cacheKey: 'today:100',
}
const data: UsageSummaryData = {
  total_requests: 0,
  total_input_tokens: 0,
  total_output_tokens: 0,
  total_tokens: 0,
  total_quota: 0,
  items: [],
}

function createWrapper(queryClient: QueryClient) {
  return function Wrapper(props: { children: ReactNode }) {
    return createElement(
      QueryClientProvider,
      { client: queryClient },
      props.children
    )
  }
}

afterEach(() => {
  mockedGetUsageSummary.mockReset()
  useAuthStore.getState().auth.reset()
})

describe('useUsageSummary', () => {
  test('uses the administrator all scope when requested', async () => {
    useAuthStore.getState().auth.setUser({ id: 1, username: 'admin', role: 10 })
    mockedGetUsageSummary.mockResolvedValue(data)
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })

    const result = renderHook(() => useUsageSummary(range, 'all'), {
      wrapper: createWrapper(queryClient),
    })

    await waitFor(() => expect(result.result.current.isSuccess).toBe(true))
    expect(result.result.current.scope).toBe('all')
    expect(result.result.current.canManageScope).toBe(true)
    expect(mockedGetUsageSummary).toHaveBeenCalledWith({ scope: 'all', range })
    expect(queryClient.getQueryCache().findAll()).toHaveLength(1)
  })

  test('forces a non-administrator request to the self scope', async () => {
    useAuthStore.getState().auth.setUser({ id: 2, username: 'user', role: 1 })
    mockedGetUsageSummary.mockResolvedValue(data)
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })

    const result = renderHook(() => useUsageSummary(range, 'all'), {
      wrapper: createWrapper(queryClient),
    })

    await waitFor(() => expect(result.result.current.isSuccess).toBe(true))
    expect(result.result.current.scope).toBe('self')
    expect(result.result.current.canManageScope).toBe(false)
    expect(mockedGetUsageSummary).toHaveBeenCalledWith({ scope: 'self', range })
  })
})
