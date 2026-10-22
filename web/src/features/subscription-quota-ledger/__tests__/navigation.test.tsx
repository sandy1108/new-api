import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from '@tanstack/react-router'
import { afterEach, expect, test } from 'vitest'

import { ROLE } from '@/lib/roles'
import { Route as LedgerRoute } from '@/routes/_authenticated/subscription-quota'
import { Route as SettingsRoute } from '@/routes/_authenticated/system-settings/subscription-quota'
import { useAuthStore } from '@/stores/auth-store'

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
