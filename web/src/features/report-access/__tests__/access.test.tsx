import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  act,
  fireEvent,
  render,
  screen,
  renderHook,
  waitFor,
} from '@testing-library/react'
import { createInstance } from 'i18next'
import { createElement, type ReactNode } from 'react'
import { afterEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { ReportAccessSettings } from '..'
import { useReportAccess, useLedgerRead } from '../hooks'

vi.mock('@/lib/api', () => ({ api: { get: vi.fn(), put: vi.fn() } }))
const verify = vi.hoisted(() => vi.fn())
vi.mock('@/features/auth/secure-verification', () => ({
  useSecureVerification: () => ({
    requestVerification: verify,
    dialogProps: {},
  }),
  SecureVerificationDialog: () => null,
}))
const get = vi.mocked(api.get)
const capabilities = {
  usage_read_all: false,
  subscription_read_all: true,
  can_manage_grants: false,
  revision: 'observer-1',
}
function wrapper(client: QueryClient) {
  return (props: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client }, props.children)
}
afterEach(() => {
  useAuthStore.getState().auth.reset()
  vi.resetAllMocks()
})

test('普通账户单项授权只解锁账本，且不授予管理能力', async () => {
  useAuthStore.getState().auth.setUser({ id: 2, username: 'viewer', role: 1 })
  get.mockResolvedValue({ data: { success: true, data: capabilities } })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const hook = renderHook(() => useReportAccess(), { wrapper: wrapper(client) })
  await waitFor(() => expect(hook.result.current.canReadLedger).toBe(true))
  expect(hook.result.current.canReadUsageAll).toBe(false)
  expect(hook.result.current.canManage).toBe(false)
  expect(useAuthStore.getState().auth.user?.role).toBe(1)
  client.clear()
})

test('换号时立即隐藏旧报表，前一账户的迟到响应不能进入新账户', async () => {
  useAuthStore.getState().auth.setUser({ id: 2, username: 'first', role: 1 })
  get.mockResolvedValue({ data: { success: true, data: capabilities } })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  let resolve!: (value: string) => void
  const pending = new Promise<string>((done) => {
    resolve = done
  })
  const hook = renderHook(() => useLedgerRead(['test'], async () => pending), {
    wrapper: wrapper(client),
  })
  await waitFor(() => expect(hook.result.current.fetchStatus).toBe('fetching'))
  get.mockResolvedValue({
    data: {
      success: true,
      data: {
        ...capabilities,
        subscription_read_all: false,
        revision: 'denied',
      },
    },
  })
  act(() =>
    useAuthStore.getState().auth.setUser({ id: 3, username: 'second', role: 1 })
  )
  await act(async () => {
    resolve('first-account-private-report')
    await pending
  })
  expect(hook.result.current.data).toBeUndefined()
  client.clear()
})

test('报表403后清掉已显示结果且重新核验能力', async () => {
  useAuthStore.getState().auth.setUser({ id: 2, username: 'viewer', role: 1 })
  get.mockResolvedValue({ data: { success: true, data: capabilities } })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  let denied = false
  const hook = renderHook(
    () =>
      useLedgerRead(['test'], async () => {
        if (denied) throw { response: { status: 403 } }
        return 'private-report'
      }),
    { wrapper: wrapper(client) }
  )
  await waitFor(() => expect(hook.result.current.data).toBe('private-report'))
  denied = true
  get.mockResolvedValue({
    data: {
      success: true,
      data: {
        ...capabilities,
        subscription_read_all: false,
        revision: 'revoked',
      },
    },
  })
  await act(async () => {
    await hook.result.current.refetch()
  })
  await waitFor(() => expect(hook.result.current.data).toBeUndefined())
  client.clear()
})

test('Root勾选两项授权时，将同一目标、勾选和版本绑定proof及保存请求', async () => {
  useAuthStore.getState().auth.setUser({ id: 1, username: 'root', role: 100 })
  get.mockImplementation(async (url) => ({
    data: {
      success: true,
      data: url.endsWith('/capabilities')
        ? { ...capabilities, usage_read_all: true, can_manage_grants: true }
        : {
            items: [
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
            ],
            total: 1,
          },
    },
  }))
  vi.mocked(api.put).mockResolvedValue({
    data: { success: true, data: { version: 5 } },
  })
  verify.mockResolvedValue({ proof_token: 'synthetic-one-use-proof' })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const view = render(
    createElement(
      QueryClientProvider,
      { client },
      createElement(ReportAccessSettings)
    )
  )
  fireEvent.click(
    await screen.findByRole('switch', { name: 'All-account token usage' })
  )
  fireEvent.click(
    screen.getByRole('switch', { name: 'All-account subscription quota' })
  )
  fireEvent.click(screen.getByRole('button', { name: 'Save' }))
  await waitFor(() =>
    expect(api.put).toHaveBeenCalledWith(
      '/api/pong-report-access/users/2',
      { usage_read_all: true, subscription_read_all: true, version: 4 },
      expect.objectContaining({
        headers: {
          'X-PongAPI-Request': '1',
          'X-Security-Proof': 'synthetic-one-use-proof',
        },
        singleUseAuthorization: true,
      })
    )
  )
  expect(verify).toHaveBeenCalledWith(
    expect.objectContaining({
      scope: 'pong.report_grants.update',
      context: {
        user_id: 2,
        usage_read_all: true,
        subscription_read_all: true,
        version: 4,
      },
    })
  )
  view.unmount()
  client.clear()
})

// 使用真实语言文件及i18next实例，避免只验证mock文案而漏掉命名空间层级。
test('报表授权文案位于实际translation命名空间，中文可被i18next读取', async () => {
  const locales = import.meta.glob('../../../i18n/locales/*.json', {
    eager: true,
    import: 'default',
  }) as Record<string, { translation: Record<string, string> }>
  for (const locale of Object.values(locales)) {
    for (const key of [
      'Account report access',
      'Only enabled ordinary accounts are listed. Create an account in Users first, then grant access here.',
      'No eligible accounts',
      'Check the exact username or user ID, or create and enable an ordinary account in Users.',
      'Saved',
      'View usage and costs for all accounts. No raw logs or API keys.',
      'View monitored subscriptions and quota history. No collection or configuration changes.',
      'Changes take effect only after saving and identity verification.',
    ]) {
      expect(locale.translation[key]).toBeTruthy()
    }
    expect(locale.translation['Report access grants']).toBeTruthy()
    expect(locale.translation['All-account token usage']).toBeTruthy()
    expect(locale.translation['All-account subscription quota']).toBeTruthy()
  }
  const instance = createInstance()
  await instance.init({
    lng: 'zh',
    resources: { zh: locales['../../../i18n/locales/zh.json'] },
  })
  expect(instance.t('Report access grants')).toBe('报表只读授权')
})
