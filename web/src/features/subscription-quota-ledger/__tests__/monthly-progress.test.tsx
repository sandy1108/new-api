import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, cleanup, render, screen } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { QuotaHistory } from '../components/history'
import { QuotaMetrics } from '../components/quota-metrics'
import { monthlyQuotaProgress } from '../lib/forms'
import type { Summary } from '../types'

afterEach(() => {
  cleanup()
  vi.useRealTimers()
  vi.restoreAllMocks()
})

test.each([
  [0, 0, 1, 0, 0],
  [1.46, 36.5, 1, 36.5, 0],
  [4, 100, 1, 100, 1],
  [5, 125, 2, 25, 1],
  [8, 200, 2, 100, 2],
  [10, 250, 3, 50, 2],
])(
  '累计%s份时显示完整比例且整段边界不清空',
  (units, percent, segment, progress, completed) => {
    expect(monthlyQuotaProgress(units)).toEqual({
      percent,
      segment,
      progress,
      completed,
    })
  }
)

test.each([Number.NaN, Infinity, -1])('无效累计值%s不伪装成零用量', (units) => {
  expect(monthlyQuotaProgress(units)).toBeNull()
})

test.each([
  [4, '100%', '100'],
  [8, '200%', '100'],
  [10, '250%', '50'],
])('累计%s份时界面显示%s且进度条不清空', (units, total, progress) => {
  render(
    <QuotaMetrics
      data={
        {
          summary: {
            seven_day: { observed_units: units, evaluated_cycles: 0 },
          },
          current: {},
        } as unknown as Summary
      }
    />
  )
  expect(screen.getByText(total)).toBeInTheDocument()
  expect(screen.getByRole('progressbar')).toHaveAttribute(
    'aria-valuenow',
    progress
  )
})

test('历史页停留一分钟后自动获取新轮次与事件，不需要手动刷新', async () => {
  vi.useFakeTimers()
  let updated = false
  vi.spyOn(api, 'get').mockImplementation(async (url) => ({
    data: {
      success: true,
      data: {
        items:
          url.endsWith('/cycles') && updated
            ? [
                {
                  window_seconds: 604800,
                  started_at: 1,
                  ended_at: 0,
                  observed_basis_points: 600,
                  high_water_basis_points: 600,
                  partial: false,
                  has_gap: false,
                  has_correction: false,
                  ended_by: '',
                },
              ]
            : [],
        total: updated ? 1 : 0,
      },
    },
  }))
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  try {
    render(
      <QueryClientProvider client={client}>
        <QuotaHistory
          filters={{ subject_id: 1, month: '2026-10', window_seconds: 604800 }}
        />
      </QueryClientProvider>
    )
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1)
    })
    expect(
      screen.queryByText('Current cycle is still in progress')
    ).not.toBeInTheDocument()
    updated = true
    await act(async () => {
      await vi.advanceTimersByTimeAsync(60_000)
    })
    expect(
      screen.getByText('Current cycle is still in progress')
    ).toBeInTheDocument()
    expect(
      vi.mocked(api.get).mock.calls.filter(([url]) => url.endsWith('/events'))
        .length
    ).toBeGreaterThanOrEqual(2)
  } finally {
    client.clear()
  }
})

test('累计125%时显示总比例与第二段25%而不是超限或红色警报', () => {
  const summary = {
    summary: {
      seven_day: { observed_units: 5, evaluated_cycles: 0, reset_count: 3 },
    },
    current: { '604800': { used_basis_points: null } },
  } as unknown as Summary
  render(<QuotaMetrics data={summary} />)
  expect(screen.getByText('125%')).toBeInTheDocument()
  expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '25')
  expect(screen.getByRole('progressbar')).toHaveAttribute(
    'aria-valuetext',
    expect.stringContaining('125%')
  )
  expect(screen.getByText('Segment 2 · 25%')).toBeInTheDocument()
  expect(screen.queryByRole('alert')).not.toBeInTheDocument()
})
