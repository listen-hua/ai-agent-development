import { api } from './api'
import type { User } from '@/types/domain'

export interface FeishuAuthConfig {
  app_id: string
  enabled: boolean
}

export interface FeishuLoginCode {
  code: string
  auth_method: 'request_access' | 'request_auth_code'
}

export class FeishuClientUnavailableError extends Error {
  constructor() {
    super('当前不在飞书客户端内')
    this.name = 'FeishuClientUnavailableError'
  }
}

interface FeishuSDKError {
  errno?: number | string
  errNo?: number | string
  errCode?: number | string
  code?: number | string
  errMsg?: string
  errString?: string
  message?: string
}

function sdkErrorMessage(error: unknown): string {
  if (error instanceof Error) return error.message
  const value = error as FeishuSDKError | null
  return value?.errMsg || value?.errString || value?.message || '飞书身份授权失败'
}

function sdkErrorCode(error: unknown): string {
  const value = error as FeishuSDKError | null
  const code = value?.errno ?? value?.errNo ?? value?.errCode ?? value?.code
  return code === undefined ? '' : String(code)
}

function sdkError(error: unknown): Error {
  if (error instanceof Error) return error
  const code = sdkErrorCode(error)
  const suffix = code ? `（错误码 ${code}）` : ''
  return new Error(`${sdkErrorMessage(error)}${suffix}`)
}

function createState(): string {
  if (typeof crypto.randomUUID === 'function') return crypto.randomUUID()
  const bytes = crypto.getRandomValues(new Uint8Array(16))
  return Array.from(bytes, (value) => value.toString(16).padStart(2, '0')).join('')
}

function reportClientFailure(stage: string, error: unknown): void {
  void api.post<void>('/api/v1/auth/feishu/client-diagnostics', {
    stage,
    errno: sdkErrorCode(error),
    message: sdkErrorMessage(error),
    h5sdk: Boolean(window.h5sdk),
    request_access: Boolean(window.tt?.requestAccess),
    request_auth_code: Boolean(window.tt?.requestAuthCode),
  }).catch(() => undefined)
}

export const authService = {
  me: () => api.get<User>('/api/v1/me'),
  feishuConfig: () => api.get<FeishuAuthConfig>('/api/v1/auth/feishu/config'),
  exchange: (loginCode: FeishuLoginCode | string) => api.post<User>(
    '/api/v1/auth/feishu/exchange',
    typeof loginCode === 'string' ? { code: loginCode } : loginCode,
  ),
  logout: () => api.post<void>('/api/v1/auth/logout'),
  requestFeishuCode(appID: string): Promise<FeishuLoginCode> {
    return new Promise((resolve, reject) => {
      let started = false
      let legacyStarted = false

      const requestLegacyCode = () => {
        if (legacyStarted) return
        legacyStarted = true
        if (!window.tt?.requestAuthCode) {
          reportClientFailure('bridge_unavailable', new Error('requestAccess 和 requestAuthCode 均不可用'))
          reject(new FeishuClientUnavailableError())
          return
        }
        try {
          window.tt.requestAuthCode({
            appId: appID,
            success: ({ code }) => {
              if (!code) {
                const error = new Error('飞书返回了空授权码')
                reportClientFailure('request_auth_code_empty', error)
                reject(error)
                return
              }
              resolve({ code, auth_method: 'request_auth_code' })
            },
            fail: (error) => {
              reportClientFailure('request_auth_code_fail', error)
              reject(sdkError(error))
            },
          })
        } catch (error) {
          reportClientFailure('request_auth_code_throw', error)
          reject(sdkError(error))
        }
      }

      const execute = () => {
        if (started) return
        started = true
        if (window.tt?.requestAuthCode) {
          requestLegacyCode()
          return
        }

        if (!window.tt?.requestAccess) {
          reportClientFailure('bridge_unavailable', new Error('requestAccess 和 requestAuthCode 均不可用'))
          reject(new FeishuClientUnavailableError())
          return
        }

        try {
          const state = createState()
          window.tt.requestAccess({
            appID,
            scopeList: [],
            state,
            success: ({ code, state: returnedState }) => {
              if (returnedState && returnedState !== state) {
                const error = new Error('飞书授权状态校验失败')
                reportClientFailure('request_access_state', error)
                reject(error)
                return
              }
              if (!code) {
                const error = new Error('飞书返回了空授权码')
                reportClientFailure('request_access_empty', error)
                reject(error)
                return
              }
              resolve({ code, auth_method: 'request_access' })
            },
            fail: (error) => {
              reportClientFailure('request_access_fail', error)
              const errno = sdkErrorCode(error)
              if (errno !== '2700002' && window.tt?.requestAuthCode) {
                requestLegacyCode()
                return
              }
              reject(sdkError(error))
            },
          })
        } catch (error) {
          reportClientFailure('request_access_throw', error)
          if (window.tt?.requestAuthCode) {
            requestLegacyCode()
          } else {
            reject(sdkError(error))
          }
        }
      }
      if (window.h5sdk?.ready) window.h5sdk.ready(execute)
      else execute()
    })
  },
}
