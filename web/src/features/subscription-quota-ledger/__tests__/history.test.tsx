import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { getReportCapabilities } from '@/features/report-access/api'
import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { QuotaHistory } from '../components/history'

vi.mock('@/features/report-access/api', () => ({
  getReportCapabilities: vi.fn(async () => ({
    usage_read_all: true,
    subscription_read_all: true,
    can_manage_grants: true,
    revision: 'root-test',
  })),
}))
beforeEach(() =>
  useAuthStore.getState().auth.setUser({ id: 1, username: 'root', role: 100 })
)
let client: QueryClient
afterEach(() => {
  cleanup()
  useAuthStore.getState().auth.reset()
  client?.clear()
  vi.restoreAllMocks()
})
test('decision requires reason and submits the event and subject versions from preview', async () => {
  const user = userEvent.setup()
  client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  const window = {
    observed_units: 0.32,
    unused_units: 0,
    reset_count: 1,
    evaluated_cycles: 0,
  }
  vi.spyOn(api, 'get').mockImplementation(async (url) => ({
    data: {
      success: true,
      data: {
        items: url.endsWith('/cycles')
          ? []
          : [
              {
                id: 17,
                kind: 'early_reset',
                status: 'pending',
                version: 2,
                observed_from: 1,
                observed_to: 2,
                old_reset_at: 3,
                new_reset_at: 4,
                previews: {
                  confirm_reset: { seven_day: window, five_hour: window },
                  plan_correction: { seven_day: window, five_hour: window },
                  segment_change: { seven_day: window, five_hour: window },
                },
                decisions: [],
              },
            ],
        total: url.endsWith('/cycles') ? 0 : 1,
        ledger_revision: 9,
      },
    },
  }))
  vi.spyOn(api, 'post').mockResolvedValue({
    data: { success: true, data: { ledger_revision: 10 } },
  })
  render(
    <QueryClientProvider client={client}>
      <QuotaHistory
        filters={{ subject_id: 1, month: '2026-10', window_seconds: 604800 }}
      />
    </QueryClientProvider>
  )
  fireEvent.click(
    await screen.findByRole('button', { name: 'Review event #17' })
  )
  await screen.findByRole('alertdialog')
  expect(
    screen.getByRole('note', { name: 'Decision explanation' })
  ).toHaveTextContent(
    'Confirm that a reset occurred. This starts a new local cycle and counts its observed usage; it does not reset upstream quota or add a whole unit.'
  )
  await user.click(screen.getByRole('combobox', { name: 'Decision' }))
  await user.click(screen.getByRole('option', { name: 'Plan correction' }))
  expect(
    screen.getByRole('note', { name: 'Decision explanation' })
  ).toHaveTextContent(
    'Treat this as a time or usage correction, not a reset. Keep the current cycle and count only growth above its previous high-water mark.'
  )
  await user.click(screen.getByRole('combobox', { name: 'Decision' }))
  await user.click(screen.getByRole('option', { name: 'Start new segment' }))
  expect(
    screen.getByRole('note', { name: 'Decision explanation' })
  ).toHaveTextContent(
    'Use only when the account or quota basis is no longer comparable. Start from a new baseline without counting its initial usage or a reset.'
  )
  expect(api.post).not.toHaveBeenCalled()
  await user.click(screen.getByRole('combobox', { name: 'Decision' }))
  await user.click(
    screen.getByRole('option', { name: 'Manual reset decision' })
  )
  fireEvent.click(screen.getByRole('button', { name: 'Save decision' }))
  await screen.findByText('Too small: expected string to have >=1 characters')
  expect(api.post).not.toHaveBeenCalled()
  fireEvent.change(screen.getByLabelText('Decision reason'), {
    target: { value: 'Observed reset manually verified' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Save decision' }))
  await waitFor(() =>
    expect(api.post).toHaveBeenCalledWith(
      '/api/private-quota/events/17/decision',
      {
        id: 17,
        decision: 'confirm_reset',
        reason: 'Observed reset manually verified',
        event_version: 2,
        subject_version: 9,
      },
      { headers: { 'X-PongAPI-Request': '1' } }
    )
  )
})

test('普通观察员可看事件已有决定和各项预览，但没有保存、原因输入和写请求', async () => {
  useAuthStore.getState().auth.setUser({ id: 2, username: 'observer', role: 1 })
  vi.mocked(getReportCapabilities).mockResolvedValue({
    usage_read_all: false,
    subscription_read_all: true,
    can_manage_grants: false,
    revision: 'observer',
  })
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const preview = {
    seven_day: {
      observed_units: 0.25,
      unused_units: 0.75,
      reset_count: 1,
      evaluated_cycles: 1,
    },
  }
  vi.spyOn(api, 'get').mockImplementation(async (url) => ({
    data: {
      success: true,
      data: {
        items: url.endsWith('/cycles')
          ? []
          : [
              {
                id: 9,
                version: 1,
                kind: 'early_reset',
                status: 'pending',
                observed_from: 1,
                observed_to: 2,
                old_reset_at: 3,
                new_reset_at: 4,
                previews: { confirm_reset: preview, plan_correction: preview },
                decisions: [
                  {
                    id: 1,
                    decision: 'plan_correction',
                    reason: 'Existing plain-text reason',
                    actor_id: 1,
                    created_at: 3,
                    supersedes_id: 0,
                  },
                ],
              },
            ],
        total: 1,
      },
    },
  }))
  const write = vi.spyOn(api, 'post')
  render(
    <QueryClientProvider client={client}>
      <QuotaHistory
        filters={{ subject_id: 1, month: '2026-10', window_seconds: 604800 }}
      />
    </QueryClientProvider>
  )
  fireEvent.click(
    await screen.findByRole('button', { name: 'Review event #9' })
  )
  await screen.findByRole('dialog')
  expect(
    screen.getAllByText(/Existing plain-text reason/).length
  ).toBeGreaterThan(0)
  expect(screen.getAllByText(/0.25/)).toHaveLength(2)
  expect(
    screen.queryByRole('button', { name: 'Save decision' })
  ).not.toBeInTheDocument()
  expect(screen.queryByRole('textbox')).not.toBeInTheDocument()
  expect(write).not.toHaveBeenCalled()
})
