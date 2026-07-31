import type { IamInitSuccessState } from '@shimmer/iam-page-sdk'
import type { PermissionKey } from '@/types/domain'

export type IAMPermission = PermissionKey

export interface IAMBootstrapResult {
  apiURL: string
  permissions: IAMPermission[]
  userID: number
}

let bootstrapPromise: Promise<IAMBootstrapResult> | null = null

export function isFeishuClient(): boolean {
  const userAgent = navigator.userAgent.toLowerCase()
  return Boolean(window.tt?.requestAccess || window.tt?.requestAuthCode || userAgent.includes('lark/') || userAgent.includes('feishu'))
}

export function initializeIAM(appID: string): Promise<IAMBootstrapResult> {
  if (bootstrapPromise) return bootstrapPromise
  const pending = (async () => {
    const normalizedAppID = appID.trim()
    if (!normalizedAppID) {
      throw new Error('IAM 应用 ID 未配置')
    }
    localStorage.setItem('appId', normalizedAppID)
    document.body.classList.add('iam-mode')
    const { registIamApp } = await import('@shimmer/iam-page-sdk')
    return new Promise<IAMBootstrapResult>((resolve, reject) => {
      const timeout = window.setTimeout(() => reject(new Error('IAM 初始化超时，请刷新页面重试')), 30_000)
      try {
        registIamApp({
          onIamStateChange: (state) => {
            if (!state.init_success) return
            window.clearTimeout(timeout)
            resolve(toBootstrapResult(state, normalizedAppID))
          },
        })
      } catch (error) {
        window.clearTimeout(timeout)
        reject(error)
      }
    })
  })().catch((error: unknown) => {
    bootstrapPromise = null
    throw error
  })
  bootstrapPromise = pending
  return pending
}

function toBootstrapResult(state: IamInitSuccessState, fallbackAppID: string): IAMBootstrapResult {
  const currentAppID = state.currentAppId || fallbackAppID
  const permissions = state.user_authorization.effect_policy
    .filter((policy) => policy.app_id === currentAppID)
    .map((policy) => policy.permission_key)
    .filter(isIAMPermission)
  return {
    apiURL: state.currentApp?.api_url?.trim() || window.location.origin,
    permissions: [...new Set(permissions)],
    userID: state.user_authorization.id,
  }
}

function isIAMPermission(value: string): value is IAMPermission {
  return [
    'agent_use',
    'knowledge_manage',
    'agent_manage',
    'image_manage',
    'notification_manage',
    'calendar_manage',
    'audit_view',
    'user_manage',
  ].includes(value)
}
