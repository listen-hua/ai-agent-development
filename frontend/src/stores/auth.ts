import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { ApiError } from '@/services/api'
import { authService, FeishuClientUnavailableError, type FeishuAuthConfig } from '@/services/auth'
import type { Role, User } from '@/types/domain'

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const initialized = ref(false)
  const isAuthenticating = ref(false)
  const authError = ref('')
  const devAuthEnabled = ref(false)
  let initializationPromise: Promise<void> | null = null
  const isAdmin = computed(() => Boolean(user.value?.roles.some((role) => role !== 'employee')))
  const hasRole = (...roles: Role[]) => Boolean(user.value?.roles.includes('super_admin') || roles.some((role) => user.value?.roles.includes(role)))

  async function signInWithFeishu(authConfig?: FeishuAuthConfig) {
    isAuthenticating.value = true
    authError.value = ''
    try {
      const config = authConfig || await authService.feishuConfig()
      devAuthEnabled.value = config.dev_auth_enabled
      if (!config.enabled || !config.app_id) throw new Error('服务端尚未配置飞书应用凭证')
      const code = await authService.requestFeishuCode(config.app_id)
      user.value = await authService.exchange(code)
    } finally {
      isAuthenticating.value = false
    }
  }

  async function initialize() {
    if (initialized.value) return
    if (initializationPromise) return initializationPromise
    initializationPromise = (async () => {
      const configPromise = authService.feishuConfig().then((config) => {
        devAuthEnabled.value = config.dev_auth_enabled
        return config
      }).catch(() => undefined)
      try {
        user.value = await authService.me()
        await configPromise
        return
      } catch (error) {
        user.value = null
        if (!(error instanceof ApiError) || error.status !== 401) {
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
    user.value = await authService.exchange(asEmployee ? 'dev:employee' : 'dev:admin')
    authError.value = ''
    initialized.value = true
  }

  async function feishuLogin() {
    await signInWithFeishu()
    initialized.value = true
  }

  async function logout() { await authService.logout(); user.value = null }
  return { user, initialized, isAuthenticating, authError, devAuthEnabled, isAdmin, hasRole, initialize, devLogin, feishuLogin, logout }
})
