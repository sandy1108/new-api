import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { QuotaHistory } from '../components/history'

let client: QueryClient
afterEach(() => {
  cleanup()
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
