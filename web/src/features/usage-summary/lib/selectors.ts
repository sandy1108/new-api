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

import type {
  UsageChannelBucket,
  UsageMetrics,
  UsageModelBucket,
  UsageSummaryItem,
  UsageSummaryViewModel,
  UsageTokenBucket,
} from '../types'

function toFiniteNumber(value: number): number {
  return Number.isFinite(value) ? value : 0
}

function createMetrics(): UsageMetrics {
  return {
    requests: 0,
    inputTokens: 0,
    outputTokens: 0,
    totalTokens: 0,
    quota: 0,
  }
}

function addMetrics(target: UsageMetrics, item: UsageSummaryItem): void {
  target.requests += toFiniteNumber(item.requests)
  target.inputTokens += toFiniteNumber(item.input_tokens)
  target.outputTokens += toFiniteNumber(item.output_tokens)
  target.totalTokens = target.inputTokens + target.outputTokens
  target.quota += toFiniteNumber(item.quota)
}

function identityKey(...parts: Array<number | string>): string {
  return JSON.stringify(parts)
}

function sortByTotalTokens<T extends UsageMetrics>(items: T[]): T[] {
  return items.sort((left, right) => right.totalTokens - left.totalTokens)
}

/**
 * Convert already aggregated server rows to the display hierarchy. Each row
 * is added once; a large `requests` value is never expanded into individual
 * log objects in the browser.
 */
export function aggregateUsageItems(
  items: UsageSummaryItem[]
): UsageSummaryViewModel {
  const totals = createMetrics()
  const tokenMap = new Map<string, UsageTokenBucket>()
  const channelMaps = new Map<string, Map<string, UsageChannelBucket>>()
  const modelMaps = new Map<string, Map<string, UsageModelBucket>>()

  for (const item of items) {
    const tokenName = item.token_name || '未命名令牌'
    const tokenKey = identityKey(item.user_id, item.token_id, tokenName)
    let token = tokenMap.get(tokenKey)
    if (!token) {
      token = {
        ...createMetrics(),
        id: item.token_id,
        name: tokenName,
        key: tokenKey,
        userId: item.user_id,
        username: item.username,
        channels: [],
      }
      tokenMap.set(tokenKey, token)
      channelMaps.set(tokenKey, new Map())
    }

    addMetrics(totals, item)
    addMetrics(token, item)

    const channelName =
      item.channel_name ||
      (item.channel_id ? `渠道 #${item.channel_id}` : '未记录渠道')
    const channelKey = identityKey(item.channel_id, channelName)
    const modelMapKey = identityKey(tokenKey, channelKey)
    const channelsForToken = channelMaps.get(tokenKey)
    if (!channelsForToken) continue

    let channel = channelsForToken.get(channelKey)
    if (!channel) {
      channel = {
        ...createMetrics(),
        id: item.channel_id,
        name: channelName,
        key: channelKey,
        models: [],
      }
      channelsForToken.set(channelKey, channel)
      token.channels.push(channel)
      modelMaps.set(modelMapKey, new Map())
    }

    addMetrics(channel, item)

    const modelName = item.model_name || '未记录模型'
    const modelsForChannel = modelMaps.get(modelMapKey)
    if (!modelsForChannel) continue

    let model = modelsForChannel.get(modelName)
    if (!model) {
      model = {
        ...createMetrics(),
        name: modelName,
      }
      modelsForChannel.set(modelName, model)
      channel.models.push(model)
    }
    addMetrics(model, item)
  }

  const tokens = sortByTotalTokens([...tokenMap.values()])
  for (const token of tokens) {
    sortByTotalTokens(token.channels)
    for (const channel of token.channels) sortByTotalTokens(channel.models)
  }

  return { totals, tokens }
}

export function selectDefaultToken(
  tokens: UsageTokenBucket[],
  selectedKey?: string
): UsageTokenBucket | null {
  if (tokens.length === 0) return null
  if (selectedKey) {
    const selected = tokens.find((token) => token.key === selectedKey)
    if (selected) return selected
  }
  return tokens[0]
}

/**
 * 解析当前渠道；令牌或时间范围变化后，失效的 key 必须回退到最高用量渠道。
 */
export function selectDefaultChannel(
  token: UsageTokenBucket | null,
  selectedKey?: string
): UsageChannelBucket | null {
  if (!token || token.channels.length === 0) return null
  if (selectedKey) {
    const selected = token.channels.find(
      (channel) => channel.key === selectedKey
    )
    if (selected) return selected
  }
  return token.channels[0]
}
