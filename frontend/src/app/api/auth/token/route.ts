import { NextResponse } from 'next/server'

export async function POST(req: Request) {
  try {
    const body = await req.json()
    const { username, password } = body || {}

    if (!username || !password) {
      return NextResponse.json({ error: 'username and password required' }, { status: 400 })
    }

    // Prefer server/runtime URL when executing on the server inside the container
    const authUrl = process.env.SERVER_AUTH_URL || process.env.NEXT_PUBLIC_AUTH_URL || process.env.NEXT_PUBLIC_KEYCLOAK_URL || 'http://localhost:8081'
    const realm = process.env.NEXT_PUBLIC_KEYCLOAK_REALM || 'faultiq'
    const clientId = process.env.NEXT_PUBLIC_KEYCLOAK_CLIENT_ID || 'faultiq-ui'
    const clientSecret = process.env.KEYCLOAK_CLIENT_SECRET || process.env.NEXT_PUBLIC_KEYCLOAK_CLIENT_SECRET

    const tokenEndpoint = `${authUrl.replace(/\/$/, '')}/realms/${realm}/protocol/openid-connect/token`

    const params = new URLSearchParams()
    params.append('grant_type', 'password')
    params.append('client_id', clientId)
    if (clientSecret) params.append('client_secret', clientSecret)
    params.append('username', username)
    params.append('password', password)

    const tokenResp = await fetch(tokenEndpoint, {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      body: params.toString(),
    })

    const text = await tokenResp.text()
    if (!tokenResp.ok) {
      // forward error
      let status = tokenResp.status || 500
      try {
        const json = JSON.parse(text)
        return NextResponse.json(json, { status })
      } catch (e) {
        return NextResponse.json({ error: text }, { status })
      }
    }

    const data = JSON.parse(text)
    return NextResponse.json(data)
  } catch (err: any) {
    return NextResponse.json({ error: err?.message || 'unexpected error' }, { status: 500 })
  }
}
