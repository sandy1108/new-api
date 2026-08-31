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

import { UsageSummaryCards } from '../components/summary-cards'
import { TokenDetail } from '../components/token-detail'
import { TokenDistribution } from '../components/token-distribution'
import { TokenTable } from '../components/token-table'
import { UsageTrend } from '../components/usage-trend'
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
  test('renders the real daily input and output trend points', () => {
    render(
      <UsageTrend
        points={[
          {
            timestamp: 1_756_560_000,
            requests: 2,
            input_tokens: 1000,
            output_tokens: 500,
            total_tokens: 1500,
            quota: 100,
          },
        ]}
      />
    )

    expect(screen.getByTestId('usage-summary-trend')).toBeInTheDocument()
    expect(screen.getByTestId('usage-trend-input-1756560000')).toHaveAttribute(
      'title',
      '1,000'
    )
    expect(screen.getByTestId('usage-trend-output-1756560000')).toHaveAttribute(
      'title',
      '500'
    )
  })

  test('shows API token distribution without mixing channel or model dimensions', () => {
    const first = makeToken({ totalTokens: 75 })
    const second = makeToken({
      id: 11,
      name: 'Backup token',
      key: '[1,11,"Backup token"]',
      totalTokens: 25,
    })

    render(<TokenDistribution tokens={[first, second]} totalTokens={100} />)

    expect(screen.getByTestId('usage-summary-distribution')).toBeInTheDocument()
    expect(screen.getByText('Backup token')).toBeInTheDocument()
    expect(screen.getByText('25%')).toBeInTheDocument()
    expect(screen.queryByText('Official')).toBeNull()
  })

  test('shows compact summary values while retaining the full number in a title', () => {
    render(
      <UsageSummaryCards
        totals={{
          requests: 123456,
          inputTokens: 123456789,
          outputTokens: 36842000,
          totalTokens: 160298789,
          quota: 420000,
        }}
      />
    )

    expect(screen.getByText('1.23亿')).toHaveAttribute('title', '123,456,789')
    expect(screen.getByText('3,684.2万')).toHaveAttribute('title', '36,842,000')
  })

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
    expect(screen.getAllByText('150').length).toBeGreaterThanOrEqual(2)

    rerender(<TokenDetail token={makeToken({ channels: [] })} />)
    expect(
      screen.getByText('No channel usage for this token.')
    ).toBeInTheDocument()
  })

  test('drills from a token into one selected channel and its models', () => {
    const token = makeToken({
      channels: [
        makeToken().channels[0],
        {
          id: 3,
          name: 'Backup',
          key: '[3,"Backup"]',
          requests: 2,
          inputTokens: 30,
          outputTokens: 20,
          totalTokens: 50,
          quota: 200,
          models: [
            {
              name: 'claude-sonnet',
              requests: 2,
              inputTokens: 30,
              outputTokens: 20,
              totalTokens: 50,
              quota: 200,
            },
          ],
        },
      ],
    })
    const onChannelSelect = vi.fn()
    const { rerender } = render(
      <TokenDetail token={token} onChannelSelect={onChannelSelect} />
    )

    expect(screen.getByText('Official')).toBeInTheDocument()
    expect(screen.getByText('gpt-5.6')).toBeInTheDocument()
    expect(screen.queryByText('按模型')).toBeNull()

    fireEvent.click(screen.getByRole('button', { name: /Backup/ }))
    expect(onChannelSelect).toHaveBeenCalledWith('[3,"Backup"]')

    rerender(
      <TokenDetail
        token={token}
        selectedChannelKey='[3,"Backup"]'
        onChannelSelect={onChannelSelect}
      />
    )
    expect(screen.getByText('claude-sonnet')).toBeInTheDocument()
    expect(screen.queryByText('gpt-5.6')).toBeNull()
  })
})

const summaryData = {
  total_requests: 5,
  total_input_tokens: 100,
  total_output_tokens: 50,
  total_tokens: 150,
  total_quota: 600,
  trend: [
    {
      timestamp: 1_756_560_000,
      requests: 3,
      input_tokens: 70,
      output_tokens: 35,
      total_tokens: 105,
      quota: 400,
    },
    {
      timestamp: 1_756_646_400,
      requests: 2,
      input_tokens: 30,
      output_tokens: 15,
      total_tokens: 45,
      quota: 200,
    },
  ],
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

const summaryDataWithMultipleTokens = {
  total_requests: 7,
  total_input_tokens: 160,
  total_output_tokens: 80,
  total_tokens: 240,
  total_quota: 900,
  items: [
    ...summaryData.items,
    {
      user_id: 1,
      username: 'alice',
      token_id: 11,
      token_name: 'Backup token',
      channel_id: 3,
      channel_name: 'Backup',
      model_name: 'claude-sonnet',
      requests: 2,
      input_tokens: 60,
      output_tokens: 30,
      total_tokens: 90,
      quota: 300,
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
  test('switches the detail context when any API token row is clicked', () => {
    mockSummaryQuery({ data: summaryDataWithMultipleTokens })
    render(<UsageSummary />)

    expect(screen.getByText('gpt-5.6')).toBeInTheDocument()
    const backupRow = screen.getByRole('row', { name: /Backup token/ })

    fireEvent.click(backupRow)

    expect(screen.getByText('claude-sonnet')).toBeInTheDocument()
    expect(screen.queryByText('gpt-5.6')).toBeNull()
  })

  test('shows loading, data, empty, refresh and error states without pagination fallback', () => {
    mockSummaryQuery({ isLoading: true })
    const { rerender } = render(<UsageSummary />)
    expect(screen.getByTestId('usage-summary-loading')).toBeInTheDocument()

    mockSummaryQuery({ data: summaryData })
    rerender(<UsageSummary />)
    expect(screen.getByText('API Token Summary')).toBeInTheDocument()
    expect(screen.getByText('Token Details')).toBeInTheDocument()
    expect(screen.getByText('Usage Trend')).toBeInTheDocument()
    expect(screen.getByText('API Token Distribution')).toBeInTheDocument()
    expect(screen.getAllByText('Codex token').length).toBeGreaterThan(0)
    expect(screen.getAllByText('150').length).toBeGreaterThan(0)

    mockSummaryQuery({ data: { ...summaryData, items: [] } })
    rerender(<UsageSummary />)
    expect(
      screen.getByText('No consumption logs in this range.')
    ).toBeInTheDocument()

    const refetch = vi.fn().mockResolvedValue(undefined)
    mockSummaryQuery({ data: summaryData, isFetching: true, refetch })
    rerender(<UsageSummary />)
    expect(screen.getAllByText('Codex token').length).toBeGreaterThan(0)
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
