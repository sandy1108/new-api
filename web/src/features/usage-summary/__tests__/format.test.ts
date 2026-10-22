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

import { INTERFACE_LANGUAGE_OPTIONS, toIntlLocale } from '@/i18n/languages'

import {
  formatUsageCompactNumber,
  formatUsageNumber,
  formatUsageQuota,
  formatUsageCost,
} from '../lib/format'

describe('usage summary number formatting', () => {
  test.each([
    ...INTERFACE_LANGUAGE_OPTIONS.map((language) => language.code),
    'invalid_locale',
  ])('formats numbers and costs for interface language %s', (language) => {
    const locale = toIntlLocale(language)
    expect(formatUsageNumber(1234567, locale)).toBe(
      new Intl.NumberFormat(locale).format(1234567)
    )
    expect(formatUsageCompactNumber(1234567, locale)).toBe(
      new Intl.NumberFormat(locale, {
        notation: 'compact',
        maximumFractionDigits: 1,
      }).format(1234567)
    )
    expect(
      formatUsageCost(
        500000,
        { quotaPerUnit: 500000, usdExchangeRate: 7 },
        locale
      ).cny
    ).toBe(
      new Intl.NumberFormat(locale, {
        style: 'currency',
        currency: 'CNY',
        minimumFractionDigits: 2,
        maximumFractionDigits: 2,
      }).format(7)
    )
  })
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

  test('formats large usage counts with Chinese compact units', () => {
    expect(formatUsageCompactNumber(123456789, 'zh-CN')).toBe('1.2亿')
    expect(formatUsageCompactNumber(36842000, 'zh-CN')).toBe('3684.2万')
    expect(formatUsageCompactNumber(9876, 'zh-CN')).toBe('9876')
    expect(formatUsageCompactNumber(999)).toBe('999')
    expect(formatUsageCompactNumber(Number.NaN)).toBe('0')
  })

  test('converts consumed quota to CNY and USD without using display mode or recharge price', () => {
    expect(
      formatUsageCost(
        1_000_000,
        { quotaPerUnit: 500_000, usdExchangeRate: 7 },
        'en-US'
      )
    ).toEqual({ cny: 'CN¥14.00', usd: '$2.00' })
    expect(
      formatUsageCost(
        1_000_000,
        { quotaPerUnit: 250_000, usdExchangeRate: 8 },
        'en-US'
      )
    ).toEqual({ cny: 'CN¥32.00', usd: '$4.00' })
  })

  test('keeps small nonzero costs visible and zero costs distinct from unavailable costs', () => {
    const config = { quotaPerUnit: 500_000, usdExchangeRate: 7 }
    expect(formatUsageCost(1, config, 'en-US')).toEqual({
      cny: 'CN¥0.000014',
      usd: '$0.000002',
    })
    expect(formatUsageCost(0, config, 'en-US')).toEqual({
      cny: 'CN¥0.00',
      usd: '$0.00',
    })
    expect(formatUsageCost(-500_000, config, 'en-US')).toEqual({
      cny: '-CN¥7.00',
      usd: '-$1.00',
    })
  })

  test.each([0, -1, Number.NaN, Number.POSITIVE_INFINITY])(
    'does not invent amounts for invalid conversion parameters: %s',
    (invalid) => {
      expect(
        formatUsageCost(
          100,
          { quotaPerUnit: invalid, usdExchangeRate: 7 },
          'en-US'
        )
      ).toEqual({ cny: null, usd: null })
      expect(
        formatUsageCost(
          100,
          { quotaPerUnit: 100, usdExchangeRate: invalid },
          'en-US'
        )
      ).toEqual({ cny: null, usd: '$1.00' })
    }
  )
})
