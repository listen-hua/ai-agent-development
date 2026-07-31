import axios, { AxiosError, type AxiosRequestConfig } from 'axios'

interface ErrorPayload {
  code?: number
  error?: string
  message?: string
  detail?: string
}

export class ApiError extends Error {
  constructor(
    message: string,
    public status: number,
    public detail?: string,
    public code?: number,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

let runtimeBaseURL = ''

const client = axios.create({
  withCredentials: true,
  timeout: 180_000,
  headers: { Accept: 'application/json' },
})

export function setApiBaseURL(value?: string): void {
  runtimeBaseURL = String(value || '').trim().replace(/\/+$/, '')
  client.defaults.baseURL = runtimeBaseURL || undefined
}

export function resolveApiURL(path: string): string {
  if (/^https?:\/\//i.test(path) || !runtimeBaseURL) return path
  if (runtimeBaseURL.endsWith('/api') && path.startsWith('/api/')) {
    return runtimeBaseURL + path.slice(4)
  }
  return runtimeBaseURL + (path.startsWith('/') ? path : `/${path}`)
}

async function request<T>(path: string, config: AxiosRequestConfig = {}): Promise<T> {
  try {
    const response = await client.request<T>({
      ...config,
      url: resolveApiURL(path),
    })
    if (response.status === 204) return undefined as T
    const payload = response.data as T & ErrorPayload
    if (payload && typeof payload === 'object' && typeof payload.code === 'number' && payload.code !== 0) {
      throw new ApiError(payload.message || payload.error || '请求失败', response.status, payload.detail, payload.code)
    }
    return response.data
  } catch (error) {
    if (error instanceof ApiError) throw error
    if (error instanceof AxiosError) {
      const payload = error.response?.data as ErrorPayload | undefined
      throw new ApiError(
        payload?.message || payload?.error || error.message || '请求失败',
        error.response?.status || 0,
        payload?.detail,
        payload?.code,
      )
    }
    throw error
  }
}

export const api = {
  get: <T>(path: string) => request<T>(path, { method: 'GET' }),
  post: <T>(path: string, body?: unknown) => request<T>(path, { method: 'POST', data: body }),
  put: <T>(path: string, body: unknown) => request<T>(path, { method: 'PUT', data: body }),
  patch: <T>(path: string, body: unknown) => request<T>(path, { method: 'PATCH', data: body }),
  upload: <T>(path: string, body: FormData, method: 'POST' | 'PUT' = 'POST') => request<T>(path, { method, data: body }),
  delete: <T = void>(path: string) => request<T>(path, { method: 'DELETE' }),
}
