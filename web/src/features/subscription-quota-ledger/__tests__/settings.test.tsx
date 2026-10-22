import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  createMemoryHistory,
  createRootRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { QuotaSettings } from '../components/settings'
import { SubscriptionQuotaSettings } from '../settings-page'

let client: QueryClient
afterEach(() => {
  cleanup()
  client?.clear()
  vi.restoreAllMocks()
})
test('settings save uses current version and CSRF header; conflict is not reported as success', async () => {
  client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  vi.spyOn(api, 'get').mockImplementation(async (url) => ({
    data: {
      success: true,
      data: url.endsWith('/settings')
        ? { enabled: false, interval_minutes: 10, version: 7, updated_at: 0 }
        : { items: [], total: 0 },
    },
  }))
  vi.spyOn(api, 'put').mockResolvedValue({
    data: {
      success: false,
      message: 'Version conflict',
      code: 'PRIVATE_QUOTA_VERSION_CONFLICT',
    },
  })
  render(
    <QueryClientProvider client={client}>
      <QuotaSettings
        subjects={[]}
        subjectPage={1}
        total={0}
        onPage={() => {}}
      />
    </QueryClientProvider>
  )
  const interval = await screen.findByLabelText('Polling interval (minutes)')
  expect(
    screen.getByRole('combobox', { name: 'Source channel' })
  ).toHaveTextContent('Source channel')
  fireEvent.change(interval, { target: { value: '20' } })
  fireEvent.click(
    screen.getByRole('button', { name: 'Save collection settings' })
  )
  await waitFor(() =>
    expect(api.put).toHaveBeenCalledWith(
      '/api/private-quota/settings',
      { enabled: false, interval_minutes: 20, version: 7 },
      { headers: { 'X-PongAPI-Request': '1' } }
    )
  )
  await screen.findByRole('alert')
  expect(screen.getByRole('alert')).toHaveTextContent('Save failed')
})

test('standalone settings page shows collection and subscriptions without ledger tabs or month filter', async () => {
  client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  vi.spyOn(api, 'get').mockImplementation(async (url) => ({
    data: {
      success: true,
      data: url.endsWith('/settings')
        ? { enabled: true, interval_minutes: 10, version: 1, updated_at: 0 }
        : { items: [], total: 0 },
    },
  }))
  render(
    <QueryClientProvider client={client}>
      <RouterProvider
        router={createRouter({
          routeTree: createRootRoute({ component: SubscriptionQuotaSettings }),
          history: createMemoryHistory({ initialEntries: ['/'] }),
        })}
      />
    </QueryClientProvider>
  )
  expect(
    await screen.findByRole('switch', { name: 'Enable quota collection' })
  ).toBeChecked()
  expect(
    screen.getByRole('heading', { name: 'Subscription quota ledger settings' })
  ).toBeInTheDocument()
  expect(screen.getByText('Monitored subscriptions')).toBeInTheDocument()
  expect(
    screen.getByRole('link', { name: 'Subscription quota ledger' })
  ).toHaveAttribute('href', '/subscription-quota')
  expect(screen.queryByRole('tab')).not.toBeInTheDocument()
  expect(
    screen.queryByLabelText('Month (Asia/Shanghai)')
  ).not.toBeInTheDocument()
})
