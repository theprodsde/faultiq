import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  /* Enable SWR for Keycloak session reuse */
  reactStrictMode: true,
  
  /* Enable client-side caching */
  onDemandEntries: {
    maxInactiveAge: 60 * 1000,
    pagesBufferLength: 5,
  },

  /* Headers for security and API */
  async headers() {
    return [
      {
        source: '/:path*',
        headers: [
          {
            key: 'X-DNS-Prefetch-Control',
            value: 'on',
          },
          {
            key: 'X-Frame-Options',
            value: 'SAMEORIGIN',
          },
          {
            key: 'X-Content-Type-Options',
            value: 'nosniff',
          },
          {
            key: 'X-XSS-Protection',
            value: '1; mode=block',
          },
          {
            key: 'Referrer-Policy',
            value: 'strict-origin-when-cross-origin',
          },
        ],
      },
    ];
  },

  /* Redirect auth-related routes */
  async redirects() {
    return [
      {
        source: '/auth',
        destination: '/login',
        permanent: false,
      },
      {
        source: '/signin',
        destination: '/login',
        permanent: false,
      },
    ];
  },

  /* Environment variables */
  env: {
    NEXT_PUBLIC_API_GATEWAY_URL: process.env.NEXT_PUBLIC_API_GATEWAY_URL || 'http://localhost:8080/api/v1',
    NEXT_PUBLIC_WEBSOCKET_URL: process.env.NEXT_PUBLIC_WEBSOCKET_URL || 'ws://localhost:8085',
    NEXT_PUBLIC_AUTH_URL: process.env.NEXT_PUBLIC_AUTH_URL || 'http://localhost:8081',
    NEXT_PUBLIC_KEYCLOAK_REALM: process.env.NEXT_PUBLIC_KEYCLOAK_REALM || 'faultiq',
    NEXT_PUBLIC_KEYCLOAK_CLIENT_ID: process.env.NEXT_PUBLIC_KEYCLOAK_CLIENT_ID || 'faultiq-ui',
    NEXT_PUBLIC_KEYCLOAK_REDIRECT_URI: process.env.NEXT_PUBLIC_KEYCLOAK_REDIRECT_URI || 'http://localhost:3001',
    NEXT_PUBLIC_HOST_URL: process.env.NEXT_PUBLIC_HOST_URL || 'http://localhost:3001',
    NEXT_PUBLIC_APP_ENV: process.env.NEXT_PUBLIC_APP_ENV || 'production',
    NEXT_PUBLIC_APP_NAME: process.env.NEXT_PUBLIC_APP_NAME || 'FaultIQ',
    NEXT_PUBLIC_ENABLE_ANALYTICS: process.env.NEXT_PUBLIC_ENABLE_ANALYTICS || 'false',
    NEXT_PUBLIC_ENABLE_DARK_MODE: process.env.NEXT_PUBLIC_ENABLE_DARK_MODE || 'true',
    NEXT_PUBLIC_ENABLE_API_EXPLORER: process.env.NEXT_PUBLIC_ENABLE_API_EXPLORER || 'true',
    NEXT_PUBLIC_ENABLE_DOCUMENTATION: process.env.NEXT_PUBLIC_ENABLE_DOCUMENTATION || 'true',
    NEXT_PUBLIC_ENABLE_MONITORING: process.env.NEXT_PUBLIC_ENABLE_MONITORING || 'true',
  },
};

export default nextConfig;
