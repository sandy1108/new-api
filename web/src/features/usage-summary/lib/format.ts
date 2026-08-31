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

const usageNumberFormatter = new Intl.NumberFormat('en-US', {
  maximumFractionDigits: 0,
})

/**
 * Format usage counters consistently without hiding negative values.
 * Invalid numeric responses are rendered as zero and handled as an API/data
 * validation concern by the caller.
 */
export function formatUsageNumber(value: number): string {
  return usageNumberFormatter.format(Number.isFinite(value) ? value : 0)
}

/**
 * 使用中文单位压缩大数，同时保留稳定的完整数值格式供 title 和核对使用。
 */
export function formatUsageCompactNumber(value: number): string {
  const safeValue = Number.isFinite(value) ? value : 0
  const absoluteValue = Math.abs(safeValue)
  let unit: { divisor: number; suffix: string } | null = null
  if (absoluteValue >= 100_000_000) {
    unit = { divisor: 100_000_000, suffix: '亿' }
  } else if (absoluteValue >= 10_000) {
    unit = { divisor: 10_000, suffix: '万' }
  } else if (absoluteValue >= 1_000) {
    unit = { divisor: 1_000, suffix: '千' }
  }

  if (!unit) return formatUsageNumber(safeValue)

  const compactFormatter = new Intl.NumberFormat('en-US', {
    maximumFractionDigits: unit.suffix === '千' ? 1 : 2,
  })
  return `${compactFormatter.format(safeValue / unit.divisor)}${unit.suffix}`
}

export function formatUsageQuota(value: number): string {
  return formatUsageNumber(value)
}
