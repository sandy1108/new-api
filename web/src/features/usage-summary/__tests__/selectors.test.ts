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
import { describe, expect, test } from 'vitest'

import type { UsageSummaryItem } from '../types'
import { aggregateUsageItems, selectDefaultToken } from '../lib/selectors'

function makeItem(overrides: Partial<UsageSummaryItem>): UsageSummaryItem {
  return {
    user_id: 1,
    username: 'alice',
    token_id: 10,
    token_name: 'Codex',
    channel_id: 2,
    channel_name: 'Official',
    model_name: 'gpt-5.6',
    requests: 1,
    input_tokens: 10,
    output_tokens: 5,
    total_tokens: 15,
    quota: 100,
    ...overrides,
  }
}

describe('usage summary selectors', () => {
  test('sums server aggregation rows without expanding request counts', () => {
    const view = aggregateUsageItems([
      makeItem({ requests: 3, input_tokens: 100, output_tokens: 20, quota: 500 }),
      makeItem({ requests: 2, input_tokens: 30, output_tokens: 10, quota: 100 }),
    ])

    expect(view.totals).toEqual({
      requests: 5,
      inputTokens: 130,
      outputTokens: 30,
      totalTokens: 160,
      quota: 600,
    })
    expect(view.tokens[0]).toMatchObject({
      requests: 5,
      inputTokens: 130,
      outputTokens: 30,
      totalTokens: 160,
      quota: 600,
    })
    expect(view.tokens[0].channels[0].models[0]).toMatchObject({
      requests: 5,
      totalTokens: 160,
    })
  })

  test('keeps same-named tokens from different users separate', () => {
    const view = aggregateUsageItems([
      makeItem({ user_id: 1, username: 'alice', input_tokens: 20 }),
      makeItem({ user_id: 2, username: 'bob', input_tokens: 200 }),
    ])

    expect(view.tokens).toHaveLength(2)
    expect(view.tokens.map((token) => token.username)).toEqual(['bob', 'alice'])
  })

  test('isolates identical channels and models across different tokens', () => {
    const view = aggregateUsageItems([
      makeItem({
        user_id: 1,
        username: 'alice',
        token_id: 10,
        input_tokens: 20,
        output_tokens: 5,
      }),
      makeItem({
        user_id: 2,
        username: 'bob',
        token_id: 11,
        input_tokens: 200,
        output_tokens: 50,
      }),
      makeItem({
        user_id: 1,
        username: 'alice',
        token_id: 10,
        input_tokens: 20,
        output_tokens: 5,
      }),
      makeItem({
        user_id: 2,
        username: 'bob',
        token_id: 11,
        input_tokens: 200,
        output_tokens: 50,
      }),
    ])

    expect(view.tokens).toHaveLength(2)
    expect(
      view.tokens.map((token) => token.channels[0].models[0].totalTokens)
    ).toEqual([500, 50])
    expect(view.tokens.every((token) => token.channels[0].models)).toBe(true)
    expect(view.tokens[0].channels[0].models).toHaveLength(1)
    expect(view.tokens[1].channels[0].models).toHaveLength(1)
  })

  test('sorts tokens, channels, and models by total tokens descending', () => {
    const view = aggregateUsageItems([
      makeItem({ token_id: 10, model_name: 'small', input_tokens: 10 }),
      makeItem({ token_id: 10, model_name: 'large', input_tokens: 100 }),
      makeItem({ token_id: 11, input_tokens: 300 }),
    ])

    expect(view.tokens[0].id).toBe(11)
    expect(view.tokens[1].channels[0].models.map((model) => model.name)).toEqual([
      'large',
      'small',
    ])
  })

  test('uses explicit fallback labels for missing names', () => {
    const view = aggregateUsageItems([
      makeItem({
        token_id: 0,
        token_name: '',
        channel_id: 0,
        channel_name: undefined,
        model_name: '',
      }),
    ])

    expect(view.tokens[0].name).toBe('未命名令牌')
    expect(view.tokens[0].channels[0].name).toBe('未记录渠道')
    expect(view.tokens[0].channels[0].models[0].name).toBe('未记录模型')
  })

  test('returns empty totals and no selected token for empty items', () => {
    const view = aggregateUsageItems([])

    expect(view).toEqual({
      totals: {
        requests: 0,
        inputTokens: 0,
        outputTokens: 0,
        totalTokens: 0,
        quota: 0,
      },
      tokens: [],
    })
    expect(selectDefaultToken(view.tokens)).toBeNull()
  })

  test('keeps an existing selected token and falls back to the top token', () => {
    const view = aggregateUsageItems([
      makeItem({ token_id: 10, input_tokens: 10 }),
      makeItem({ token_id: 11, input_tokens: 100 }),
    ])

    expect(selectDefaultToken(view.tokens, view.tokens[1].key)?.id).toBe(10)
    expect(selectDefaultToken(view.tokens, 'missing')?.id).toBe(11)
  })
})
