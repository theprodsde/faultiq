import { readFileSync } from 'fs'
import { resolve } from 'path'

interface Config {
  apiUrl: string
  websocketUrl: string
  authUrl: string
  keycloakRealm: string
  keycloakClientId: string
  redirectUri: string
  appEnv: 'development' | 'staging' | 'production'
  appName: string
  enableAnalytics: boolean
  enableDarkMode: boolean
  enableApiExplorer: boolean
  enableDocumentation: boolean
  enableMonitoring: boolean
  statusApiUrl: string
  detectionEngineUrl: string
  graphManagerUrl: string
  neo4jUrl: string
  postgresUrl: string
  redisUrl: string
}

const getConfig = (): Config => {
  return {
    apiUrl: process.env.NEXT_PUBLIC_API_GATEWAY_URL || 'http://localhost:8080/api/v1',
    websocketUrl: process.env.NEXT_PUBLIC_WEBSOCKET_URL || 'ws://localhost:8085',
    authUrl: process.env.NEXT_PUBLIC_AUTH_URL || 'http://localhost:8081',
    keycloakRealm: process.env.NEXT_PUBLIC_KEYCLOAK_REALM || 'faultiq',
    keycloakClientId: process.env.NEXT_PUBLIC_KEYCLOAK_CLIENT_ID || 'faultiq-ui',
    redirectUri: process.env.NEXT_PUBLIC_KEYCLOAK_REDIRECT_URI || 'http://localhost:3001',
    appEnv: (process.env.NEXT_PUBLIC_APP_ENV as any) || 'development',
    appName: process.env.NEXT_PUBLIC_APP_NAME || 'FaultIQ',
    enableAnalytics: process.env.NEXT_PUBLIC_ENABLE_ANALYTICS === 'true',
    enableDarkMode: process.env.NEXT_PUBLIC_ENABLE_DARK_MODE === 'true',
    enableApiExplorer: process.env.NEXT_PUBLIC_ENABLE_API_EXPLORER === 'true',
    enableDocumentation: process.env.NEXT_PUBLIC_ENABLE_DOCUMENTATION === 'true',
    enableMonitoring: process.env.NEXT_PUBLIC_ENABLE_MONITORING === 'true',
    // Default status API should point at the API Gateway host in local dev
    statusApiUrl: process.env.NEXT_PUBLIC_STATUS_API_URL || 'http://localhost:8080',
    detectionEngineUrl: process.env.NEXT_PUBLIC_DETECTION_ENGINE_URL || 'http://localhost:8082',
    graphManagerUrl: process.env.NEXT_PUBLIC_GRAPH_MANAGER_URL || 'http://localhost:8086',
    neo4jUrl: process.env.NEXT_PUBLIC_NEO4J_URL || 'http://localhost:7474',
    postgresUrl: process.env.NEXT_PUBLIC_POSTGRES_URL || 'http://localhost:5434',
    redisUrl: process.env.NEXT_PUBLIC_REDIS_URL || 'http://localhost:6379',
  }
}

export const config = getConfig()

export default config
