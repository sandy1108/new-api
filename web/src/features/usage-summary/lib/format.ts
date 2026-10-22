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

import { getCurrencyFractionDigits } from '@/lib/currency'
import { formatCompactNumber, formatNumber } from '@/lib/format'
import type { CurrencyConfig } from '@/stores/system-config-store'

/**
 * Format usage counters consistently without hiding negative values.
 * Invalid numeric responses are rendered as zero and handled as an API/data
 * validation concern by the caller.
 */
export function formatUsageNumber(value: number, locale?: string): string {
  return formatNumber(Number.isFinite(value) ? value : 0, locale)
}

/**
 * 跟随当前语言压缩大数（中文万/亿等），完整数值另供 title 和核对使用。
 */
export function formatUsageCompactNumber(
  value: number,
  locale?: string
): string {
  return formatCompactNumber(Number.isFinite(value) ? value : 0, locale)
}

/** 消耗额度是日志里的原始单位，不等同于实际输入/输出 Token。 */
export function formatUsageQuota(value: number, locale?: string): string {
  return formatUsageNumber(value, locale)
}

/**
 * 与插件一致：从历史日志 quota 换算双币种，不套用当前模型价或充值购买比率。
 * 公共货币展示函数只能跟随站点选择的单一币种，无法同时固定显示 CNY/USD；
 * 此处复用公共精度规则，单独指定币种。无效参数不使用默认汇率冒充有效结果。
 */
export function formatUsageCost(
  quota: number,
  config: Pick<CurrencyConfig, 'quotaPerUnit' | 'usdExchangeRate'>,
  locale?: string
): { cny: string | null; usd: string | null } {
  if (
    !Number.isFinite(quota) ||
    !Number.isFinite(config.quotaPerUnit) ||
    config.quotaPerUnit <= 0
  ) {
    return { cny: null, usd: null }
  }
  const usd = quota / config.quotaPerUnit
  const cny =
    Number.isFinite(config.usdExchangeRate) && config.usdExchangeRate > 0
      ? usd * config.usdExchangeRate
      : Number.NaN
  return {
    cny: formatUsageMoney(cny, 'CNY', locale),
    usd: formatUsageMoney(usd, 'USD', locale),
  }
}

function formatUsageMoney(
  value: number,
  currency: string,
  locale?: string
): string | null {
  if (!Number.isFinite(value)) return null
  const digits = getCurrencyFractionDigits(value, {
    digitsLarge: 2,
    digitsSmall: 6,
  })
  const threshold = 10 ** -digits
  const belowPrecision = value !== 0 && Math.abs(value) < threshold
  const formatted = new Intl.NumberFormat(locale, {
    style: 'currency',
    currency,
    minimumFractionDigits: 2,
    maximumFractionDigits: digits,
  }).format(belowPrecision ? threshold : value)
  if (belowPrecision) return value > 0 ? `<${formatted}` : `>-${formatted}`
  return formatted
}
