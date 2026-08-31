import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { ApiError, setApiBaseURL } from '@/services/api'
import { authService, FeishuClientUnavailableError, type FeishuAuthConfig } from '@/services/auth'
import { initializeIAM, isFeishuClient } from '@/services/iam'
import type { AuthSource, PermissionKey, User } from '@/types/domain'

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const initialized = ref(false)
  const isAuthenticating = ref(false)
  const isIAMInitializing = ref(false)
  const authError = ref('')
  const devAuthEnabled = ref(false)
  const iamConfigured = ref(false)
  const authSource = ref<AuthSource | ''>('')
  const permissions = ref<PermissionKey[]>([])
  let initializationPromise: Promise<void> | null = null

  const isIAM = computed(() => authSource.value === 'iam')
  const isAdmin = computed(() => permissions.value.some((permission) => permission !== 'agent_use'))
  const hasPermission = (permission: PermissionKey) => permissions.value.includes(permission)
  const can = (permission: PermissionKey) => hasPermission(permission)
  function applyUser(value: User, fallbackSource: AuthSource) {
    user.value = value
    authSource.value = value.auth_source || fallbackSource
    permissions.value = (value.permissions || []).filter(isPermissionKey)
  }
  function acceptResolvedUser(value: User) {
    applyUser(value, (authSource.value || value.auth_source || 'feishu') as AuthSource)
  }
  function clearResolvedUser() {
    user.value = null
    authSource.value = ''
    permissions.value = []
  }

  async function signInWithFeishu(authConfig?: FeishuAuthConfig) {
    isAuthenticating.value = true
    authError.value = ''
    try {
      const config = authConfig || await authService.feishuConfig()
      devAuthEnabled.value = config.dev_auth_enabled
      if (!config.enabled || !config.app_id) throw new Error('服务端尚未配置飞书应用凭证')
      const code = await authService.requestFeishuCode(config.app_id)
      applyUser(await authService.exchange(code), 'feishu')
      document.body.classList.remove('iam-mode')
    } finally {
      isAuthenticating.value = false
    }
  }

  async function initializeIAMBrowser(appID: string) {
    isIAMInitializing.value = true
    isAuthenticating.value = true
    authError.value = ''
    try {
      const state = await initializeIAM(appID)
      setApiBaseURL(state.apiURL)
      const authenticatedUser = await authService.iamExchange(state.userID)
      if (authenticatedUser.iam_user_id && authenticatedUser.iam_user_id !== state.userID) {
        throw new Error('IAM 页面身份与服务端身份不一致，请重新登录')
      }
      applyUser(authenticatedUser, 'iam')
    } finally {
      isIAMInitializing.value = false
      isAuthenticating.value = false
    }
  }

  async function initialize() {
    if (initialized.value) return
    if (initializationPromise) return initializationPromise
    initializationPromise = (async () => {
      const iamConfig = await authService.iamConfig().catch(() => ({ app_id: '', enabled: false }))
      iamConfigured.value = iamConfig.enabled
      if (!isFeishuClient() && iamConfig.enabled) {
        try {
          await initializeIAMBrowser(iamConfig.app_id)
        } catch (error) {
          user.value = null
          authSource.value = ''
          authError.value = formatIAMError(error)
        }
        return
      }

      const configPromise = authService.feishuConfig().then((config) => {
        devAuthEnabled.value = config.dev_auth_enabled
        return config
      }).catch(() => undefined)
      try {
        applyUser(await authService.me(), 'feishu')
        await configPromise
        return
      } catch (error) {
        user.value = null
        const unauthenticated = error instanceof ApiError && (error.status === 401 || error.code === 510000)
        if (!unauthenticated) {
          authError.value = error instanceof Error ? error.message : '读取登录状态失败'
          return
        }
      }

      try {
        await signInWithFeishu(await configPromise)
      } catch (error) {
        if (!(error instanceof FeishuClientUnavailableError)) {
          authError.value = error instanceof Error ? error.message : '飞书免登失败'
        }
      }
    })().finally(() => {
      initialized.value = true
      initializationPromise = null
    })
    return initializationPromise
  }

  async function devLogin(asEmployee = false) {
    applyUser(await authService.exchange(asEmployee ? 'dev:employee' : 'dev:admin'), 'dev')
    authError.value = ''
    initialized.value = true
  }

  async function feishuLogin() {
    await signInWithFeishu()
    initialized.value = true
  }

  async function recoverSession() {
    authError.value = ''
    try {
      if (isFeishuClient()) {
        await signInWithFeishu()
      } else {
        const config = await authService.iamConfig()
        iamConfigured.value = config.enabled
        if (!config.enabled || !config.app_id) {
          throw new Error('登录已过期，请重新登录')
        }
        await initializeIAMBrowser(config.app_id)
      }
      initialized.value = true
    } catch (error) {
      clearResolvedUser()
      initialized.value = false
      authError.value = isFeishuClient()
        ? (error instanceof Error ? error.message : '飞书登录已过期，请重新登录')
        : formatIAMError(error)
      throw error
    }
  }

  async function logout() {
    await authService.logout()
    clearResolvedUser()
  }

  return {
    user,
    initialized,
    isAuthenticating,
    isIAMInitializing,
    authError,
    devAuthEnabled,
    iamConfigured,
    authSource,
    permissions,
    isIAM,
    isAdmin,
    hasPermission,
    can,
    acceptResolvedUser,
    initialize,
    recoverSession,
    devLogin,
    feishuLogin,
    logout,
  }
})

function isPermissionKey(value: string): value is PermissionKey {
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

function formatIAMError(error: unknown): string {
  if (error instanceof ApiError && error.code === 510001) {
    return '当前 IAM 账号没有访问微光的权限，请联系管理员'
  }
  if (error instanceof ApiError && error.code === 510000) {
    return 'IAM 登录已失效，请通过上方 IAM 导航重新登录'
  }
  return error instanceof Error ? error.message : 'IAM 登录失败，请刷新页面重试'
}
