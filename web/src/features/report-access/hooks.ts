import {
  useQuery,
  useQueryClient,
  type QueryClient,
  type QueryKey,
} from '@tanstack/react-query'
import { CanceledError, isAxiosError } from 'axios'
import { useEffect } from 'react'

import { useAuthStore } from '@/stores/auth-store'

import { getReportCapabilities, type ReportCapabilities } from './api'

const accessKey = ['pong-report-access'] as const
export function reportIdentity(): string {
  const { user, session } = useAuthStore.getState().auth
  return `${user?.id ?? 0}/${session?.sid ?? ''}/${user?.role ?? 0}`
}

export function useReportAccess() {
  const user = useAuthStore((state) => state.auth.user)
  const sid = useAuthStore((state) => state.auth.session?.sid)
  const identity = `${user?.id ?? 0}/${sid ?? ''}/${user?.role ?? 0}`
  const client = useQueryClient()
  const query = useQuery({
    queryKey: [...accessKey, identity],
    queryFn: async ({ signal }) => {
      const result = await getReportCapabilities(signal)
      // API即使忽略AbortSignal，换号后的迟到响应也不得用于当前身份。
      if (reportIdentity() !== identity) throw new CanceledError()
      return result
    },
    enabled: Boolean(user),
    staleTime: 5_000,
    refetchInterval: 30_000,
    refetchOnWindowFocus: 'always',
    retry: false,
  })
  const caps = query.isError ? undefined : query.data
  const revision = caps?.revision ?? 'unverified'
  useEffect(() => {
    const stale = {
      predicate: (item: { queryKey: QueryKey }) =>
        item.queryKey[0] === 'pong-report-data' &&
        (item.queryKey[1] !== identity || item.queryKey[2] !== revision),
    }
    void client.cancelQueries(stale)
    client.removeQueries(stale)
  }, [client, identity, revision])
  return {
    ...query,
    identity,
    revision,
    canReadUsageAll: Boolean(user && caps?.usage_read_all),
    canReadLedger: Boolean(user && caps?.subscription_read_all),
    canManage: Boolean(user?.role === 100 && caps?.can_manage_grants),
  }
}

export function reportReadFailure(client: QueryClient, error: unknown): never {
  const status = isAxiosError(error)
    ? error.response?.status
    : (error as { response?: { status?: number } })?.response?.status
  if (status === 403 || status === 401) {
    client.setQueriesData<ReportCapabilities>(
      { queryKey: accessKey },
      (previous) =>
        previous
          ? {
              ...previous,
              usage_read_all: false,
              subscription_read_all: false,
              can_manage_grants: false,
              revision: 'recheck',
            }
          : previous
    )
    void client.invalidateQueries({ queryKey: accessKey })
  }
  throw error
}

export function useLedgerRead<T>(
  key: QueryKey,
  read: (signal: AbortSignal) => Promise<T>
) {
  const access = useReportAccess()
  const client = useQueryClient()
  const query = useQuery({
    queryKey: [
      'pong-report-data',
      access.identity,
      access.revision,
      'ledger',
      ...key,
    ],
    queryFn: async ({ signal }) => {
      try {
        const result = await read(signal)
        if (reportIdentity() !== access.identity) throw new CanceledError()
        return result
      } catch (error) {
        if (reportIdentity() !== access.identity) throw new CanceledError()
        return reportReadFailure(client, error)
      }
    },
    enabled: access.canReadLedger,
    refetchInterval: 60_000,
    retry: false,
  })
  return {
    ...query,
    data: access.canReadLedger && !query.isError ? query.data : undefined,
  }
}
