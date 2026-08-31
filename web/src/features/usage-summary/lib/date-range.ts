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

import type { UsageSummaryRange, UsageSummaryRangeId } from '../types'

const MILLISECONDS_PER_SECOND = 1000

function startOfLocalDay(date: Date): Date {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate())
}

function startOfLocalWeek(date: Date): Date {
  const start = startOfLocalDay(date)
  const daysSinceMonday = (start.getDay() + 6) % 7
  start.setDate(start.getDate() - daysSinceMonday)
  return start
}

function toUnixSeconds(date: Date): number {
  return Math.floor(date.getTime() / MILLISECONDS_PER_SECOND)
}

function endOfPreviousPeriod(nextPeriodStart: Date): Date {
  return new Date(nextPeriodStart.getTime() - 1)
}

/**
 * Build the closed Unix-second range used by the usage-summary endpoints.
 * Current periods end at the supplied moment; completed periods end one
 * millisecond before their next local-calendar boundary.
 */
export function createUsageSummaryRange(
  id: UsageSummaryRangeId,
  now: Date = new Date()
): UsageSummaryRange {
  const current = new Date(now)
  let start = startOfLocalDay(current)
  let end = current

  switch (id) {
    case 'yesterday': {
      const todayStart = startOfLocalDay(current)
      start = new Date(todayStart)
      start.setDate(start.getDate() - 1)
      end = endOfPreviousPeriod(todayStart)
      break
    }
    case 'week':
      start = startOfLocalWeek(current)
      break
    case 'last-week': {
      const currentWeekStart = startOfLocalWeek(current)
      start = new Date(currentWeekStart)
      start.setDate(start.getDate() - 7)
      end = endOfPreviousPeriod(currentWeekStart)
      break
    }
    case 'month':
      start = new Date(current.getFullYear(), current.getMonth(), 1)
      break
    case 'last-month': {
      const currentMonthStart = new Date(
        current.getFullYear(),
        current.getMonth(),
        1
      )
      start = new Date(current.getFullYear(), current.getMonth() - 1, 1)
      end = endOfPreviousPeriod(currentMonthStart)
      break
    }
    case 'quarter': {
      const quarterStartMonth = Math.floor(current.getMonth() / 3) * 3
      start = new Date(current.getFullYear(), quarterStartMonth, 1)
      break
    }
    case 'today':
      break
  }

  const startTimestamp = toUnixSeconds(start)
  return {
    id,
    startTimestamp,
    endTimestamp: toUnixSeconds(end),
    cacheKey: `${id}:${startTimestamp}`,
  }
}
