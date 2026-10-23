import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, expect, test } from 'vitest'

import { QuotaMetrics } from '../components/quota-metrics'
import type { Summary } from '../types'

afterEach(cleanup)

const data = {
  summary: {
    coverage_start: 1792368000,
    seven_day: {
      observed_units: 5,
      evaluated_cycles: 2,
      unused_units: 0.79,
      reset_count: 3,
      early_resets: 3,
      expired_resets: 0,
      manual_resets: 0,
    },
  },
  current: { '604800': { used_basis_points: 4700 } },
} as unknown as Summary

test('主卡统一四个内容行，避免比例卡多一行导致留白不对称', () => {
  render(<QuotaMetrics data={data} />)
  const cards = screen.getAllByRole('group')
  for (const card of cards.slice(0, 2)) {
    expect(card).toHaveClass('py-0')
    const content = card.querySelector('[data-slot="card-content"]')
    if (!content) throw new Error('主指标卡缺少内容区域')
    expect(content).toHaveClass('p-6', 'grid')
    expect(content.children).toHaveLength(4)
    expect(content.children[0]).toHaveClass('text-base')
    expect(content.children[1]).toHaveClass('text-4xl', 'font-bold')
    expect(content.children[3]).toHaveClass('text-sm')
  }
  expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '25')
})

test('辅助卡统一三行，标题和数值使用放大的清晰字重', () => {
  render(<QuotaMetrics data={data} />)
  for (const card of screen.getAllByRole('group').slice(2)) {
    const content = card.querySelector('[data-slot="card-content"]')
    if (!content) throw new Error('辅助指标卡缺少内容区域')
    expect(content.children).toHaveLength(3)
    expect(content.children[0]).toHaveClass('text-base')
    expect(content.children[1]).toHaveClass('text-3xl', 'font-bold')
    expect(content.children[2]).toHaveClass('text-sm')
  }
})
