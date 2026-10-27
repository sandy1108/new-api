import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from '@tanstack/react-router'
import { afterEach, expect, test, vi } from 'vitest'

import { ROLE } from '@/lib/roles'
import { Route as LedgerRoute } from '@/routes/_authenticated/subscription-quota'
import { Route as SettingsRoute } from '@/routes/_authenticated/system-settings/subscription-quota'
import { useAuthStore } from '@/stores/auth-store'

let observer = false
vi.mock('@/features/report-access/api', () => ({
  getReportCapabilities: vi.fn(async () => ({
    usage_read_all: false,
    subscription_read_all:
      observer || useAuthStore.getState().auth.user?.role === 100,
    can_manage_grants: false,
    revision: 'route-test',
  })),
}))
afterEach(() => {
  observer = false
})
afterEach(() => useAuthStore.getState().auth.reset())

async function openQuotaRoute(url: string, role: number = ROLE.SUPER_ADMIN) {
  useAuthStore.getState().auth.setUser({ id: 1, username: 'tester', role })
  const root = createRootRoute()
  // 使用真实路由的参数校验和权限/兼容跳转，只省去与导航无关的页面渲染。
  const ledger = createRoute({
    getParentRoute: () => root,
    path: '/subscription-quota',
    validateSearch: LedgerRoute.options.validateSearch,
  })
  const settings = createRoute({
    getParentRoute: () => root,
    path: '/system-settings/subscription-quota',
    validateSearch: SettingsRoute.options.validateSearch,
  })
  // 测试父布局只提供路由运行时；真实钩子不依赖父布局上下文，仅使用search/全局登录态。
  ledger.options.beforeLoad = LedgerRoute.options
    .beforeLoad as typeof ledger.options.beforeLoad
  settings.options.beforeLoad = SettingsRoute.options
    .beforeLoad as typeof settings.options.beforeLoad
  const forbidden = createRoute({ getParentRoute: () => root, path: '/403' })
  const router = createRouter({
    routeTree: root.addChildren([ledger, settings, forbidden]),
    history: createMemoryHistory({ initialEntries: [url] }),
  })
  await router.load()
  return router
}

test.each(['overview', 'history'])(
  'legacy %s bookmark redirects to the same console tab',
  async (view) => {
    const router = await openQuotaRoute(
      `/system-settings/subscription-quota?view=${view}`
    )
    expect(router.state.location.pathname).toBe('/subscription-quota')
    expect(router.state.location.search).toMatchObject({ view })
  }
)

test.each(['', '?view=settings', '?view=invalid'])(
  'settings entry %s stays in system settings',
  async (search) => {
    const router = await openQuotaRoute(
      `/system-settings/subscription-quota${search}`
    )
    expect(router.state.location.pathname).toBe(
      '/system-settings/subscription-quota'
    )
    expect(router.state.matches.at(-1)?.status).toBe('success')
  }
)

test('invalid console tab falls back to overview', async () => {
  const router = await openQuotaRoute('/subscription-quota?view=settings')
  expect(router.state.matches.at(-1)?.search).toMatchObject({
    view: 'overview',
  })
})

test.each([
  ['/subscription-quota', ROLE.USER],
  ['/subscription-quota?view=history', ROLE.ADMIN],
  ['/system-settings/subscription-quota', ROLE.ADMIN],
  ['/system-settings/subscription-quota?view=overview', ROLE.USER],
])(
  'non-root access to %s redirects to forbidden (role %s)',
  async (url, role) => {
    const router = await openQuotaRoute(url, role)
    expect(router.state.location.pathname).toBe('/403')
  }
)

test('账本观察者可打开报表，但仍不能打开Root配置页', async () => {
  observer = true
  const ledger = await openQuotaRoute(
    '/subscription-quota?view=history',
    ROLE.USER
  )
  expect(ledger.state.location.pathname).toBe('/subscription-quota')
  expect(ledger.state.matches.at(-1)?.status).toBe('success')
  const settings = await openQuotaRoute(
    '/system-settings/subscription-quota',
    ROLE.USER
  )
  expect(settings.state.location.pathname).toBe('/403')
})
