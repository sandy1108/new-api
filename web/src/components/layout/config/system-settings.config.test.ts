import type { TFunction } from 'i18next'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { ROLE } from '@/lib/roles'

import { getNavGroupsForPath } from '../lib/sidebar-view-registry'

const authStore = vi.hoisted(() => ({ getState: vi.fn() }))

vi.mock('@/stores/auth-store', () => ({ useAuthStore: authStore }))

const t = ((key: string) => key) as unknown as TFunction

describe('system settings navigation role visibility', () => {
  beforeEach(() => vi.clearAllMocks())

  it('hides client access credentials from non-root administrators', () => {
    authStore.getState.mockReturnValue({ auth: { user: { role: ROLE.ADMIN } } })

    const groups = getNavGroupsForPath('/system-settings/site', t) ?? []

    expect(
      groups
        .flatMap((group) => group.items)
        .some((item) => item.title === '客户端访问凭据')
    ).toBe(false)
  })

  it('shows client access credentials to Root', () => {
    authStore.getState.mockReturnValue({
      auth: { user: { role: ROLE.SUPER_ADMIN } },
    })

    const groups = getNavGroupsForPath('/system-settings/site', t) ?? []

    expect(
      groups
        .flatMap((group) => group.items)
        .some((item) => item.title === '客户端访问凭据')
    ).toBe(true)
  })
})
