import { z } from 'zod'

export const settingsSchema = z.object({
  enabled: z.boolean(),
  interval_minutes: z.number().int().min(2).max(1440),
})
export const subjectSchema = z.object({
  name: z
    .string()
    .trim()
    .min(1)
    .max(128)
    .refine((value) => !/\p{Cc}/u.test(value), 'Invalid name'),
  source_channel_id: z.number().int().positive(),
  enabled: z.boolean(),
})
export const decisionSchema = z.object({
  decision: z.enum(['confirm_reset', 'plan_correction', 'segment_change']),
  reason: z
    .string()
    .trim()
    .min(1)
    .max(500)
    .refine((value) => !/\p{Cc}/u.test(value), 'Invalid reason'),
})

// 固定北京时间自然月，不能用浏览器本地时区推断月界。
export function beijingMonth(now = new Date()): string {
  return new Date(now.getTime() + 8 * 3600_000).toISOString().slice(0, 7)
}
export function ledgerTime(seconds: number, locale?: string): string {
  if (!seconds) return '—'
  return new Intl.DateTimeFormat(locale, {
    timeZone: 'Asia/Shanghai',
    dateStyle: 'short',
    timeStyle: 'short',
  }).format(new Date(seconds * 1000))
}

// 1份等于一整轮周额度；普通两位小数会把微小的有效增长显示为0。
// 公共formatNumber尚不支持精度选项，因此在私有模块保留四位小数，不影响官方页面。
export function formatWeeklyUnits(
  value: number | null | undefined,
  locale?: string
): string {
  if (value == null || !Number.isFinite(value)) return '—'
  return new Intl.NumberFormat(locale, { maximumFractionDigits: 4 }).format(
    value
  )
}
