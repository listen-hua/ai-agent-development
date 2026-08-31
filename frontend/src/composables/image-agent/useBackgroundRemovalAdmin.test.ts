import { beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  config: vi.fn(),
  statistics: vi.fn(),
  jobs: vi.fn(),
  update: vi.fn(),
  test: vi.fn(),
  refresh: vi.fn(),
  success: vi.fn(),
  error: vi.fn(),
  warning: vi.fn(),
}))

vi.mock('element-plus', () => ({
  ElMessage: { success: mocks.success, error: mocks.error, warning: mocks.warning },
}))

vi.mock('@/services/image-agent', () => ({
  imageAgentAdminService: {
    backgroundRemovalConfig: mocks.config,
    backgroundRemovalStatistics: mocks.statistics,
    backgroundRemovalJobs: mocks.jobs,
    updateBackgroundRemovalConfig: mocks.update,
    testBackgroundRemoval: mocks.test,
    refreshBackgroundRemovalAccount: mocks.refresh,
  },
}))

import type { PixianBackgroundRemovalConfig } from '@/types/image-agent'
import { useBackgroundRemovalAdmin } from './useBackgroundRemovalAdmin'

const configured: PixianBackgroundRemovalConfig = {
  enabled: false,
  test_mode: true,
  api_id_hint: '***1234',
  api_secret_hint: '***5678',
  has_api_id: true,
  has_api_secret: true,
  timeout_seconds: 180,
  concurrency: 2,
  max_pixels: 25_000_000,
  account_credits: 10,
  updated_at: '2026-08-19T00:00:00Z',
}

describe('Pixian background removal admin workflow', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.config.mockResolvedValue(configured)
    mocks.statistics.mockResolvedValue({ today_calls: 0, thirty_day_succeeded: 0, thirty_day_failed: 0, thirty_day_images: 0, credits_charged: 0, credits_calculated: 0 })
    mocks.jobs.mockResolvedValue([])
  })

  it('shows the server reason and always releases saving state', async () => {
    mocks.update.mockRejectedValue(new Error('没有 image_manage 权限'))
    const admin = useBackgroundRemovalAdmin()

    await expect(admin.save({ enabled: false, test_mode: true, timeout_seconds: 180, concurrency: 2, max_pixels: 25_000_000 })).resolves.toBe(false)

    expect(admin.saving.value).toBe(false)
    expect(mocks.error).toHaveBeenCalledWith(expect.stringContaining('没有 image_manage 权限'))
  })

  it('requires saved credentials before running a connection test', async () => {
    const admin = useBackgroundRemovalAdmin()

    await expect(admin.test()).resolves.toBe(false)

    expect(mocks.test).not.toHaveBeenCalled()
    expect(mocks.warning).toHaveBeenCalledWith('请先保存 API ID 和 API Secret')
  })

  it('uses saved credentials and releases testing state after a failed test', async () => {
    const admin = useBackgroundRemovalAdmin()
    await admin.load()
    mocks.test.mockRejectedValue(new Error('Pixian credentials are invalid'))

    await expect(admin.test()).resolves.toBe(false)

    expect(admin.testingAction.value).toBe('')
    expect(mocks.error).toHaveBeenCalledWith(expect.stringContaining('Pixian credentials are invalid'))
  })
})
