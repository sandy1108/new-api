import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { afterEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { SubscriptionQuotaLedger } from '..'
import { beijingMonth, decisionSchema, settingsSchema } from '../lib/forms'
import type { Summary, View } from '../types'

const window = {
  observed_units: 0.3,
  unused_units: 0,
  reset_count: 0,
  early_resets: 0,
  expired_resets: 0,
  manual_resets: 0,
  pending_events: 0,
  pending_since: 0,
  partial_cycles: 1,
  excluded_cycles: 0,
  evaluated_cycles: 0,
  current: null,
}
const subjects = [1, 2].map((id) => ({
  id,
  name: `Plus ${id}`,
  source_channel_id: id,
  enabled: true,
  version: 1,
  next_due_at: 0,
  last_attempt_at: 0,
  last_success_at: 0,
  error_code: '',
}))
let client: QueryClient
afterEach(() => {
  cleanup()
  client?.clear()
  vi.restoreAllMocks()
})

function Harness() {
  const [view, onViewChange] = useState<View>('overview')
  return <SubscriptionQuotaLedger view={view} onViewChange={onViewChange} />
}
function renderLedger() {
  client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <Harness />
    </QueryClientProvider>
  )
}

test('switching subscription loads its own summary and preserves unknown versus zero', async () => {
  vi.spyOn(api, 'get').mockImplementation(async (url, config) => {
    if (url.endsWith('/subjects')) {
      return { data: { success: true, data: { items: subjects, total: 2 } } }
    }
    if (url.endsWith('/trend')) {
      return { data: { success: true, data: { items: [], total: 0 } } }
    }
    const id = Number(config?.params?.subject_id)
    const subject = subjects.find((item) => item.id === id)
    if (!subject) {
      throw new Error('Unknown fixture subject')
    }
    const summary: Summary = {
      collection_enabled: false,
      subject,
      ledger_revision: 1,
      summary: {
        month: '2026-10',
        timezone: 'Asia/Shanghai',
        coverage_start: 1,
        rules_version: 1,
        seven_day: { ...window, observed_units: id === 1 ? 0.3 : 0.7 },
        five_hour: window,
        gap_seconds: 0,
        invalid_samples: 0,
      },
      current: {
        '604800': {
          used_basis_points: id === 1 ? 0 : null,
          reset_at: 0,
          fetched_at: 1,
          quality_code: '',
          stale: true,
        },
      },
      last_observed_at: 1,
    }
    return { data: { success: true, data: summary } }
  })
  renderLedger()
  await screen.findByText('0.3')
  expect(screen.getByText('0%')).toBeInTheDocument()
  const user = userEvent.setup()
  await user.click(screen.getByRole('combobox', { name: 'Subscription' }))
  await user.click(await screen.findByRole('option', { name: 'Plus 2' }))
  await screen.findByText('0.7')
  expect(screen.queryByText('0.3')).not.toBeInTheDocument()
  expect(screen.queryByText('0%')).not.toBeInTheDocument()
  expect(
    screen.getByText('Collection is paused, stale or incomplete')
  ).toBeInTheDocument()
  fireEvent.change(screen.getByLabelText('Month (Asia/Shanghai)'), {
    target: { value: '2026-09' },
  })
  await waitFor(() =>
    expect(api.get).toHaveBeenCalledWith(
      '/api/private-quota/summary',
      expect.objectContaining({
        params: expect.objectContaining({ subject_id: 2, month: '2026-09' }),
      })
    )
  )
})

test('empty subscriptions offer settings and failed queries show retry', async () => {
  vi.spyOn(api, 'get').mockRejectedValue(new Error('offline'))
  renderLedger()
  await screen.findByRole('button', { name: 'Retry' })
  vi.mocked(api.get).mockResolvedValue({
    data: { success: true, data: { items: [], total: 0 } },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
  await screen.findByText('No monitored subscriptions')
})

test('month uses Beijing boundary and forms reject invalid interval and empty decision reasons', () => {
  expect(beijingMonth(new Date('2026-09-30T16:00:00Z'))).toBe('2026-10')
  expect(
    settingsSchema.safeParse({ enabled: true, interval_minutes: 1 }).success
  ).toBe(false)
  expect(
    settingsSchema.safeParse({ enabled: false, interval_minutes: 10 }).success
  ).toBe(true)
  expect(
    decisionSchema.safeParse({ decision: 'confirm_reset', reason: ' ' }).success
  ).toBe(false)
})
