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

import { formatUsageNumber, formatUsageQuota } from '../lib/format'

describe('usage summary number formatting', () => {
  test('formats finite usage counts with thousands separators', () => {
    expect(formatUsageNumber(1234567)).toBe('1,234,567')
  })

  test('formats non-finite usage counts as zero', () => {
    expect(formatUsageNumber(Number.NaN)).toBe('0')
    expect(formatUsageNumber(Number.POSITIVE_INFINITY)).toBe('0')
  })

  test('formats quota through the same safe integer representation', () => {
    expect(formatUsageQuota(420000)).toBe('420,000')
  })
})
