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

export function formatUsageQuota(value: number): string {
  return formatUsageNumber(value)
}
