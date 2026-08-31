import { computed, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { ApiError } from '@/services/api'
import { imageAgentAdminService } from '@/services/image-agent'
import type { BackgroundRemovalJob, BackgroundRemovalStatistics, PixianBackgroundRemovalConfig } from '@/types/image-agent'

export interface BackgroundRemovalConfigInput {
  enabled: boolean
  test_mode: boolean
  api_id?: string
  api_secret?: string
  timeout_seconds: number
  concurrency: number
  max_pixels: number
}

function errorDetail(error: unknown): string {
  if (error instanceof ApiError) return error.detail || error.message
  return error instanceof Error ? error.message : '未知错误'
}

export function useBackgroundRemovalAdmin() {
  const config = ref<PixianBackgroundRemovalConfig>()
  const statistics = ref<BackgroundRemovalStatistics>()
  const jobs = ref<BackgroundRemovalJob[]>([])
  const loading = ref(false)
  const saving = ref(false)
  const testingAction = ref<'test' | 'refresh' | ''>('')
  const loadError = ref('')
  const credentialsConfigured = computed(() => Boolean(config.value?.has_api_id && config.value?.has_api_secret))

  async function load(): Promise<boolean> {
    loading.value = true
    loadError.value = ''
    try {
      const result = await Promise.all([
        imageAgentAdminService.backgroundRemovalConfig(),
        imageAgentAdminService.backgroundRemovalStatistics(),
        imageAgentAdminService.backgroundRemovalJobs(),
      ])
      ;[config.value, statistics.value, jobs.value] = result
      return true
    } catch (error) {
      loadError.value = `抠图服务配置加载失败：${errorDetail(error)}`
      ElMessage.error(loadError.value)
      return false
    } finally {
      loading.value = false
    }
  }

  async function save(input: BackgroundRemovalConfigInput): Promise<boolean> {
    saving.value = true
    try {
      config.value = await imageAgentAdminService.updateBackgroundRemovalConfig(input)
      loadError.value = ''
      ElMessage.success('Pixian 抠图服务配置已保存')
      return true
    } catch (error) {
      ElMessage.error(`保存 Pixian 配置失败：${errorDetail(error)}`)
      return false
    } finally {
      saving.value = false
    }
  }

  async function test(refresh = false): Promise<boolean> {
    if (!credentialsConfigured.value) {
      ElMessage.warning('请先保存 API ID 和 API Secret')
      return false
    }
    testingAction.value = refresh ? 'refresh' : 'test'
    try {
      config.value = refresh
        ? await imageAgentAdminService.refreshBackgroundRemovalAccount()
        : await imageAgentAdminService.testBackgroundRemoval()
      ElMessage.success(`Pixian 连接成功，剩余 ${config.value.account_credits} Credits`)
      return true
    } catch (error) {
      ElMessage.error(`${refresh ? '刷新余额' : 'Pixian 连接测试'}失败：${errorDetail(error)}`)
      return false
    } finally {
      testingAction.value = ''
    }
  }

  return { config, statistics, jobs, loading, saving, testingAction, loadError, credentialsConfigured, load, save, test }
}
