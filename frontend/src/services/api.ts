export class ApiError extends Error { constructor(message: string, public status: number, public detail?: string) { super(message) } }

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  if (!(init.body instanceof FormData)) headers.set('Content-Type', 'application/json')
  const response = await fetch(path, { ...init, headers, credentials: 'include' })
  if (!response.ok) {
    const payload = await response.json().catch(() => ({ error: '请求失败' })) as { error?: string; detail?: string }
    throw new ApiError(payload.error || '请求失败', response.status, payload.detail)
  }
  if (response.status === 204) return undefined as T
  return response.json() as Promise<T>
}

export const api = {
  get: <T>(path: string) => request<T>(path),
  post: <T>(path: string, body?: unknown) => request<T>(path, { method: 'POST', body: body === undefined ? undefined : JSON.stringify(body) }),
  put: <T>(path: string, body: unknown) => request<T>(path, { method: 'PUT', body: JSON.stringify(body) }),
  patch: <T>(path: string, body: unknown) => request<T>(path, { method: 'PATCH', body: JSON.stringify(body) }),
  upload: <T>(path: string, body: FormData) => request<T>(path, { method: 'POST', body }),
  delete: (path: string) => request<void>(path, { method: 'DELETE' }),
}
