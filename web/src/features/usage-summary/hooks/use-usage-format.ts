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

import { useTranslation } from 'react-i18next'

import { toIntlLocale } from '@/i18n/languages'
import { useSystemConfigStore } from '@/stores/system-config-store'

import {
  formatUsageCompactNumber,
  formatUsageCost,
  formatUsageNumber,
} from '../lib/format'

/** 订阅语言和站点配置变化，所有消费维度使用相同的格式与换算参数。 */
export function useUsageFormat() {
  const { i18n } = useTranslation()
  const currency = useSystemConfigStore((state) => state.config.currency)
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  return {
    locale,
    number: (value: number) => formatUsageNumber(value, locale),
    compact: (value: number) => formatUsageCompactNumber(value, locale),
    cost: (quota: number) => formatUsageCost(quota, currency, locale),
  }
}
