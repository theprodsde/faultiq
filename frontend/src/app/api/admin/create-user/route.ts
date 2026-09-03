import { NextResponse } from 'next/server'
import { config } from '@/config'

interface CreateUserBody {
  username: string
  password: string
  firstName?: string
  lastName?: string
  email?: string
  realmRoles?: string[]
  attributes?: Record<string,string>
}

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

export async function POST(req: Request) {
  try {
    const body: CreateUserBody = await req.json()
    const adminToken = await getAdminToken()
    const realm = config.keycloakRealm
    // create user
    const createUrl = `${config.authUrl.replace(/\/$/, '')}/admin/realms/${realm}/users`
    const userRep: any = {
      username: body.username,
      enabled: true,
      firstName: body.firstName || undefined,
      lastName: body.lastName || undefined,
      email: body.email || undefined,
      attributes: body.attributes || undefined,
    }
    const createRes = await fetch(createUrl, {
      method: 'POST',
      headers: { Authorization: `Bearer ${adminToken}`, 'Content-Type': 'application/json' },
      body: JSON.stringify(userRep),
    })
    if (createRes.status !== 201) {
      const txt = await createRes.text()
      return NextResponse.json({ error: 'create_failed', detail: txt }, { status: 500 })
    }
    // Retrieve created user id via search
    const searchRes = await fetch(`${createUrl}?username=${encodeURIComponent(body.username)}`, {
      headers: { Authorization: `Bearer ${adminToken}` },
    })
    const users = await searchRes.json()
    const created = users[0]
    if (!created || !created.id) {
      return NextResponse.json({ error: 'user_not_found_after_create' }, { status: 500 })
    }
    const userId = created.id
    // set password
    const credUrl = `${createUrl}/${userId}/reset-password`
    const credBody = { type: 'password', value: body.password, temporary: false }
    await fetch(credUrl, {
      method: 'PUT',
      headers: { Authorization: `Bearer ${adminToken}`, 'Content-Type': 'application/json' },
      body: JSON.stringify(credBody),
    })
    // assign realm roles if provided
    if (body.realmRoles && body.realmRoles.length > 0) {
      // fetch role representations
      const rolesRes = await fetch(`${config.authUrl.replace(/\/$/, '')}/admin/realms/${realm}/roles`, {
        headers: { Authorization: `Bearer ${adminToken}` },
      })
      const allRoles = await rolesRes.json()
      const toAssign = allRoles.filter((r: any) => body.realmRoles!.includes(r.name))
      if (toAssign.length > 0) {
        await fetch(`${createUrl}/${userId}/role-mappings/realm`, {
          method: 'POST',
          headers: { Authorization: `Bearer ${adminToken}`, 'Content-Type': 'application/json' },
          body: JSON.stringify(toAssign),
        })
      }
    }

    return NextResponse.json({ ok: true, id: userId })
  } catch (e: any) {
    return NextResponse.json({ error: e.message || String(e) }, { status: 500 })
  }
}
