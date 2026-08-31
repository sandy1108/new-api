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
import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, test, vi } from 'vitest'

import { TokenDetail } from '../components/token-detail'
import { TokenTable } from '../components/token-table'
import { useUsageSummary } from '../hooks/use-usage-summary'
import { UsageSummary } from '../index'
import type { UsageTokenBucket } from '../types'

vi.mock('../hooks/use-usage-summary', () => ({
  useUsageSummary: vi.fn(),
}))

const mockedUseUsageSummary = vi.mocked(useUsageSummary)

function makeToken(
  overrides: Partial<UsageTokenBucket> = {}
): UsageTokenBucket {
  return {
    id: 10,
    name: 'Codex token',
    key: '[1,10,"Codex token"]',
    userId: 1,
    username: 'alice',
    requests: 5,
    inputTokens: 100,
    outputTokens: 50,
    totalTokens: 150,
    quota: 600,
    channels: [
      {
        id: 2,
        name: 'Official',
        key: '[2,"Official"]',
        requests: 5,
        inputTokens: 100,
        outputTokens: 50,
        totalTokens: 150,
        quota: 600,
        models: [
          {
            name: 'gpt-5.6',
            requests: 5,
            inputTokens: 100,
            outputTokens: 50,
            totalTokens: 150,
            quota: 600,
          },
        ],
      },
    ],
    ...overrides,
  }
}

describe('usage summary hierarchy', () => {
  test('marks the selected token and emits click and keyboard selection changes', () => {
    const first = makeToken()
    const second = makeToken({
      id: 11,
      name: 'Backup token',
      key: '[1,11,"Backup token"]',
    })
    const onSelect = vi.fn()

    render(
      <TokenTable
        tokens={[first, second]}
        selectedKey={first.key}
        onSelect={onSelect}
      />
    )

    const firstRow = screen.getByRole('button', { name: /Codex token/ })
    const secondRow = screen.getByRole('button', { name: /Backup token/ })
    expect(firstRow).toHaveAttribute('aria-selected', 'true')
    expect(secondRow).toHaveAttribute('aria-selected', 'false')

    fireEvent.click(secondRow)
    fireEvent.keyDown(secondRow, { key: 'Enter' })

    expect(onSelect).toHaveBeenNthCalledWith(1, second.key)
    expect(onSelect).toHaveBeenNthCalledWith(2, second.key)
  })

  test('renders the selected token channel and model details, including empty state', () => {
    const token = makeToken()
    const { rerender } = render(<TokenDetail token={token} />)

    expect(screen.getByText('Official')).toBeInTheDocument()
    expect(screen.getByText('gpt-5.6')).toBeInTheDocument()
    expect(screen.getAllByText('150')).toHaveLength(2)

    rerender(<TokenDetail token={makeToken({ channels: [] })} />)
    expect(
      screen.getByText('No channel usage for this token.')
    ).toBeInTheDocument()
  })
})

const summaryData = {
  total_requests: 5,
  total_input_tokens: 100,
  total_output_tokens: 50,
  total_tokens: 150,
  total_quota: 600,
  items: [
    {
      user_id: 1,
      username: 'alice',
      token_id: 10,
      token_name: 'Codex token',
      channel_id: 2,
      channel_name: 'Official',
      model_name: 'gpt-5.6',
      requests: 5,
      input_tokens: 100,
      output_tokens: 50,
      total_tokens: 150,
      quota: 600,
    },
  ],
}

function mockSummaryQuery(overrides: Record<string, unknown> = {}): void {
  mockedUseUsageSummary.mockReturnValue({
    data: undefined,
    isLoading: false,
    isFetching: false,
    isError: false,
    error: null,
    refetch: vi.fn().mockResolvedValue(undefined),
    scope: 'self',
    canManageScope: false,
    ...overrides,
  } as never)
}

describe('usage summary page states', () => {
  test('shows loading, data, empty, refresh and error states without pagination fallback', () => {
    mockSummaryQuery({ isLoading: true })
    const { rerender } = render(<UsageSummary />)
    expect(screen.getByTestId('usage-summary-loading')).toBeInTheDocument()

    mockSummaryQuery({ data: summaryData })
    rerender(<UsageSummary />)
    expect(screen.getByText('Codex token')).toBeInTheDocument()
    expect(screen.getAllByText('150').length).toBeGreaterThan(0)

    mockSummaryQuery({ data: { ...summaryData, items: [] } })
    rerender(<UsageSummary />)
    expect(
      screen.getByText('No consumption logs in this range.')
    ).toBeInTheDocument()

    const refetch = vi.fn().mockResolvedValue(undefined)
    mockSummaryQuery({ data: summaryData, isFetching: true, refetch })
    rerender(<UsageSummary />)
    expect(screen.getByText('Codex token')).toBeInTheDocument()
    expect(screen.getByText('Refreshing...')).toBeInTheDocument()

    mockSummaryQuery({
      isError: true,
      error: new Error('request failed'),
      refetch,
    })
    rerender(<UsageSummary />)
    expect(screen.getByText('Failed to load usage summary')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
    expect(refetch).toHaveBeenCalledOnce()
  })

  test('does not render the administrator scope switch for a regular user', () => {
    mockSummaryQuery({
      data: summaryData,
      scope: 'self',
      canManageScope: false,
    })
    render(<UsageSummary />)

    expect(screen.queryByRole('tab', { name: 'All' })).toBeNull()
    expect(screen.queryByRole('tab', { name: 'Only Mine' })).toBeNull()
  })

  test('renders administrator scope controls when the hook grants the permission', () => {
    mockSummaryQuery({ data: summaryData, scope: 'all', canManageScope: true })
    render(<UsageSummary />)

    expect(screen.getByRole('tab', { name: 'All' })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: 'Only Mine' })).toBeInTheDocument()
  })
})
