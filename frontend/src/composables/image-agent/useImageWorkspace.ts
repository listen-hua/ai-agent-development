import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { ApiError } from '@/services/api'
import { imageAgentService } from '@/services/image-agent'
import { hasCartoonStrength } from '@/utils/imagePrompt'
import {
  canvasImportPositions, validateCanvasImportFiles, type CanvasImportOrigin,
} from '@/utils/imageCanvasImport'
import type {
  ImageAgentOptions, ImageAsset, ImageCanvas, ImageFormState, ImageJob, ImagePromptAction, ImageViewport,
  ImageCanvasNode,
} from '@/types/image-agent'

const emptyOptions = (): ImageAgentOptions => ({ relays: [], models: [], projects: [], prompt_actions: [] })

export function useImageWorkspace() {
  const options = ref<ImageAgentOptions>(emptyOptions())
  const canvas = ref<ImageCanvas>()
  const references = ref<ImageAsset[]>([])
  const form = ref<ImageFormState>({
    prompt: '',
    relayId: '',
    modelId: '',
    projectId: '',
    reversePrompt: false,
    aspectRatio: '1:1',
    imageSize: '1K',
    count: 1,
  })
  const loading = ref(true)
  const uploading = ref(false)
  const importing = ref(false)
  const importProgress = ref('')
  const generating = ref(false)
  const currentJob = ref<ImageJob>()
  const selectedProject = computed(() => options.value.projects.find((item) => item.id === form.value.projectId))
  const selectedModel = computed(() => options.value.models.find((item) => item.id === form.value.modelId))
  let saveTimer: number | undefined
  let pollTimer: number | undefined
  let saveInFlight = false
  let savePromise: Promise<void> | undefined
  let pendingCanvasPatch: { viewport: ImageViewport; nodes: ImageCanvasNode[] } | undefined
  let projectWatchReady = false

  async function initialize() {
    loading.value = true
    try {
      const loaded = await imageAgentService.options()
      options.value = loaded
      const project = loaded.projects.find((item) => item.enabled) || loaded.projects[0]
      const relay = loaded.relays[0]
      form.value.projectId = project?.id || ''
      form.value.relayId = relay?.id || ''
      form.value.modelId = loaded.models.find((item) => item.relay_id === relay?.id)?.id || ''
      if (project) {
        await loadProject(project.id)
      }
      projectWatchReady = true
    } catch (error) {
      showError(error, '读取 AI 生图配置失败')
    } finally {
      loading.value = false
    }
  }

  async function loadProject(projectId: string) {
    if (!projectId) {
      canvas.value = undefined
      references.value = []
      return
    }
    references.value = []
    const [projectOptions, projectCanvas] = await Promise.all([
      imageAgentService.options(projectId),
      imageAgentService.canvas(projectId),
    ])
    options.value = { ...projectOptions, prompt_actions: projectOptions.prompt_actions }
    canvas.value = projectCanvas
  }

  watch(() => form.value.projectId, async (projectId, previous) => {
    if (!projectWatchReady || !projectId || projectId === previous) return
    loading.value = true
    try {
      await loadProject(projectId)
    } catch (error) {
      showError(error, '切换项目失败')
    } finally {
      loading.value = false
    }
  })

  watch(() => form.value.relayId, (relayId) => {
    if (!options.value.models.some((item) => item.relay_id === relayId && item.id === form.value.modelId)) {
      form.value.modelId = options.value.models.find((item) => item.relay_id === relayId)?.id || ''
    }
  })

  watch(selectedModel, (model) => {
    if (!model) return
    if (!model.supported_sizes.includes(form.value.imageSize)) form.value.imageSize = model.supported_sizes[0] || '1K'
    if (form.value.count > model.max_count) form.value.count = model.max_count
    if (!model.supports_reverse) form.value.reversePrompt = false
  })

  async function upload(files: File[]) {
    if (!form.value.projectId) return
    const allowed = ['image/jpeg', 'image/png', 'image/webp']
    const available = 5 - references.value.length
    const selected = files.slice(0, available)
    if (!selected.length) {
      ElMessage.warning('参考图最多 5 张')
      return
    }
    if (selected.some((file) => !allowed.includes(file.type))) {
      ElMessage.warning('只支持 JPG、PNG 和 WEBP 图片')
      return
    }
    if (selected.some((file) => file.size > 10 * 1024 * 1024)) {
      ElMessage.warning('单张参考图不能超过 10 MB')
      return
    }
    const existingTotal = references.value.reduce((sum, item) => sum + item.size_bytes, 0)
    const incomingTotal = selected.reduce((sum, item) => sum + item.size, 0)
    if (existingTotal + incomingTotal > 30 * 1024 * 1024) {
      ElMessage.warning('参考图总大小不能超过 30 MB')
      return
    }
    uploading.value = true
    try {
      for (const file of selected) {
        references.value.push(await imageAgentService.uploadAsset(file, form.value.projectId))
      }
      if (files.length > selected.length) ElMessage.warning('超出 5 张的图片未上传')
    } catch (error) {
      showError(error, '上传参考图失败')
    } finally {
      uploading.value = false
    }
  }

  function addCanvasReference(assetId: string, index?: number) {
    const existingIndex = references.value.findIndex((item) => item.id === assetId)
    if (existingIndex >= 0 && (index === undefined || existingIndex === index)) {
      ElMessage.info('这张图片已经在参考图中')
      return
    }
    if (existingIndex < 0 && references.value.length >= 5 && (index === undefined || index >= references.value.length)) {
      ElMessage.warning('参考图最多 5 张')
      return
    }
    const asset: ImageAsset = {
      id: assetId,
      owner_id: '',
      project_id: form.value.projectId,
      mime_type: 'image/png',
      file_name: '画布图片',
      width: 0,
      height: 0,
      size_bytes: 0,
      source: 'generated',
      created_at: new Date().toISOString(),
    }
    if (existingIndex >= 0) {
      references.value.splice(existingIndex, 1)
      if (index !== undefined && existingIndex < index) index--
    }
    if (index !== undefined && index < references.value.length) references.value.splice(index, 1, asset)
    else references.value.push(asset)
  }

  function removeReference(id: string) {
    references.value = references.value.filter((item) => item.id !== id)
  }

  function scheduleCanvasSave(value: { viewport: ImageViewport; nodes: ImageCanvasNode[] }) {
    if (!canvas.value || !selectedProject.value?.enabled || importing.value) return
    pendingCanvasPatch = value
    window.clearTimeout(saveTimer)
    saveTimer = window.setTimeout(flushCanvasSave, 550)
  }

  function flushCanvasSave(): Promise<void> {
    if (savePromise) return savePromise
    if (!pendingCanvasPatch || !canvas.value) return Promise.resolve()
    saveInFlight = true
    const patch = pendingCanvasPatch
    pendingCanvasPatch = undefined
    savePromise = (async () => {
      try {
        const saved = await imageAgentService.saveCanvas(form.value.projectId, {
          viewport: patch.viewport,
          nodes: patch.nodes,
          version: canvas.value!.version,
        })
        canvas.value!.version = saved.version
        canvas.value!.viewport = patch.viewport
        canvas.value!.nodes = patch.nodes
      } catch (error) {
        if (error instanceof ApiError && error.status === 409) {
          canvas.value = await imageAgentService.canvas(form.value.projectId)
          ElMessage.warning('画布已在其他页面更新，已加载最新版本')
        } else {
          showError(error, '保存画布位置失败')
        }
      }
    })().finally(() => {
      saveInFlight = false
      savePromise = undefined
      if (pendingCanvasPatch) saveTimer = window.setTimeout(flushCanvasSave, 100)
    })
    return savePromise
  }

  async function drainCanvasSave() {
    window.clearTimeout(saveTimer)
    while (savePromise || pendingCanvasPatch || saveInFlight) {
      if (savePromise) await savePromise
      else await flushCanvasSave()
      window.clearTimeout(saveTimer)
    }
  }

  async function importToCanvas(files: File[], point: { x: number; y: number }, origin: CanvasImportOrigin) {
    if (importing.value) {
      ElMessage.info('图片正在导入，请稍候')
      return
    }
    if (!form.value.projectId || !canvas.value) {
      ElMessage.warning('请先选择一个生图项目')
      return
    }
    if (!selectedProject.value?.enabled) {
      ElMessage.warning('项目已停用，历史画布只读')
      return
    }
    const { accepted, rejected } = validateCanvasImportFiles(files)
    if (!accepted.length) {
      ElMessage.warning(importRejectionMessage(rejected))
      return
    }
    importing.value = true
    const projectID = form.value.projectId
    let succeeded = 0
    let failed = rejected.length
    let lastError: unknown
    try {
      await drainCanvasSave()
      const positions = canvasImportPositions(point, accepted.length)
      let stop = false
      for (let index = 0; index < accepted.length && !stop; index++) {
        importProgress.value = `正在导入 ${index + 1}/${accepted.length}`
        let retryConflict = true
        while (true) {
          try {
            canvas.value = await imageAgentService.importCanvasAsset(projectID, accepted[index], {
              ...positions[index],
              version: canvas.value!.version,
              origin,
            })
            succeeded++
            break
          } catch (error) {
            if (error instanceof ApiError && error.status === 409 && retryConflict) {
              canvas.value = await imageAgentService.canvas(projectID)
              retryConflict = false
              continue
            }
            lastError = error
            failed++
            if (error instanceof ApiError && error.status === 409) {
              failed += accepted.length - index - 1
              stop = true
              ElMessage.warning('画布持续发生版本冲突，已停止导入剩余图片')
            }
            break
          }
        }
      }
      if (succeeded && failed) ElMessage.warning(`已导入 ${succeeded} 张，${failed} 张未导入`)
      else if (succeeded) ElMessage.success(`已将 ${succeeded} 张图片添加到画布`)
      else showError(lastError, '图片导入失败')
    } catch (error) {
      showError(error, '准备画布导入失败')
    } finally {
      importing.value = false
      importProgress.value = ''
    }
  }

  async function submit(center: { x: number; y: number; anchorNodeId?: string } = { x: 0, y: 0 }) {
    if (generating.value || !form.value.projectId || !form.value.modelId || !form.value.relayId) return
    if (!selectedProject.value?.enabled) {
      ElMessage.warning('项目已停用，历史画布仅可查看')
      return
    }
    generating.value = true
    window.clearTimeout(pollTimer)
    try {
      if (!form.value.reversePrompt) await drainCanvasSave()
      currentJob.value = await imageAgentService.createJob({
        project_id: form.value.projectId,
        relay_id: form.value.relayId,
        model_id: form.value.modelId,
        kind: form.value.reversePrompt ? 'reverse_prompt' : 'generate',
        prompt: form.value.prompt.trim(),
        aspect_ratio: form.value.aspectRatio,
        image_size: form.value.imageSize,
        count: form.value.count,
        reference_asset_ids: references.value.map((item) => item.id),
        idempotency_key: crypto.randomUUID(),
        placement_x: center.x,
        placement_y: center.y,
        anchor_node_id: center.anchorNodeId || undefined,
      })
      if (!form.value.reversePrompt) canvas.value = await imageAgentService.canvas(form.value.projectId)
      pollJob(currentJob.value.id)
    } catch (error) {
      generating.value = false
      showError(error, form.value.reversePrompt ? '创建图片反推任务失败' : '创建生图任务失败')
    }
  }

  async function pollJob(id: string) {
    try {
      const job = await imageAgentService.job(id)
      currentJob.value = job
      if (job.status === 'succeeded' || job.status === 'partial' || job.status === 'failed' || job.status === 'cancelled') {
        generating.value = false
        if (job.kind === 'reverse_prompt' && job.reversed_prompt) {
          form.value.prompt = job.reversed_prompt
          form.value.reversePrompt = false
          ElMessage.success('图片描述已反推完成')
        } else {
          canvas.value = await imageAgentService.canvas(form.value.projectId)
          if (job.status === 'succeeded') ElMessage.success('图片生成完成')
          else if (job.status === 'partial') ElMessage.warning(`部分图片生成成功：${job.error || '可重试失败项'}`)
          else if (job.status === 'failed') ElMessage.error(job.error || '图片生成失败')
        }
        return
      }
      if (job.kind === 'generate') canvas.value = await imageAgentService.canvas(form.value.projectId)
      pollTimer = window.setTimeout(() => pollJob(id), 1500)
    } catch (error) {
      generating.value = false
      showError(error, '读取生图任务状态失败')
    }
  }

  async function applyAction(
    action: ImagePromptAction,
    center: { x: number; y: number; anchorNodeId?: string } = { x: 0, y: 0 },
  ) {
    form.value.prompt = action.prompt_template
    if (!form.value.reversePrompt && !hasCartoonStrength(action.prompt_template)) {
      await submit(center)
    }
  }

  onBeforeUnmount(() => {
    window.clearTimeout(saveTimer)
    window.clearTimeout(pollTimer)
  })

  return {
    options, canvas, references, form, loading, uploading, importing, importProgress, generating, currentJob, selectedProject,
    initialize, upload, importToCanvas, addCanvasReference, removeReference, scheduleCanvasSave, submit, applyAction,
  }
}

function importRejectionMessage(rejected: ReturnType<typeof validateCanvasImportFiles>['rejected']) {
  if (rejected.some((item) => item.reason === 'limit')) return '每次最多导入 20 张图片'
  if (rejected.some((item) => item.reason === 'size')) return '单张图片不能超过 10 MB'
  return '只支持 JPG、PNG 和 WEBP 图片文件'
}

function showError(error: unknown, fallback: string) {
  if (error instanceof ApiError && error.detail) {
    ElMessage.error(error.detail)
  } else {
    ElMessage.error(error instanceof Error ? error.message : fallback)
  }
}
