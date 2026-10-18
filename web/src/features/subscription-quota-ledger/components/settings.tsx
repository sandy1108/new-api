import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import type { z } from 'zod'

import { StaticDataTable } from '@/components/data-table'
import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { toIntlLocale } from '@/i18n/languages'
import { handleServerError } from '@/lib/handle-server-error'

import {
  getChannels,
  getSettings,
  quotaKey,
  saveSettings,
  saveSubject,
} from '../api'
import { ledgerTime, settingsSchema, subjectSchema } from '../lib/forms'
import type { Settings, Subject } from '../types'
import { QuotaSelect } from './quota-select'

export function QuotaSettings(props: {
  subjects: Subject[]
  subjectPage: number
  total: number
  onPage: (page: number) => void
}) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const [editing, setEditing] = useState<Subject | null>(null)
  const [channelPage, setChannelPage] = useState(1)
  const settings = useQuery({
    queryKey: [...quotaKey, 'settings'],
    queryFn: getSettings,
  })
  const channels = useQuery({
    queryKey: [...quotaKey, 'channels', channelPage],
    queryFn: () => getChannels(channelPage),
  })
  if (settings.isPending || channels.isPending) return <LoadingState />
  if (settings.isError || channels.isError) {
    return (
      <ErrorState
        onRetry={() => {
          void settings.refetch()
          void channels.refetch()
        }}
      />
    )
  }
  return (
    <div className='space-y-4'>
      <GlobalSettings key={settings.data.version} settings={settings.data} />
      <Card>
        <CardHeader>
          <CardTitle>{t('Monitored subscriptions')}</CardTitle>
        </CardHeader>
        <CardContent className='space-y-4'>
          <StaticDataTable
            data={props.subjects}
            getRowKey={(row) => row.id}
            emptyContent={t('No Data')}
            columns={[
              { id: 'name', header: t('Name'), cell: (row) => row.name },
              {
                id: 'enabled',
                header: t('Collection'),
                cell: (row) => (row.enabled ? t('Enabled') : t('Paused')),
              },
              {
                id: 'last',
                header: t('Last observation'),
                cell: (row) => ledgerTime(row.last_success_at, locale),
              },
              {
                id: 'error',
                header: t('Status'),
                cell: (row) => row.error_code || '—',
              },
              {
                id: 'action',
                header: t('Actions'),
                cell: (row) => (
                  <Button
                    variant='outline'
                    size='sm'
                    onClick={() => setEditing(row)}
                  >
                    {t('Edit')} · {row.name}
                  </Button>
                ),
              },
            ]}
          />
          <div className='flex gap-2'>
            <Button
              variant='outline'
              disabled={props.subjectPage === 1}
              onClick={() => props.onPage(props.subjectPage - 1)}
            >
              {t('Previous')}
            </Button>
            <Button
              variant='outline'
              disabled={props.subjectPage * 100 >= props.total}
              onClick={() => props.onPage(props.subjectPage + 1)}
            >
              {t('Next')}
            </Button>
          </div>
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>
            {editing
              ? t('Edit monitored subscription')
              : t('Add monitored subscription')}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <SubjectForm
            key={editing ? `${editing.id}/${editing.version}` : 'new'}
            subject={editing}
            channels={channels.data.items}
            onSaved={() => setEditing(null)}
          />
          <div className='mt-4 flex gap-2'>
            <Button
              variant='outline'
              disabled={channelPage === 1}
              onClick={() => setChannelPage(channelPage - 1)}
            >
              {t('Previous channels')}
            </Button>
            <Button
              variant='outline'
              disabled={channelPage * 100 >= channels.data.total}
              onClick={() => setChannelPage(channelPage + 1)}
            >
              {t('Next channels')}
            </Button>
            {editing && (
              <Button variant='ghost' onClick={() => setEditing(null)}>
                {t('Cancel')}
              </Button>
            )}
          </div>
        </CardContent>
      </Card>
    </div>
  )
}

function GlobalSettings(props: { settings: Settings }) {
  const { t } = useTranslation()
  const client = useQueryClient()
  const form = useForm<z.infer<typeof settingsSchema>>({
    resolver: zodResolver(settingsSchema),
    defaultValues: props.settings,
  })
  const mutation = useMutation({
    mutationFn: saveSettings,
    onSuccess: async () => {
      toast.success(t('Saved successfully'))
      await client.invalidateQueries({ queryKey: quotaKey })
    },
    onError: (error) => handleServerError(error),
  })
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Collection settings')}</CardTitle>
      </CardHeader>
      <CardContent>
        <Form {...form}>
          <form
            className='space-y-4'
            onSubmit={form.handleSubmit((input) =>
              mutation.mutate({ ...input, version: props.settings.version })
            )}
          >
            <FormField
              control={form.control}
              name='enabled'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Enable quota collection')}</FormLabel>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='interval_minutes'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Polling interval (minutes)')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={2}
                      max={1440}
                      {...field}
                      onChange={(event) =>
                        field.onChange(Number(event.target.value))
                      }
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <p className='text-muted-foreground text-sm'>
              {t(
                'Collection is off by default. Saving takes effect without restart; enabling can refresh OAuth credentials.'
              )}
            </p>
            <Button type='submit' disabled={mutation.isPending}>
              {t('Save collection settings')}
            </Button>
            {mutation.isError && (
              <p role='alert'>
                {t('Save failed. Reload current settings before retrying.')}
              </p>
            )}
          </form>
        </Form>
      </CardContent>
    </Card>
  )
}

function SubjectForm(props: {
  subject: Subject | null
  channels: { id: number; name: string; bound: boolean }[]
  onSaved: () => void
}) {
  const { t } = useTranslation()
  const client = useQueryClient()
  const form = useForm<z.infer<typeof subjectSchema>>({
    resolver: zodResolver(subjectSchema),
    defaultValues: {
      name: props.subject?.name ?? '',
      source_channel_id: props.subject?.source_channel_id ?? 0,
      enabled: props.subject?.enabled ?? false,
    },
  })
  const mutation = useMutation({
    mutationFn: saveSubject,
    onSuccess: async () => {
      toast.success(t('Saved successfully'))
      form.reset()
      props.onSaved()
      await client.invalidateQueries({ queryKey: quotaKey })
    },
    onError: (error) => handleServerError(error),
  })
  const options = props.channels.map((channel) => ({
    value: String(channel.id),
    label: `${channel.name}${channel.bound ? ` · ${t('Bound')}` : ''}`,
  }))
  if (
    props.subject &&
    !props.channels.some(
      (channel) => channel.id === props.subject?.source_channel_id
    )
  ) {
    options.unshift({
      value: String(props.subject.source_channel_id),
      label: `${t('Current source')} #${props.subject.source_channel_id}`,
    })
  }
  return (
    <Form {...form}>
      <form
        className='space-y-4'
        onSubmit={form.handleSubmit((input) =>
          mutation.mutate({
            ...input,
            id: props.subject?.id,
            version: props.subject?.version,
          })
        )}
      >
        <FormField
          control={form.control}
          name='name'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Subscription name')}</FormLabel>
              <FormControl>
                <Input maxLength={128} {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name='source_channel_id'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Source channel')}</FormLabel>
              <QuotaSelect
                label={t('Source channel')}
                value={String(field.value)}
                onChange={(value) => field.onChange(Number(value))}
                options={options}
              />
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name='enabled'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Monitor this subscription')}</FormLabel>
              <FormControl>
                <Switch
                  checked={field.value}
                  onCheckedChange={field.onChange}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <p className='text-muted-foreground text-sm'>
          {t(
            'Only enabled single-key Codex channels are supported. Switching source must keep the same account identity; duplicates never overwrite an existing ledger.'
          )}
        </p>
        <Button type='submit' disabled={mutation.isPending}>
          {t('Save monitored subscription')}
        </Button>
        {mutation.isError && (
          <p role='alert'>
            {t('Save failed. Reload current settings before retrying.')}
          </p>
        )}
      </form>
    </Form>
  )
}
