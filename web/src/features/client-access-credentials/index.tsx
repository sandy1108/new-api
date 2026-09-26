import { useEffect, useState } from 'react'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout/components/section-page-layout'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { handleServerError } from '@/lib/handle-server-error'

import {
  createCredential, credentialAction, getCredential, listCredentials,
  listResources, listScopes, updateGrants,
  type Credential, type Detail, type Grant, type Resource, type Scope,
} from './api'

const statusLabel: Record<number, string> = { 1: '启用', 2: '禁用', 3: '已撤销' }

export function ClientAccessCredentials() {
  const [items, setItems] = useState<Credential[]>([])
  const [scopes, setScopes] = useState<Scope[]>([])
  const [resources, setResources] = useState<Record<string, Resource[]>>({})
  const [name, setName] = useState('')
  const [expiry, setExpiry] = useState('')
  const [createSelected, setCreateSelected] = useState<Record<string, string>>({})
  const [editSelected, setEditSelected] = useState<Record<string, string>>({})
  const [detail, setDetail] = useState<Detail | null>(null)
  const [secret, setSecret] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  async function reload() {
    const [credentials, catalog] = await Promise.all([listCredentials(), listScopes()])
    setItems(credentials.items ?? [])
    setScopes(catalog)
    const options = await Promise.all(catalog.map((scope) => listResources(scope.key)))
    setResources(Object.fromEntries(catalog.map((scope, index) => [scope.key, options[index]])))
  }

  useEffect(() => { void reload().catch(handleServerError) }, [])

  function selectedGrants(selected: Record<string, string>): Grant[] {
    return Object.entries(selected).filter(([, id]) => id).map(([scope, resource_id]) => ({ scope, resource_id }))
  }

  async function perform(action: () => Promise<void>) {
    setBusy(true)
    try { await action(); await reload() }
    catch (error) {
      if (typeof error === 'object' && error !== null && 'response' in error &&
          (error.response as { status?: number } | undefined)?.status === 409) {
        setDetail(null)
        toast.error('凭据已被其他管理员修改，请重新打开后重试')
        await reload().catch(handleServerError)
      } else {
        handleServerError(error)
      }
    }
    finally { setBusy(false) }
  }

  async function openDetail(id: number) {
    try {
      const current = await getCredential(id)
      setDetail(current)
      setEditSelected(Object.fromEntries(current.grants.map((grant) => [grant.scope, grant.resource_id])))
    } catch (error) { handleServerError(error) }
  }

  function issueExpiry(): number {
    if (!expiry) return -1
    const parsed = Date.parse(expiry)
    if (!Number.isFinite(parsed) || parsed <= Date.now()) throw new Error('过期时间必须晚于当前时间')
    return Math.floor(parsed / 1000)
  }

  return <SectionPageLayout>
    <SectionPageLayout.Title>客户端访问凭据</SectionPageLayout.Title>
    <SectionPageLayout.Content>
      <div className='space-y-5 pb-8'>
        <p className='text-muted-foreground text-sm'>仅用于受限客户端 API；它不是后台管理令牌，也不能调用模型接口。首期仅支持读取绑定的 Codex 渠道额度。</p>
        {secret && <Card className='border-amber-500/50'>
          <CardHeader><CardTitle>请立即保存新凭据</CardTitle><CardDescription>完整凭据只在本次显示。关闭后无法再次查看；请勿截图、记录到日志或放入 URL。</CardDescription></CardHeader>
          <CardContent className='space-y-3'><Input aria-label='新客户端凭据' value={secret} readOnly onFocus={(event) => event.currentTarget.select()} />
            <Button variant='outline' onClick={() => setSecret(null)}>我已保存，关闭</Button></CardContent>
        </Card>}
        <Card><CardHeader><CardTitle>创建凭据</CardTitle><CardDescription>选择 Scope 后，只能绑定此 Scope 允许的渠道。最终授权由服务端再次校验。</CardDescription></CardHeader>
          <CardContent className='space-y-4'>
            <div className='grid gap-3 sm:grid-cols-2'><div className='space-y-2'><Label htmlFor='client-access-name'>名称</Label><Input id='client-access-name' value={name} maxLength={128} onChange={(event) => setName(event.target.value)} /></div>
              <div className='space-y-2'><Label htmlFor='client-access-expiry'>过期时间（留空为长期有效）</Label><Input id='client-access-expiry' type='datetime-local' value={expiry} onChange={(event) => setExpiry(event.target.value)} /></div></div>
            <ScopePicker scopes={scopes} resources={resources} selected={createSelected} onChange={setCreateSelected} />
            <Button disabled={busy || !name.trim() || selectedGrants(createSelected).length === 0} onClick={() => void perform(async () => {
              const issued = await createCredential({ name: name.trim(), expired_time: issueExpiry(), grants: selectedGrants(createSelected) })
              setSecret(issued.token); setName(''); setExpiry(''); setCreateSelected({})
              toast.success('凭据已创建，请立即保存')
            })}>创建凭据</Button>
          </CardContent></Card>
        <Card><CardHeader><CardTitle>已有凭据</CardTitle><CardDescription>列表仅显示前缀；轮换会让旧凭据立即失效。撤销不可恢复。</CardDescription></CardHeader>
          <CardContent className='space-y-3'>{items.length === 0 ? <p className='text-muted-foreground text-sm'>暂无凭据</p> : items.map((item) => <div key={item.id} className='flex flex-wrap items-center justify-between gap-3 rounded-lg border p-3'>
            <div><div className='font-medium'>{item.name}</div><div className='text-muted-foreground text-xs'>{item.token_prefix}… · {statusLabel[item.status] ?? '未知'} · 最后使用：{item.last_used_time ? new Date(item.last_used_time * 1000).toLocaleString() : '从未'}</div></div>
            <div className='flex flex-wrap gap-2'><Button variant='outline' size='sm' onClick={() => void openDetail(item.id)}>授权</Button>
              {item.status !== 3 && <Button variant='outline' size='sm' disabled={busy} onClick={() => void perform(async () => { if (!window.confirm('轮换后旧凭据立即失效，确认继续？')) return; const issued = await credentialAction(item.id, item.version, 'rotate'); if ('token' in issued) setSecret(issued.token) })}>轮换</Button>}
              {item.status === 1 && <Button variant='outline' size='sm' disabled={busy} onClick={() => void perform(async () => { await credentialAction(item.id, item.version, 'disable') })}>禁用</Button>}
              {item.status === 2 && <Button variant='outline' size='sm' disabled={busy} onClick={() => void perform(async () => { await credentialAction(item.id, item.version, 'enable') })}>启用</Button>}
              {item.status !== 3 && <Button variant='destructive' size='sm' disabled={busy} onClick={() => void perform(async () => { if (!window.confirm('撤销不可恢复，确认撤销此凭据？')) return; await credentialAction(item.id, item.version, 'revoke') })}>撤销</Button>}</div>
          </div>)}</CardContent></Card>
        {detail && <Card><CardHeader><CardTitle>编辑授权：{detail.credential.name}</CardTitle><CardDescription>保存时检查版本；若其他管理员已修改，请重新打开。</CardDescription></CardHeader>
          <CardContent className='space-y-4'><ScopePicker scopes={scopes} resources={resources} selected={editSelected} onChange={setEditSelected} />
            <div className='flex gap-2'><Button disabled={busy || detail.credential.status === 3} onClick={() => void perform(async () => {
              await updateGrants(detail.credential.id, detail.credential.version, selectedGrants(editSelected))
              setDetail(null); setEditSelected({}); toast.success('授权已更新')
            })}>保存授权</Button><Button variant='outline' onClick={() => { setDetail(null); setEditSelected({}) }}>取消</Button></div></CardContent></Card>}
      </div>
    </SectionPageLayout.Content>
  </SectionPageLayout>
}

function ScopePicker({ scopes, resources, selected, onChange }: { scopes: Scope[]; resources: Record<string, Resource[]>; selected: Record<string, string>; onChange: (value: Record<string, string>) => void }) {
  return <div className='space-y-3'>{scopes.map((scope) => <div key={scope.key} className='space-y-2 rounded-lg border p-3'>
    <label className='flex items-center gap-2'><Checkbox checked={scope.key in selected} onCheckedChange={(checked) => {
      const next = { ...selected }; if (checked) next[scope.key] = ''; else delete next[scope.key]; onChange(next)
    }} /><span className='font-medium'>{scope.display_name}</span><code className='text-muted-foreground text-xs'>{scope.key}</code></label>
    <p className='text-muted-foreground text-xs'>{scope.description}</p>
    {scope.key in selected && <div className='space-y-1'><Label htmlFor={`resource-${scope.key}`}>授权渠道</Label><select id={`resource-${scope.key}`} className='border-input bg-background h-8 w-full rounded-lg border px-2 text-sm' value={selected[scope.key]} onChange={(event) => onChange({ ...selected, [scope.key]: event.target.value })}>
      <option value=''>请选择渠道</option>{(resources[scope.key] ?? []).map((resource) => <option key={resource.id} value={String(resource.id)}>{resource.name} (#{resource.id})</option>)}</select>
      {(resources[scope.key] ?? []).length === 0 && <p className='text-destructive text-xs'>没有可用的符合条件的渠道</p>}</div>}
  </div>)}</div>
}
