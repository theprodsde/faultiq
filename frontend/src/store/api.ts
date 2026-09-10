/**
 * RTK Query base API — all service endpoints are injected from feature slices.
 * Auth token is read from the global `authToken` set by auth-provider.
 */
import { createApi, fetchBaseQuery } from "@reduxjs/toolkit/query/react"

export const baseApi = createApi({
  reducerPath: "api",
  baseQuery: fetchBaseQuery({
    baseUrl: process.env.NEXT_PUBLIC_API_GATEWAY_URL ?? "http://localhost:8080/api/v1",
    prepareHeaders: (headers) => {
      // Primary: window accessor set by auth-provider
      let token: string | undefined
      if (typeof window !== "undefined") {
        token = window.__faultiqToken
        // Fallback: check localStorage
        if (!token) {
          try {
            token = localStorage.getItem('faultiq.token') || undefined
          } catch {}
        }
      }
      if (token) headers.set("Authorization", `Bearer ${token}`)
      return headers
    },
  }),
  tagTypes: ["Tenant", "Project", "Incident", "Graph", "Signal", "Recommendation"],
  endpoints: () => ({}),
})
