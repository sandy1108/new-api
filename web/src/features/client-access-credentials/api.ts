import { api } from '@/lib/api'
import { requireServerSuccess } from '@/lib/server-error-message'

export type Scope = { key: string; display_name: string; description: string; resource_type: string; max_resources: number }
export type Resource = { id: number; name: string; type: number }
export type Grant = { scope: string; resource_id: string }
export type Credential = { id: number; name: string; token_prefix: string; status: number; version: number; expired_time: number; created_time: number; last_used_time: number }
export type Detail = { credential: Credential; grants: Grant[] }
export type Issued = { credential: Credential; token: string }

const base = '/api/client-access'

export async function listScopes(): Promise<Scope[]> {
  return requireServerSuccess((await api.get(`${base}/scopes`)).data).data
}
export async function listResources(scope: string): Promise<Resource[]> {
  return requireServerSuccess((await api.get(`${base}/scopes/${encodeURIComponent(scope)}/resources`)).data).data
}
export async function listCredentials(): Promise<{ items: Credential[]; total: number }> {
  return requireServerSuccess((await api.get(`${base}/credentials`)).data).data
}
export async function getCredential(id: number): Promise<Detail> {
  return requireServerSuccess((await api.get(`${base}/credentials/${id}`)).data).data
}
export async function createCredential(input: { name: string; expired_time: number; grants: Grant[] }): Promise<Issued> {
  return requireServerSuccess((await api.post(`${base}/credentials`, input)).data).data
}
export async function updateGrants(id: number, version: number, grants: Grant[]): Promise<Credential> {
  return requireServerSuccess((await api.put(`${base}/credentials/${id}/grants`, { version, grants })).data).data
}
export async function credentialAction(id: number, version: number, action: 'rotate' | 'disable' | 'enable' | 'revoke'): Promise<Credential | Issued> {
  return requireServerSuccess((await api.post(`${base}/credentials/${id}/${action}`, { version })).data).data
}
