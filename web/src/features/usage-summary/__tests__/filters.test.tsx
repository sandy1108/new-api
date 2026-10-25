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
import { act, fireEvent, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import i18next from 'i18next'
import { useRef } from 'react'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { UsageSummaryFilters } from '../components/filters'
import type { UsageSummaryRangeId } from '../types'

let pageWidth = 1000
let rangesWidth = 500
let resize: () => void
const layout = document.createElement('div')
const title = document.createElement('span')
const onRangeChange = vi.fn()

beforeEach(() => {
  i18next.addResourceBundle('zhCN', 'translation', {
    'Date range': '时间范围',
    'Last Month': '上月',
    Yesterday: '昨天',
  })
  pageWidth = 1000
  rangesWidth = 500
  vi.stubGlobal(
    'ResizeObserver',
    class {
      constructor(private callback: () => void) {}
      observe(element: Element) {
        if (element === layout) resize = this.callback
      }
      disconnect() {}
    }
  )
  vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(
    function (this: HTMLElement) {
      let width = 120
      if (this === layout || this.dataset.testid === 'usage-filter-page') {
        width = pageWidth
      }
      if (this.dataset.testid === 'usage-range-tabs') width = rangesWidth
      if (this.dataset.testid === 'usage-summary-secondary-filters') width = 180
      return {
        width,
        height: 32,
        top: 0,
        bottom: 32,
        left: 0,
        right: width,
        x: 0,
        y: 0,
        toJSON: () => ({}),
      }
    }
  )
})

afterEach(async () => {
  vi.unstubAllGlobals()
  await i18next.changeLanguage('en')
  i18next.removeResourceBundle('zhCN', 'translation')
})

function filters(
  rangeId: UsageSummaryRangeId = 'last-month',
  canManageScope = true
) {
  return (
    <UsageSummaryFilters
      rangeId={rangeId}
      scope='all'
      canManageScope={canManageScope}
      layoutRef={{ current: layout }}
      titleRef={{ current: title }}
      onRangeChange={onRangeChange}
      onScopeChange={vi.fn()}
      onRefresh={vi.fn()}
    />
  )
}

test('when all ranges fit, shows seven directly selectable tabs with the current range selected', () => {
  render(filters())
  const ranges = within(screen.getByRole('tablist', { name: 'Date range' }))
  expect(ranges.getAllByRole('tab')).toHaveLength(7)
  expect(ranges.getByRole('tab', { name: 'Last Month' })).toHaveAttribute(
    'aria-selected',
    'true'
  )
  fireEvent.click(ranges.getByRole('tab', { name: 'Yesterday' }))
  expect(onRangeChange).toHaveBeenCalledWith('yesterday')
  expect(
    screen.queryByRole('combobox', { name: 'Date range' })
  ).not.toBeInTheDocument()
})

test('when width becomes too narrow, falls back to the dropdown and keeps the translated selection', async () => {
  const view = render(filters())
  act(() => {
    pageWidth = 420
    resize()
  })
  expect(
    screen.queryByRole('tablist', { name: 'Date range' })
  ).not.toBeInTheDocument()
  expect(
    screen.getByRole('combobox', { name: 'Date range' })
  ).toHaveTextContent('Last Month')
  await act(() => i18next.changeLanguage('zhCN'))
  expect(screen.getByRole('combobox', { name: '时间范围' })).toHaveTextContent(
    '上月'
  )
  view.rerender(filters('yesterday'))
  act(() => {
    pageWidth = 1000
    resize()
  })
  expect(screen.getByRole('tab', { name: '昨天' })).toHaveAttribute(
    'aria-selected',
    'true'
  )
})

test('uses rendered label widths when deciding whether tabs fit, and expands again when they fit exactly', () => {
  render(filters())
  act(() => {
    rangesWidth = 700
    resize()
  })
  expect(
    screen.getByRole('combobox', { name: 'Date range' })
  ).toBeInTheDocument()
  act(() => {
    pageWidth = 1064
    resize()
  })
  expect(
    screen.getByRole('tablist', { name: 'Date range' })
  ).toBeInTheDocument()
})

test('compact dropdown changes the range and expanded tabs support keyboard navigation', async () => {
  pageWidth = 420
  render(filters())
  const user = userEvent.setup()
  await user.click(screen.getByRole('combobox', { name: 'Date range' }))
  await user.click(screen.getByRole('option', { name: 'Yesterday' }))
  expect(onRangeChange).toHaveBeenCalledWith('yesterday')
  act(() => {
    pageWidth = 1000
    resize()
  })
  screen.getByRole('tab', { name: 'Last Month' }).focus()
  await user.keyboard('{ArrowRight}{Enter}')
  expect(onRangeChange).toHaveBeenCalledWith('quarter')
})

test('without administrator scope, only date range tabs are exposed', () => {
  render(filters('today', false))
  expect(screen.getAllByRole('tablist')).toHaveLength(1)
  expect(
    screen.queryByRole('tab', { name: 'Only Mine' })
  ).not.toBeInTheDocument()
})

function PageFixture() {
  const layoutRef = useRef<HTMLDivElement>(null)
  const titleRef = useRef<HTMLSpanElement>(null)
  return (
    <div ref={layoutRef} data-testid='usage-filter-page'>
      <span ref={titleRef}>Token Usage</span>
      <UsageSummaryFilters
        layoutRef={layoutRef}
        titleRef={titleRef}
        rangeId='today'
        scope='all'
        canManageScope
        onRangeChange={onRangeChange}
        onScopeChange={vi.fn()}
        onRefresh={vi.fn()}
      />
    </div>
  )
}

test('when ancestor refs attach on mount, measures the page and expands without requiring a later resize', () => {
  render(<PageFixture />)
  expect(
    screen.getByRole('tablist', { name: 'Date range' })
  ).toBeInTheDocument()
})
