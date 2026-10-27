import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { ReportAccessSettings } from '..'
import type { ReportGrant } from '../api'

vi.mock('@/lib/api', () => ({ api: { get: vi.fn(), put: vi.fn() } }))
vi.mock('@/lib/handle-server-error', () => ({ handleServerError: vi.fn() }))
const verify = vi.hoisted(() => vi.fn())
vi.mock('@/features/auth/secure-verification', () => ({
  useSecureVerification: () => ({
    requestVerification: verify,
    dialogProps: {},
  }),
  SecureVerificationDialog: () => null,
}))
const get = vi.mocked(api.get)
const put = vi.mocked(api.put)
let client: QueryClient
let items: ReportGrant[]
let total: number

beforeEach(() => {
  useAuthStore.getState().auth.setUser({ id: 1, username: 'root', role: 100 })
  items = [
    {
      id: 2,
      username: 'viewer',
      display_name: 'Viewer',
      usage_read_all: false,
      subscription_read_all: false,
      version: 4,
      invalidated: false,
      updated_at: 0,
    },
  ]
  total = 1
  get.mockImplementation(async (url) => ({
    data: {
      success: true,
      data: url.endsWith('/capabilities')
        ? {
            usage_read_all: true,
            subscription_read_all: true,
            can_manage_grants: true,
            revision: 'root-1',
          }
        : { items: structuredClone(items), total },
    },
  }))
  verify.mockResolvedValue({ proof_token: 'synthetic-proof' })
  put.mockImplementation(async () => ({
    data: { success: true, data: { version: 5 } },
  }))
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
})
afterEach(() => {
  cleanup()
  client.clear()
  useAuthStore.getState().auth.reset()
  vi.resetAllMocks()
})

function show() {
  return render(
    <QueryClientProvider client={client}>
      <ReportAccessSettings />
    </QueryClientProvider>
  )
}

test('无变更时禁用保存，修改权限只显示未保存而不会直接写入', async () => {
  show()
  const region = await screen.findByRole('region', { name: 'Viewer' })
  expect(within(region).getByRole('button', { name: 'Save' })).toBeDisabled()
  fireEvent.click(
    within(region).getByRole('switch', { name: 'All-account token usage' })
  )
  expect(within(region).getByText('Unsaved changes')).toBeVisible()
  expect(within(region).getByRole('button', { name: 'Save' })).toBeEnabled()
  expect(put).not.toHaveBeenCalled()
})

test('点击权限名称可切换对应选项，取消后恢复已保存值', async () => {
  show()
  const region = await screen.findByRole('region', { name: 'Viewer' })
  fireEvent.click(within(region).getByText('All-account subscription quota'))
  expect(
    within(region).getByRole('switch', {
      name: 'All-account subscription quota',
    })
  ).toBeChecked()
  fireEvent.click(within(region).getByRole('button', { name: 'Cancel' }))
  expect(
    within(region).getByRole('switch', {
      name: 'All-account subscription quota',
    })
  ).not.toBeChecked()
  expect(within(region).getByRole('button', { name: 'Save' })).toBeDisabled()
  expect(verify).not.toHaveBeenCalled()
})

test('多账户分别编辑时保存仅绑定目标账户与最终勾选，验证中禁止再次修改', async () => {
  items.push({ ...items[0], id: 3, username: 'other', display_name: 'Other' })
  let resolve!: (value: { proof_token: string }) => void
  verify.mockReturnValue(
    new Promise((done) => {
      resolve = done
    })
  )
  show()
  const viewer = await screen.findByRole('region', { name: 'Viewer' })
  const other = screen.getByRole('region', { name: 'Other' })
  fireEvent.click(
    within(viewer).getByRole('switch', { name: 'All-account token usage' })
  )
  fireEvent.click(within(viewer).getByRole('button', { name: 'Save' }))
  await waitFor(() => expect(verify).toHaveBeenCalled())
  expect(
    within(viewer).getByRole('switch', { name: 'All-account token usage' })
  ).toHaveAttribute('aria-disabled', 'true')
  expect(
    within(other).getByRole('switch', { name: 'All-account token usage' })
  ).toBeEnabled()
  expect(verify).toHaveBeenCalledWith(
    expect.objectContaining({
      context: {
        user_id: 2,
        usage_read_all: true,
        subscription_read_all: false,
        version: 4,
      },
    })
  )
  items[0] = { ...items[0], usage_read_all: true, version: 5 }
  await act(async () => resolve({ proof_token: 'synthetic-proof' }))
  await waitFor(() =>
    expect(put).toHaveBeenCalledWith(
      '/api/pong-report-access/users/2',
      expect.objectContaining({
        usage_read_all: true,
        subscription_read_all: false,
        version: 4,
      }),
      expect.any(Object)
    )
  )
  await waitFor(() =>
    expect(
      within(screen.getByRole('region', { name: 'Viewer' })).getByRole(
        'button',
        { name: 'Save' }
      )
    ).toBeDisabled()
  )
})

test('二次验证取消后保留草稿且允许再次保存，不发送授权请求', async () => {
  verify.mockResolvedValue(null)
  show()
  const region = await screen.findByRole('region', { name: 'Viewer' })
  fireEvent.click(
    within(region).getByRole('switch', { name: 'All-account token usage' })
  )
  fireEvent.click(within(region).getByRole('button', { name: 'Save' }))
  await waitFor(() =>
    expect(within(region).getByRole('button', { name: 'Save' })).toBeEnabled()
  )
  expect(
    within(region).getByRole('switch', { name: 'All-account token usage' })
  ).toBeChecked()
  expect(put).not.toHaveBeenCalled()
})

test('安全版本失效的授权原值也可以重新保存', async () => {
  items[0] = { ...items[0], usage_read_all: true, invalidated: true }
  show()
  const region = await screen.findByRole('region', { name: 'Viewer' })
  expect(
    within(region).getByText(
      'Account security changed. Save again to renew these grants.'
    )
  ).toBeVisible()
  expect(within(region).getByRole('button', { name: 'Save' })).toBeEnabled()
})

test('搜索仅在提交后生效，清空搜索恢复完整列表', async () => {
  show()
  await screen.findByRole('region', { name: 'Viewer' })
  fireEvent.change(
    screen.getByRole('textbox', { name: 'Username or user ID' }),
    { target: { value: 'viewer' } }
  )
  expect(get).not.toHaveBeenCalledWith(
    '/api/pong-report-access/users',
    expect.objectContaining({
      params: expect.objectContaining({ search: 'viewer' }),
    })
  )
  fireEvent.click(screen.getByRole('button', { name: /^Search$/ }))
  await waitFor(() =>
    expect(get).toHaveBeenCalledWith(
      '/api/pong-report-access/users',
      expect.objectContaining({
        params: expect.objectContaining({ search: 'viewer' }),
      })
    )
  )
  fireEvent.click(screen.getByRole('button', { name: 'Clear search' }))
  await waitFor(() =>
    expect(
      screen.getByRole('textbox', { name: 'Username or user ID' })
    ).toHaveValue('')
  )
})

test('没有普通账号时显示引导空状态而不是无说明的空白', async () => {
  items = []
  total = 0
  show()
  expect(await screen.findByText('No eligible accounts')).toBeVisible()
  expect(screen.queryByRole('button', { name: 'Save' })).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Next' })).toBeDisabled()
})

test('授权列表请求失败时显示错误重试，不同时显示加载占位', async () => {
  get.mockImplementation(async (url) => {
    if (!url.endsWith('/capabilities')) throw new Error('test-list-unavailable')
    return {
      data: {
        success: true,
        data: {
          usage_read_all: true,
          subscription_read_all: true,
          can_manage_grants: true,
          revision: 'root-1',
        },
      },
    }
  })
  show()
  expect(await screen.findByText('Oops! Something went wrong')).toBeVisible()
  expect(screen.queryByText('Loading...')).not.toBeInTheDocument()
  expect(screen.queryByRole('button', { name: 'Save' })).not.toBeInTheDocument()
})
