import { api } from './api'
import type { User } from '@/types/domain'
import { createSecureClientUUID } from '@/utils/clientId'

export interface FeishuAuthConfig {
  app_id: string
  enabled: boolean
  dev_auth_enabled: boolean
}

export interface IAMAuthConfig {
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
  if (code === '2602002') {
    return new Error(`飞书未能为当前网页应用签发授权码，请确认工作台中打开的应用与服务端 FEISHU_APP_ID 一致，并已发布最新应用版本${suffix}`)
  }
  return new Error(`${sdkErrorMessage(error)}${suffix}`)
}

function createState(): string {
  return createSecureClientUUID()
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
  iamConfig: () => api.get<IAMAuthConfig>('/api/v1/auth/iam/config'),
  iamExchange: (iamUserID?: number) => api.post<User>('/api/v1/auth/iam/exchange', iamUserID ? { iam_user_id: iamUserID } : {}),
  feishuConfig: () => api.get<FeishuAuthConfig>('/api/v1/auth/feishu/config'),
  exchange: (loginCode: FeishuLoginCode | string) => api.post<User>(
    '/api/v1/auth/feishu/exchange',
    typeof loginCode === 'string' ? { code: loginCode } : loginCode,
  ),
  logout: () => api.post<void>('/api/v1/auth/logout'),
  requestFeishuCode(appID: string): Promise<FeishuLoginCode> {
    return new Promise((resolve, reject) => {
      let started = false
      let accessStarted = false
      let legacyStarted = false
      let settled = false

      const succeed = (result: FeishuLoginCode) => {
        if (settled) return
        settled = true
        resolve(result)
      }

      const fail = (error: unknown) => {
        if (settled) return
        settled = true
        reject(sdkError(error))
      }

      const requestLegacyCode = () => {
        if (legacyStarted) return
        legacyStarted = true
        if (!window.tt?.requestAuthCode) {
          const error = new Error('requestAccess 和 requestAuthCode 均不可用')
          reportClientFailure('bridge_unavailable', error)
          if (!accessStarted) fail(new FeishuClientUnavailableError())
          else fail(error)
          return
        }
        try {
          window.tt.requestAuthCode({
            appId: appID,
            success: ({ code }) => {
              if (!code) {
                const error = new Error('飞书返回了空授权码')
                reportClientFailure('request_auth_code_empty', error)
                fail(error)
                return
              }
              succeed({ code, auth_method: 'request_auth_code' })
            },
            fail: (error) => {
              reportClientFailure('request_auth_code_fail', error)
              fail(error)
            },
          })
        } catch (error) {
          reportClientFailure('request_auth_code_throw', error)
          fail(error)
        }
      }

      const requestAccessCode = () => {
        if (accessStarted) return
        accessStarted = true
        if (!window.tt?.requestAccess) {
          requestLegacyCode()
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
                fail(error)
                return
              }
              if (!code) {
                const error = new Error('飞书返回了空授权码')
                reportClientFailure('request_access_empty', error)
                fail(error)
                return
              }
              succeed({ code, auth_method: 'request_access' })
            },
            fail: (error) => {
              reportClientFailure('request_access_fail', error)
              if (window.tt?.requestAuthCode) {
                requestLegacyCode()
                return
              }
              fail(error)
            },
          })
        } catch (error) {
          reportClientFailure('request_access_throw', error)
          if (window.tt?.requestAuthCode) {
            requestLegacyCode()
          } else {
            fail(error)
          }
        }
      }

      const execute = () => {
        if (started) return
        started = true
        if (!window.tt?.requestAccess && !window.tt?.requestAuthCode) {
          const error = new Error('requestAccess 和 requestAuthCode 均不可用')
          reportClientFailure('bridge_unavailable', error)
          fail(new FeishuClientUnavailableError())
          return
        }
        requestAccessCode()
      }
      if (window.h5sdk?.ready) window.h5sdk.ready(execute)
      else execute()
    })
  },
}
