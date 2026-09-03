import axios, { AxiosInstance, AxiosRequestConfig, AxiosError } from 'axios'
import { config } from '@/config'

const serverBase = (typeof window === 'undefined' && process.env.SERVER_API_GATEWAY_URL) ? process.env.SERVER_API_GATEWAY_URL : config.apiUrl

const api: AxiosInstance = axios.create({
  baseURL: serverBase,
  timeout: 15000,
})

let authToken: string | null = null
let unauthorizedHandler: (() => void) | null = null

export function setAuthToken(token: string | null) {
  authToken = token
  if (token) {
    api.defaults.headers.common['Authorization'] = `Bearer ${token}`
  } else {
    delete api.defaults.headers.common['Authorization']
  }
}

export function onUnauthorized(cb: () => void) {
  unauthorizedHandler = cb
}

api.interceptors.request.use((cfg: any) => {
  if (authToken && cfg && cfg.headers && !cfg.headers['Authorization']) {
    cfg.headers['Authorization'] = `Bearer ${authToken}`
  }
  return cfg
})

api.interceptors.response.use(
  (res) => res,
  async (err: AxiosError) => {
    const status = err.response?.status
    if (status === 401) {
      if (unauthorizedHandler) unauthorizedHandler()
    }
    return Promise.reject(err)
  }
)

export default api

