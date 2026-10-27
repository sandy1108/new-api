import { useQuery } from '@tanstack/react-query'
import { Search } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { SectionPageLayout } from '@/components/layout/components/section-page-layout'
import { LoadingState } from '@/components/loading-state'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { SettingsCard } from '@/features/system-settings/components/settings-card'

import { getReportGrants } from './api'
import { GrantCard } from './components/grant-card'
import { useReportAccess } from './hooks'

// 独立Root设置页，不复用用户编辑/角色管理，防止观察授权扩大为管理员权限。
export function ReportAccessSettings() {
  const { t } = useTranslation()
  const access = useReportAccess()
  const [page, setPage] = useState(1)
  const [search, setSearch] = useState('')
  const [input, setInput] = useState('')
  const users = useQuery({
    queryKey: ['pong-report-grants', access.identity, page, search],
    queryFn: ({ signal }) => getReportGrants(page, search, signal),
    enabled: access.canManage,
  })
  const pending = access.isPending || (access.canManage && users.isPending)
  const failed = access.isError || users.isError
  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        {t('Report access grants')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='min-w-0 space-y-5 pb-4'>
          <SettingsCard
            title={t('Account report access')}
            description={t(
              'Grant read-only access to all accounts in the selected reports. This does not grant administrator permissions.'
            )}
          >
            <form
              className='space-y-3'
              onSubmit={(event) => {
                event.preventDefault()
                setSearch(input.trim())
                setPage(1)
              }}
            >
              <div className='flex flex-col gap-3 sm:flex-row sm:items-end'>
                <div className='min-w-0 flex-1 space-y-2 sm:max-w-md'>
                  <Label htmlFor='report-account-search'>
                    {t('Username or user ID')}
                  </Label>
                  <Input
                    id='report-account-search'
                    placeholder={t('Username or user ID')}
                    value={input}
                    onChange={(event) => setInput(event.target.value)}
                    maxLength={80}
                  />
                </div>
                <div className='flex gap-2'>
                  <Button type='submit' disabled={pending || users.isFetching}>
                    <Search aria-hidden='true' className='size-4' />
                    {t('Search')}
                  </Button>
                  {(input || search) && (
                    <Button
                      type='button'
                      variant='outline'
                      onClick={() => {
                        setInput('')
                        setSearch('')
                        setPage(1)
                      }}
                    >
                      {t('Clear search')}
                    </Button>
                  )}
                </div>
              </div>
              <p className='text-muted-foreground text-xs leading-relaxed'>
                {t(
                  'Only enabled ordinary accounts are listed. Create an account in Users first, then grant access here.'
                )}
              </p>
            </form>
          </SettingsCard>
          {pending && !failed && <LoadingState />}
          {failed && (
            <ErrorState
              onRetry={() => {
                void access.refetch()
                void users.refetch()
              }}
            />
          )}
          {!pending && !failed && access.canManage && users.data && (
            <>
              {users.data.total === 0 ? (
                <EmptyState
                  title={t('No eligible accounts')}
                  description={t(
                    'Check the exact username or user ID, or create and enable an ordinary account in Users.'
                  )}
                  bordered
                />
              ) : (
                <div className='space-y-4'>
                  {users.data.items.map((user) => (
                    <GrantCard
                      key={`${access.identity}/${user.id}/${user.version}/${user.invalidated}`}
                      user={user}
                    />
                  ))}
                </div>
              )}
              <div className='flex flex-wrap items-center justify-between gap-3 border-t pt-4'>
                <p className='text-muted-foreground text-sm'>
                  {t('Total')}: {users.data.total} · {t('Page')} {page}
                </p>
                <div className='flex gap-2'>
                  <Button
                    variant='outline'
                    disabled={page <= 1 || users.isFetching}
                    onClick={() => setPage(page - 1)}
                  >
                    {t('Previous')}
                  </Button>
                  <Button
                    variant='outline'
                    disabled={page * 20 >= users.data.total || users.isFetching}
                    onClick={() => setPage(page + 1)}
                  >
                    {t('Next')}
                  </Button>
                </div>
              </div>
            </>
          )}
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
