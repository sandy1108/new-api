import { useQueryClient } from '@tanstack/react-query'
import { Loader2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import {
  SecureVerificationDialog,
  useSecureVerification,
} from '@/features/auth/secure-verification'
import { SettingsCard } from '@/features/system-settings/components/settings-card'
import {
  SettingsSwitchContent,
  SettingsSwitchRow,
} from '@/features/system-settings/components/settings-form-layout'
import { handleServerError } from '@/lib/handle-server-error'

import { saveReportGrant, type ReportGrant } from '../api'
import { reportIdentity, useReportAccess } from '../hooks'

// 按账户隔离草稿、再次验证与保存；开关只编辑草稿，绝不直接提交权限。
export function GrantCard(props: { user: ReportGrant }) {
  const { t } = useTranslation()
  const access = useReportAccess()
  const client = useQueryClient()
  const verification = useSecureVerification()
  const [usage, setUsage] = useState(props.user.usage_read_all)
  const [ledger, setLedger] = useState(props.user.subscription_read_all)
  const [saving, setSaving] = useState(false)
  const dirty =
    usage !== props.user.usage_read_all ||
    ledger !== props.user.subscription_read_all
  const changed = dirty || props.user.invalidated
  const save = async () => {
    if (saving || !access.canManage || !changed) return
    setSaving(true)
    // 将最终勾选和版本绑定到同一个proof，验证期间禁用修改且不自动重试。
    const identity = reportIdentity()
    const input = {
      usage_read_all: usage,
      subscription_read_all: ledger,
      version: props.user.version,
    }
    try {
      const proof = await verification.requestVerification({
        scope: 'pong.report_grants.update',
        context: { user_id: props.user.id, ...input },
        title: t('Report access grants'),
      })
      if (!proof || reportIdentity() !== identity) return
      await saveReportGrant(props.user.id, input, proof.proof_token)
      toast.success(t('Saved successfully'))
      await client.invalidateQueries({ queryKey: ['pong-report-grants'] })
      await client.invalidateQueries({ queryKey: ['pong-report-access'] })
    } catch (error) {
      handleServerError(error)
      await client.invalidateQueries({ queryKey: ['pong-report-grants'] })
    } finally {
      setSaving(false)
    }
  }
  const name = props.user.display_name || props.user.username
  return (
    <section aria-label={name}>
      <SettingsCard
        title={name}
        description={`@${props.user.username} · ID ${props.user.id}`}
        className='min-w-0 [&_[data-slot=card-description]]:break-all [&_[data-slot=card-title]]:break-all'
      >
        <div className='mb-3 flex flex-wrap items-center gap-2'>
          <Badge variant='secondary'>{t('Common User')}</Badge>
          <Badge variant={changed ? 'warning' : 'outline'}>
            {changed ? t('Unsaved changes') : t('Saved')}
          </Badge>
        </div>
        {props.user.invalidated && (
          <p
            role='status'
            className='text-destructive mb-3 text-sm leading-relaxed'
          >
            {t('Account security changed. Save again to renew these grants.')}
          </p>
        )}
        <div className='divide-y rounded-xl border px-4'>
          <SettingsSwitchRow>
            <SettingsSwitchContent>
              <Label
                htmlFor={`usage-${props.user.id}`}
                className='leading-relaxed'
              >
                {t('All-account token usage')}
              </Label>
              <p className='text-muted-foreground text-xs leading-relaxed'>
                {t(
                  'View usage and costs for all accounts. No raw logs or API keys.'
                )}
              </p>
            </SettingsSwitchContent>
            <Switch
              id={`usage-${props.user.id}`}
              aria-label={t('All-account token usage')}
              checked={usage}
              disabled={saving || !access.canManage}
              onCheckedChange={setUsage}
            />
          </SettingsSwitchRow>
          <SettingsSwitchRow>
            <SettingsSwitchContent>
              <Label
                htmlFor={`ledger-${props.user.id}`}
                className='leading-relaxed'
              >
                {t('All-account subscription quota')}
              </Label>
              <p className='text-muted-foreground text-xs leading-relaxed'>
                {t(
                  'View monitored subscriptions and quota history. No collection or configuration changes.'
                )}
              </p>
            </SettingsSwitchContent>
            <Switch
              id={`ledger-${props.user.id}`}
              aria-label={t('All-account subscription quota')}
              checked={ledger}
              disabled={saving || !access.canManage}
              onCheckedChange={setLedger}
            />
          </SettingsSwitchRow>
        </div>
        <div className='mt-4 flex flex-col gap-3 border-t pt-4 sm:flex-row sm:items-center sm:justify-between'>
          <p className='text-muted-foreground text-xs leading-relaxed'>
            {t(
              'Changes take effect only after saving and identity verification.'
            )}
          </p>
          <div className='flex shrink-0 gap-2'>
            {dirty && (
              <Button
                variant='outline'
                disabled={saving}
                onClick={() => {
                  setUsage(props.user.usage_read_all)
                  setLedger(props.user.subscription_read_all)
                }}
              >
                {t('Cancel')}
              </Button>
            )}
            <Button
              disabled={saving || !access.canManage || !changed}
              aria-busy={saving}
              onClick={() => void save()}
            >
              {saving && (
                <Loader2 aria-hidden='true' className='size-4 animate-spin' />
              )}
              {t('Save')}
            </Button>
          </div>
        </div>
        <SecureVerificationDialog {...verification.dialogProps} />
      </SettingsCard>
    </section>
  )
}
