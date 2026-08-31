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
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'

import { USAGE_SUMMARY_RANGES } from '../constants'
import type { UsageSummaryRangeId, UsageSummaryScope } from '../types'

const usageSummaryRangeIds = new Set<string>(
  USAGE_SUMMARY_RANGES.map((option) => option.id)
)

export interface UsageSummaryFiltersProps {
  rangeId: UsageSummaryRangeId
  scope: UsageSummaryScope
  canManageScope: boolean
  refreshing?: boolean
  onRangeChange: (rangeId: UsageSummaryRangeId) => void
  onScopeChange: (scope: UsageSummaryScope) => void
  onRefresh: () => void
}

export function UsageSummaryFilters(props: UsageSummaryFiltersProps) {
  const { t } = useTranslation()

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
      className='flex flex-wrap items-center gap-2'
      data-testid='usage-summary-filters'
    >
      <label htmlFor='usage-summary-range' className='sr-only'>
        {t('Date range')}
      </label>
      <Select value={props.rangeId} onValueChange={handleRangeChange}>
        <SelectTrigger
          id='usage-summary-range'
          size='sm'
          aria-label={t('Date range')}
        >
          <SelectValue />
        </SelectTrigger>
        <SelectContent alignItemWithTrigger={false}>
          {USAGE_SUMMARY_RANGES.map((option) => (
            <SelectItem key={option.id} value={option.id}>
              {t(option.labelKey)}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

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
  )
}
