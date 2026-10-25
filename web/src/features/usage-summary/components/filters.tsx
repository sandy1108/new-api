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
import { RefreshCw } from 'lucide-react'
import { useEffect, useRef, useState, type RefObject } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { cn } from '@/lib/utils'

import { USAGE_SUMMARY_RANGES } from '../constants'
import type { UsageSummaryRangeId, UsageSummaryScope } from '../types'

const usageSummaryRangeIds = new Set<string>(
  USAGE_SUMMARY_RANGES.map((option) => option.id)
)

export interface UsageSummaryFiltersProps {
  layoutRef: RefObject<HTMLDivElement | null>
  titleRef: RefObject<HTMLSpanElement | null>
  rangeId: UsageSummaryRangeId
  scope: UsageSummaryScope
  canManageScope: boolean
  refreshing?: boolean
  onRangeChange: (rangeId: UsageSummaryRangeId) => void
  onScopeChange: (scope: UsageSummaryScope) => void
  onRefresh: () => void
}

export function UsageSummaryFilters(props: UsageSummaryFiltersProps) {
  const { t, i18n } = useTranslation()
  const rangesRef = useRef<HTMLDivElement>(null)
  const secondaryRef = useRef<HTMLDivElement>(null)
  const [expanded, setExpanded] = useState(false)

  // 等待祖先容器ref完成挂载后再测量；子组件的layout effect早于祖先ref挂载。
  useEffect(() => {
    const page = props.layoutRef.current
    const title = props.titleRef.current
    const ranges = rangesRef.current
    const secondary = secondaryRef.current
    if (!page || !title || !ranges || !secondary) return

    // 始终测量同一组实际标签，避免下拉/平铺切换改变测量基准而反复抖动。
    // 64px预留页边距、标题间距、筛选组间距；侧栏及翻译变化同样参与计算。
    const measure = () => {
      const required =
        ranges.getBoundingClientRect().width +
        secondary.getBoundingClientRect().width +
        title.getBoundingClientRect().width +
        64
      setExpanded(page.getBoundingClientRect().width >= required)
    }
    measure()
    const observer = new ResizeObserver(measure)
    for (const element of [page, title, ranges, secondary]) {
      observer.observe(element)
    }
    return () => observer.disconnect()
  }, [
    props.layoutRef,
    props.titleRef,
    props.canManageScope,
    i18n.resolvedLanguage,
  ])

  const handleRangeChange = (value: string | null) => {
    if (value && usageSummaryRangeIds.has(value)) {
      props.onRangeChange(value as UsageSummaryRangeId)
    }
  }

  const handleScopeChange = (value: string) => {
    if (value === 'all' || value === 'self') {
      props.onScopeChange(value)
    }
  }

  return (
    <div
      className='relative flex max-w-full flex-wrap items-center gap-2'
      data-testid='usage-summary-filters'
    >
      <div
        ref={rangesRef}
        data-testid='usage-range-tabs'
        aria-hidden={!expanded}
        inert={!expanded}
        className={cn(
          'w-max shrink-0',
          !expanded && 'pointer-events-none invisible absolute top-0 left-0'
        )}
      >
        <Tabs value={props.rangeId} onValueChange={handleRangeChange}>
          <TabsList aria-label={t('Date range')}>
            {USAGE_SUMMARY_RANGES.map((option) => (
              <TabsTrigger
                key={option.id}
                value={option.id}
                className='min-w-14 px-3'
              >
                {t(option.labelKey)}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
      </div>
      {!expanded && (
        <Select value={props.rangeId} onValueChange={handleRangeChange}>
          <SelectTrigger
            id='usage-summary-range'
            size='sm'
            className='min-w-28'
            aria-label={t('Date range')}
          >
            <SelectValue>
              {t(
                USAGE_SUMMARY_RANGES.find(
                  (option) => option.id === props.rangeId
                )?.labelKey ?? 'Today'
              )}
            </SelectValue>
          </SelectTrigger>
          <SelectContent alignItemWithTrigger={false}>
            <SelectGroup>
              {USAGE_SUMMARY_RANGES.map((option) => (
                <SelectItem key={option.id} value={option.id}>
                  {t(option.labelKey)}
                </SelectItem>
              ))}
            </SelectGroup>
          </SelectContent>
        </Select>
      )}
      <div
        ref={secondaryRef}
        data-testid='usage-summary-secondary-filters'
        className='flex shrink-0 items-center gap-2'
      >
        {props.canManageScope && (
          <Tabs value={props.scope} onValueChange={handleScopeChange}>
            <TabsList>
              <TabsTrigger value='all'>{t('All')}</TabsTrigger>
              <TabsTrigger value='self'>{t('Only Mine')}</TabsTrigger>
            </TabsList>
          </Tabs>
        )}

        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={props.onRefresh}
          disabled={props.refreshing}
          aria-label={t('Refresh')}
        >
          <RefreshCw
            className={props.refreshing ? 'size-3.5 animate-spin' : 'size-3.5'}
            aria-hidden='true'
          />
          {props.refreshing ? t('Refreshing...') : t('Refresh')}
        </Button>
      </div>
    </div>
  )
}
