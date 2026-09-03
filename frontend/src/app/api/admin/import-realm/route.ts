import { NextResponse } from 'next/server'
import { readFile } from 'fs/promises'
import { resolve } from 'path'
import { config } from '@/config'

async function getAdminToken() {
  const adminUser = process.env.KEYCLOAK_ADMIN_USERNAME
  const adminPass = process.env.KEYCLOAK_ADMIN_PASSWORD
  if (!adminUser || !adminPass) throw new Error('Missing KEYCLOAK_ADMIN_USERNAME / KEYCLOAK_ADMIN_PASSWORD env vars')

  const tokenUrl = `${config.authUrl.replace(/\/$/, '')}/realms/master/protocol/openid-connect/token`
  const body = new URLSearchParams({
    grant_type: 'password',
    client_id: 'admin-cli',
    username: adminUser,
    password: adminPass,
  })
  const res = await fetch(tokenUrl, { method: 'POST', body })
  if (!res.ok) throw new Error('Failed to obtain admin token')
  const json = await res.json()
  return json.access_token as string
}

export async function POST() {
  try {
    const realmPath = resolve(process.cwd(), 'scripts/keycloak/realm.json')
    const raw = await readFile(realmPath, 'utf-8')
    const parsed = JSON.parse(raw)
    const users = parsed.users || []
    if (!users.length) return NextResponse.json({ ok: true, imported: 0 })
    const adminToken = await getAdminToken()
    const realm = config.keycloakRealm
    const createUrl = `${config.authUrl.replace(/\/$/, '')}/admin/realms/${realm}/users`
    let imported = 0
    for (const u of users) {
      const userRep: any = {
        username: u.username,
        enabled: u.enabled !== false,
        firstName: u.firstName || undefined,
        lastName: u.lastName || undefined,
        email: u.email || undefined,
        attributes: u.attributes || undefined,
      }
      const res = await fetch(createUrl, {
        method: 'POST',
        headers: { Authorization: `Bearer ${adminToken}`, 'Content-Type': 'application/json' },
        body: JSON.stringify(userRep),
      })
      if (res.status === 201) {
        // find created
        const searchRes = await fetch(`${createUrl}?username=${encodeURIComponent(u.username)}`, {
          headers: { Authorization: `Bearer ${adminToken}` },
        })
        const arr = await searchRes.json()
        const created = arr[0]
        if (created && created.id) {
          const credUrl = `${createUrl}/${created.id}/reset-password`
          const password = (u.credentials && u.credentials[0] && u.credentials[0].value) || ''
          if (password) {
            await fetch(credUrl, {
              method: 'PUT',
              headers: { Authorization: `Bearer ${adminToken}`, 'Content-Type': 'application/json' },
              body: JSON.stringify({ type: 'password', value: password, temporary: false }),
            })
          }
          imported++
        }
      }
    }
    return NextResponse.json({ ok: true, imported })
  } catch (e: any) {
    return NextResponse.json({ error: e.message || String(e) }, { status: 500 })
  }
}
