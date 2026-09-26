import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { ClientAccessCredentials } from '..'
import * as requests from '../api'

vi.mock('../api', () => ({
  listScopes: vi.fn(), listResources: vi.fn(), listCredentials: vi.fn(),
  getCredential: vi.fn(), createCredential: vi.fn(), updateGrants: vi.fn(),
  credentialAction: vi.fn(),
}))

const credential = { id: 1, name: 'dashboard', token_prefix: 'napi_client_abc', status: 1, version: 2, expired_time: -1, created_time: 1, last_used_time: 0 }

beforeEach(() => {
  vi.mocked(requests.listScopes).mockResolvedValue([{ key: 'codex.quota.read', display_name: 'Codex 额度读取', description: '只读额度', resource_type: 'channel', max_resources: 1 }])
  vi.mocked(requests.listResources).mockResolvedValue([{ id: 17, name: 'ChatGPT Plus', type: 57 }])
  vi.mocked(requests.listCredentials).mockResolvedValue({ items: [credential], total: 1 })
  vi.mocked(requests.getCredential).mockResolvedValue({ credential, grants: [{ scope: 'codex.quota.read', resource_id: '17' }] })
  vi.mocked(requests.createCredential).mockResolvedValue({ credential, token: 'napi_client_one-time-secret' })
})
afterEach(() => vi.clearAllMocks())

describe('客户端访问凭据页面', () => {
  it('只列出服务端筛选的渠道，并在创建后一次性显示新凭据', async () => {
    render(<ClientAccessCredentials />)
    await screen.findByText('Codex 额度读取')
    fireEvent.change(screen.getByLabelText('名称'), { target: { value: 'phone' } })
    fireEvent.click(screen.getByText('Codex 额度读取'))
    fireEvent.change(screen.getByLabelText('授权渠道'), { target: { value: '17' } })
    fireEvent.click(screen.getByRole('button', { name: '创建凭据' }))
    await waitFor(() => expect(requests.createCredential).toHaveBeenCalledWith({
      name: 'phone', expired_time: -1,
      grants: [{ scope: 'codex.quota.read', resource_id: '17' }],
    }))
    expect(screen.getByLabelText('新客户端凭据')).toHaveValue('napi_client_one-time-secret')
    fireEvent.click(screen.getByRole('button', { name: '我已保存，关闭' }))
    expect(screen.queryByLabelText('新客户端凭据')).not.toBeInTheDocument()
  })

  it('编辑时使用详情中的 Grant 和乐观版本，冲突后要求重新打开', async () => {
    vi.mocked(requests.updateGrants).mockRejectedValue({ response: { status: 409 } })
    render(<ClientAccessCredentials />)
    fireEvent.click(await screen.findByRole('button', { name: '授权' }))
    await screen.findByText('编辑授权：dashboard')
    expect(screen.getByLabelText('授权渠道')).toHaveValue('17')
    fireEvent.click(screen.getByRole('button', { name: '保存授权' }))
    await waitFor(() => expect(requests.updateGrants).toHaveBeenCalledWith(1, 2, [
      { scope: 'codex.quota.read', resource_id: '17' },
    ]))
    await waitFor(() => expect(screen.queryByText('编辑授权：dashboard')).not.toBeInTheDocument())
  })
})
