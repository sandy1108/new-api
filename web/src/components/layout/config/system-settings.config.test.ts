import type { TFunction } from 'i18next'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { ROLE } from '@/lib/roles'

import { resolveSidebarView } from '../lib/sidebar-view-registry'

const authStore = vi.hoisted(() => ({ getState: vi.fn() }))

vi.mock('@/stores/auth-store', () => ({ useAuthStore: authStore }))

const t = ((key: string) => key) as unknown as TFunction

describe('system settings navigation role visibility', () => {
  beforeEach(() => vi.clearAllMocks())

  it('hides client access credentials from non-root administrators', () => {
    authStore.getState.mockReturnValue({ auth: { user: { role: ROLE.ADMIN } } })

    const groups =
      resolveSidebarView('/system-settings/site')?.getNavGroups(t) ?? []

    expect(
      groups
        .flatMap((group) => group.items)
        .some((item) => item.url === '/system-settings/client-access')
    ).toBe(false)
  })

  it('shows client access credentials to Root', () => {
    authStore.getState.mockReturnValue({
      auth: { user: { role: ROLE.SUPER_ADMIN } },
    })

    const groups =
      resolveSidebarView('/system-settings/site')?.getNavGroups(t) ?? []

    expect(
      groups
        .flatMap((group) => group.items)
        .some((item) => item.url === '/system-settings/client-access')
    ).toBe(true)
  })
})
