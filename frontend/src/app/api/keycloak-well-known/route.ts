import type { NextRequest } from 'next/server'

export async function GET(req: NextRequest) {
  try {
    // Detect environment - use localhost when running locally, docker network when in container
    let serverAuth = process.env.SERVER_AUTH_URL
    
    if (!serverAuth) {
      // Auto-detect: if NODE_ENV indicates local dev, use localhost, otherwise use docker network
      serverAuth = process.env.NODE_ENV === 'development' 
        ? 'http://localhost:8081'
        : 'http://keycloak:8080'
    }
    
    const kcUrl = `${serverAuth.replace(/\/$/, '')}/realms/faultiq/.well-known/openid-configuration`
    console.log('[Keycloak Proxy] Fetching from:', kcUrl)
    
    const res = await fetch(kcUrl, { method: 'GET' })
    let body = await res.text()

    // Rewrite internal Keycloak URLs to the public-facing auth URL
    try {
      const publicAuthUrl = (process.env.NEXT_PUBLIC_AUTH_URL || 'http://localhost:8081').replace(/\/$/, '')
      const internalBase = 'http://keycloak:8080'
      if (body && body.indexOf(internalBase) !== -1) {
        body = body.replace(new RegExp(internalBase, 'g'), publicAuthUrl)
      }
    } catch (e) {
      // If rewriting fails, fall back to returning original body
    }
    const headers = new Headers()
    headers.set('Content-Type', 'application/json')
    // Allow browser to fetch this proxied endpoint from the frontend origin
    headers.set('Access-Control-Allow-Origin', '*')
    return new Response(body, { status: res.status, headers })
  } catch (e) {
    console.error('[Keycloak Proxy] Error:', e)
    return new Response(JSON.stringify({ error: 'proxy error' }), { status: 502, headers: { 'Content-Type': 'application/json' } })
  }
}
