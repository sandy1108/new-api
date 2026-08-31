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

import { createUsageSummaryRange } from '../lib/date-range'

const NOW = new Date('2026-08-05T14:32:18+08:00')
const timestamp = (value: string) => Math.floor(new Date(value).getTime() / 1000)

describe('createUsageSummaryRange', () => {
  test('today starts at local midnight and ends at now', () => {
    const range = createUsageSummaryRange('today', NOW)

    expect(range).toEqual({
      id: 'today',
      startTimestamp: timestamp('2026-08-05T00:00:00+08:00'),
      endTimestamp: timestamp('2026-08-05T14:32:18+08:00'),
      cacheKey: `today:${timestamp('2026-08-05T00:00:00+08:00')}`,
    })
  })

  test('yesterday covers the previous complete local calendar day', () => {
    const range = createUsageSummaryRange('yesterday', NOW)

    expect(range.startTimestamp).toBe(
      timestamp('2026-08-04T00:00:00+08:00')
    )
    expect(range.endTimestamp).toBe(
      timestamp('2026-08-04T23:59:59+08:00')
    )
  })

  test('this week starts on Monday and ends at now', () => {
    const range = createUsageSummaryRange('week', NOW)

    expect(range.startTimestamp).toBe(
      timestamp('2026-08-03T00:00:00+08:00')
    )
    expect(range.endTimestamp).toBe(
      timestamp('2026-08-05T14:32:18+08:00')
    )
  })

  test('last week covers the previous complete Monday-to-Sunday week', () => {
    const range = createUsageSummaryRange('last-week', NOW)

    expect(range.startTimestamp).toBe(
      timestamp('2026-07-27T00:00:00+08:00')
    )
    expect(range.endTimestamp).toBe(
      timestamp('2026-08-02T23:59:59+08:00')
    )
  })

  test('this month starts on the first local day and ends at now', () => {
    const range = createUsageSummaryRange('month', NOW)

    expect(range.startTimestamp).toBe(
      timestamp('2026-08-01T00:00:00+08:00')
    )
    expect(range.endTimestamp).toBe(
      timestamp('2026-08-05T14:32:18+08:00')
    )
  })

  test('last month covers the previous complete local calendar month', () => {
    const range = createUsageSummaryRange('last-month', NOW)

    expect(range.startTimestamp).toBe(
      timestamp('2026-07-01T00:00:00+08:00')
    )
    expect(range.endTimestamp).toBe(
      timestamp('2026-07-31T23:59:59+08:00')
    )
  })

  test('this quarter starts at the calendar quarter boundary', () => {
    const range = createUsageSummaryRange('quarter', NOW)

    expect(range.startTimestamp).toBe(
      timestamp('2026-07-01T00:00:00+08:00')
    )
    expect(range.endTimestamp).toBe(
      timestamp('2026-08-05T14:32:18+08:00')
    )
  })
})
