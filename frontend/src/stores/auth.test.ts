import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '@/services/api'
import { authService } from '@/services/auth'
import * as iamService from '@/services/iam'
import type { User } from '@/types/domain'
import { useAuthStore } from './auth'

const employee: User = {
  id: 'user-1',
  feishu_open_id: 'ou_test',
  name: '测试员工',
  avatar_url: '',
  department_ids: [],
  job_title: '',
  job_level_id: '',
  job_family_id: '',
  employee_type: 0,
  status: 'active',
  roles: ['employee'],
}

describe('auth store initialization', () => {
  beforeEach(() => setActivePinia(createPinia()))
  afterEach(() => {
    vi.restoreAllMocks()
    document.body.classList.remove('iam-mode')
  })

  it('automatically signs in with the Feishu identity when no session exists', async () => {
    vi.spyOn(authService, 'me').mockRejectedValue(new ApiError('未登录', 401))
    vi.spyOn(authService, 'iamConfig').mockResolvedValue({ app_id: '', enabled: false })
    vi.spyOn(authService, 'feishuConfig').mockResolvedValue({ app_id: 'cli_test', enabled: true, dev_auth_enabled: false })
    const loginCode = { code: 'login-code', auth_method: 'request_auth_code' as const }
    vi.spyOn(authService, 'requestFeishuCode').mockResolvedValue(loginCode)
    vi.spyOn(authService, 'exchange').mockResolvedValue(employee)

    const auth = useAuthStore()
    await auth.initialize()

    expect(auth.user).toEqual(employee)
    expect(auth.initialized).toBe(true)
    expect(auth.devAuthEnabled).toBe(false)
    expect(authService.requestFeishuCode).toHaveBeenCalledWith('cli_test')
    expect(authService.exchange).toHaveBeenCalledWith(loginCode)
  })

  it('uses IAM in a normal browser and keeps IAM permissions separate from local roles', async () => {
    vi.spyOn(authService, 'iamConfig').mockResolvedValue({ app_id: 'shimmer-ai', enabled: true })
    const me = vi.spyOn(authService, 'me').mockResolvedValue(employee)
    vi.spyOn(iamService, 'initializeIAM').mockResolvedValue({
      apiURL: 'https://shimmer.example.com',
      permissions: ['agent_use', 'knowledge_manage'],
      userID: 18,
    })
    vi.spyOn(authService, 'iamExchange').mockResolvedValue({
      ...employee,
      iam_user_id: 18,
      auth_source: 'iam',
      iam_permissions: ['agent_use', 'knowledge_manage'],
    })

    const auth = useAuthStore()
    await auth.initialize()

    expect(auth.authSource).toBe('iam')
    expect(auth.can('knowledge_manage', 'knowledge_admin')).toBe(true)
    expect(auth.can('image_manage', 'image_admin')).toBe(false)
    expect(me).not.toHaveBeenCalled()
  })
})
