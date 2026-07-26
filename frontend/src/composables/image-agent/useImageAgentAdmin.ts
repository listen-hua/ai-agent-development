import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { directoryService } from '@/services/admin'
import { imageAgentAdminService } from '@/services/image-agent'
import type { DirectoryOptions } from '@/types/domain'
import type {
  ImageModel, ImageModelInput, ImageProject, ImageProjectInput, ImagePromptAction, ImagePromptActionInput,
  ImageRelay, ImageRelayInput,
} from '@/types/image-agent'

export function useImageAgentAdmin() {
  const relays = ref<ImageRelay[]>([])
  const models = ref<ImageModel[]>([])
  const projects = ref<ImageProject[]>([])
  const promptActions = ref<ImagePromptAction[]>([])
  const directory = ref<DirectoryOptions>({ departments: [], job_titles: [], users: [] })
  const loading = ref(true)
  const saving = ref(false)
  const acting = ref('')

  async function load() {
    loading.value = true
    try {
      ;[relays.value, models.value, projects.value, promptActions.value, directory.value] = await Promise.all([
        imageAgentAdminService.relays(),
        imageAgentAdminService.models(),
        imageAgentAdminService.projects(),
        imageAgentAdminService.promptActions(),
        directoryService.options(),
      ])
    } finally {
      loading.value = false
    }
  }

  async function saveRelay(id: string | undefined, input: ImageRelayInput) {
    saving.value = true
    try {
      const value = id
        ? await imageAgentAdminService.updateRelay(id, input)
        : await imageAgentAdminService.createRelay(input)
      upsert(relays.value, value)
      ElMessage.success('中转站配置已保存')
      return value
    } finally {
      saving.value = false
    }
  }

  async function testRelay(id: string) {
    acting.value = `test:${id}`
    try {
      await imageAgentAdminService.testRelay(id)
      ElMessage.success('连接成功，API Key 与模型接口可用')
    } finally {
      acting.value = ''
    }
  }

  async function syncModels(id: string) {
    acting.value = `sync:${id}`
    try {
      const values = await imageAgentAdminService.syncModels(id)
      models.value = [...models.value.filter((item) => item.relay_id !== id), ...values]
      ElMessage.success(`已同步 ${values.length} 个模型，请明确启用生图模型`)
    } finally {
      acting.value = ''
    }
  }

  async function saveModel(id: string, input: ImageModelInput) {
    saving.value = true
    try {
      const value = await imageAgentAdminService.updateModel(id, input)
      upsert(models.value, value)
      ElMessage.success('模型能力配置已保存')
      return value
    } finally {
      saving.value = false
    }
  }

  async function saveProject(id: string | undefined, input: ImageProjectInput) {
    saving.value = true
    try {
      const value = id
        ? await imageAgentAdminService.updateProject(id, input)
        : await imageAgentAdminService.createProject(input)
      upsert(projects.value, value)
      ElMessage.success('项目配置已保存')
      return value
    } finally {
      saving.value = false
    }
  }

  async function savePromptAction(id: string | undefined, input: ImagePromptActionInput) {
    saving.value = true
    try {
      const value = id
        ? await imageAgentAdminService.updatePromptAction(id, input)
        : await imageAgentAdminService.createPromptAction(input)
      upsert(promptActions.value, value)
      ElMessage.success('功能按键已保存')
      return value
    } finally {
      saving.value = false
    }
  }

  async function deletePromptAction(id: string) {
    await imageAgentAdminService.deletePromptAction(id)
    promptActions.value = promptActions.value.filter((item) => item.id !== id)
    ElMessage.success('功能按键已删除')
  }

  return {
    relays, models, projects, promptActions, directory, loading, saving, acting,
    load, saveRelay, testRelay, syncModels, saveModel, saveProject, savePromptAction, deletePromptAction,
  }
}

function upsert<T extends { id: string }>(values: T[], value: T) {
  const index = values.findIndex((item) => item.id === value.id)
  if (index >= 0) values.splice(index, 1, value)
  else values.push(value)
}
