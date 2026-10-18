import { api } from '@/lib/api'
import { requireServerSuccess } from '@/lib/server-error-message'

import type {
  Channel,
  Cycle,
  Decision,
  Filters,
  Page,
  QuotaEvent,
  Settings,
  Subject,
  Summary,
  Trend,
} from './types'

const base = '/api/private-quota'
const writeConfig = { headers: { 'X-PongAPI-Request': '1' } }
export const quotaKey = ['private-quota'] as const

export async function getSettings(): Promise<Settings> {
  return requireServerSuccess((await api.get(`${base}/settings`)).data).data
}
export async function saveSettings(
  input: Pick<Settings, 'enabled' | 'interval_minutes' | 'version'>
): Promise<Settings> {
  return requireServerSuccess(
    (await api.put(`${base}/settings`, input, writeConfig)).data
  ).data
}
export async function getSubjects(page: number): Promise<Page<Subject>> {
  return requireServerSuccess(
    (await api.get(`${base}/subjects`, { params: { page, page_size: 100 } }))
      .data
  ).data
}
export async function getChannels(page: number): Promise<Page<Channel>> {
  return requireServerSuccess(
    (await api.get(`${base}/channels`, { params: { page, page_size: 100 } }))
      .data
  ).data
}
export async function saveSubject(input: {
  id?: number
  name: string
  source_channel_id: number
  enabled: boolean
  version?: number
}): Promise<Subject> {
  const response = input.id
    ? await api.put(`${base}/subjects/${input.id}`, input, writeConfig)
    : await api.post(`${base}/subjects`, input, writeConfig)
  return requireServerSuccess(response.data).data
}
export async function getSummary(filters: Filters): Promise<Summary> {
  return requireServerSuccess(
    (await api.get(`${base}/summary`, { params: filters })).data
  ).data
}
export async function getTrend(filters: Filters): Promise<Page<Trend>> {
  return requireServerSuccess(
    (await api.get(`${base}/trend`, { params: filters })).data
  ).data
}
export async function getCycles(
  filters: Filters,
  page: number
): Promise<Page<Cycle>> {
  return requireServerSuccess(
    (await api.get(`${base}/cycles`, { params: { ...filters, page } })).data
  ).data
}
export async function getEvents(
  filters: Filters,
  page: number
): Promise<Page<QuotaEvent>> {
  return requireServerSuccess(
    (await api.get(`${base}/events`, { params: { ...filters, page } })).data
  ).data
}
export async function decideEvent(input: {
  id: number
  decision: Decision
  reason: string
  event_version: number
  subject_version: number
}): Promise<{ ledger_revision: number }> {
  return requireServerSuccess(
    (await api.post(`${base}/events/${input.id}/decision`, input, writeConfig))
      .data
  ).data
}
