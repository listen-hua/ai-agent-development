import { computed, onBeforeUnmount, ref, watch, type Ref } from 'vue'
import { ElMessage } from 'element-plus'
import { ApiError } from '@/services/api'
import { imageAgentService } from '@/services/image-agent'
import { createClientUUID } from '@/utils/clientId'
import { hasCartoonStrength } from '@/utils/imagePrompt'
import {
  canvasImportPositions, validateCanvasImportFiles, type CanvasImportOrigin,
} from '@/utils/imageCanvasImport'
import {
  applyLocalCanvasLayout, canvasLayoutFromCanvas, cloneCanvasLayout, mergeRemoteCanvasWithLocalLayout,
  sameCanvasLayout, type ImageCanvasLayout,
} from '@/utils/imageCanvasPersistence'
import type {
  ImageAgentOptions, ImageAsset, ImageCanvas, ImageFormState, ImageJob, ImagePromptAction, ImageViewport,
  ImageCanvasNode, BackgroundRemovalJob,
} from '@/types/image-agent'

const emptyOptions = (): ImageAgentOptions => ({ relays: [], models: [], projects: [], prompt_actions: [], background_removal_enabled: false })

interface CanvasSaveTask extends ImageCanvasLayout {
  canvasId: string
  epoch: number
  revision: number
  conflictAttempts: number
}

export function useImageWorkspace(canvasId: Ref<string>) {
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
  const deletingNodeId = ref('')
  const batchDeletingCount = ref(0)
  const generating = ref(false)
  const currentJob = ref<ImageJob>()
  const backgroundRemovalJob = ref<BackgroundRemovalJob>()
  const backgroundRemovingCount = ref(0)
  const selectedProject = computed(() => options.value.projects.find((item) => item.id === form.value.projectId))
  const selectedModel = computed(() => options.value.models.find((item) => item.id === form.value.modelId))
  let saveTimer: number | undefined
  let pollTimer: number | undefined
  let backgroundRemovalPollTimer: number | undefined
  let saveInFlight = false
  let savePromise: Promise<void> | undefined
  let pendingCanvasPatch: CanvasSaveTask | undefined
  let lastScheduledLayout: ImageCanvasLayout | undefined
  let canvasEpoch = 0
  let canvasRevision = 0
  let projectWatchReady = false
  let promptActionRequest = 0

  async function initialize() {
    loading.value = true
    try {
      if (canvas.value) await drainCanvasSave()
      beginCanvasLoad()
      canvas.value = undefined
      references.value = []
      currentJob.value = undefined
      projectWatchReady = false
      const [loaded, projectCanvas] = await Promise.all([
        imageAgentService.options(),
        imageAgentService.canvasById(canvasId.value),
      ])
      options.value = loaded
      const project = loaded.projects.find((item) => item.enabled) || loaded.projects[0]
      const relay = loaded.relays[0]
      form.value.projectId = project?.id || ''
      form.value.relayId = relay?.id || ''
      form.value.modelId = loaded.models.find((item) => item.relay_id === relay?.id)?.id || ''
      canvas.value = projectCanvas
      lastScheduledLayout = canvasLayoutFromCanvas(projectCanvas)
      if (project) options.value.prompt_actions = await imageAgentService.promptActions(project.id)
      projectWatchReady = true
    } catch (error) {
      showError(error, '读取 AI 生图配置失败')
    } finally {
      loading.value = false
    }
  }

  function beginCanvasLoad() {
    canvasEpoch++
    promptActionRequest++
    window.clearTimeout(saveTimer)
    pendingCanvasPatch = undefined
    return canvasEpoch
  }

  watch(() => form.value.projectId, async (projectId, previous) => {
    if (!projectWatchReady || !projectId || projectId === previous) return
    const request = ++promptActionRequest
    try {
      const actions = await imageAgentService.promptActions(projectId)
      if (request === promptActionRequest && projectId === form.value.projectId) options.value.prompt_actions = actions
    } catch (error) {
      showError(error, '读取项目功能按键失败')
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
	if (form.value.aspectRatio === 'original' && (!['chat_completions', 'gpt_image_2'].includes(model.protocol) || !model.supports_reference)) {
      form.value.aspectRatio = '1:1'
      ElMessage.info('当前模型不支持原图比例，已切换为 1:1')
    }
  })

  watch(() => references.value.length, (count, previous) => {
    if (previous > 0 && count === 0 && form.value.aspectRatio === 'original') {
      form.value.aspectRatio = '1:1'
      ElMessage.info('参考图已移除，画面比例已切换为 1:1')
    }
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

  async function deleteCanvasNode(nodeId: string) {
    if (!nodeId || deletingNodeId.value || !canvas.value) return
    const currentCanvasID = canvas.value.id
    deletingNodeId.value = nodeId
    try {
      await drainCanvasSave()
      let retryConflict = true
      while (canvas.value?.nodes.some((node) => node.id === nodeId)) {
        const deletedNode = canvas.value.nodes.find((node) => node.id === nodeId)
        try {
          canvas.value = await imageAgentService.deleteCanvasNodeById(currentCanvasID, nodeId, canvas.value.version)
          if (deletedNode?.asset_id) removeReference(deletedNode.asset_id)
          ElMessage.success('图片已从画布删除')
          return
        } catch (error) {
          if (error instanceof ApiError && error.status === 409 && retryConflict) {
            canvas.value = await imageAgentService.canvasById(currentCanvasID)
            retryConflict = false
            continue
          }
          throw error
        }
      }
      ElMessage.info('图片已经不在当前画布中')
    } catch (error) {
      showError(error, '删除画布图片失败')
    } finally {
      deletingNodeId.value = ''
    }
  }

  async function deleteCanvasNodes(nodeIds: string[]) {
    if (!canvas.value || deletingNodeId.value) return false
    const uniqueNodeIDs = [...new Set(nodeIds.filter(Boolean))]
    if (!uniqueNodeIDs.length) return false
    const currentCanvasID = canvas.value.id
    deletingNodeId.value = '__batch__'
    batchDeletingCount.value = uniqueNodeIDs.length
    try {
      await drainCanvasSave()
      let pendingIDs = uniqueNodeIDs
      let retryConflict = true
      while (pendingIDs.length) {
        const nodes = canvas.value?.nodes || []
        const deletingNodes = nodes.filter((node) => pendingIDs.includes(node.id))
        pendingIDs = deletingNodes.map((node) => node.id)
        if (!pendingIDs.length || !canvas.value) break
        try {
          canvas.value = await imageAgentService.deleteCanvasNodesById(currentCanvasID, pendingIDs, canvas.value.version)
          const deletedAssets = new Set(deletingNodes.map((node) => node.asset_id).filter(Boolean))
          references.value = references.value.filter((asset) => !deletedAssets.has(asset.id))
          ElMessage.success(`已从画布删除 ${pendingIDs.length} 张图片`)
          return true
        } catch (error) {
          if (error instanceof ApiError && error.status === 409 && retryConflict) {
            canvas.value = await imageAgentService.canvasById(currentCanvasID)
            retryConflict = false
            continue
          }
          throw error
        }
      }
      ElMessage.info('所选图片已经不在当前画布中')
      return true
    } catch (error) {
      showError(error, '批量删除画布图片失败，请刷新后重试')
      return false
    } finally {
      deletingNodeId.value = ''
      batchDeletingCount.value = 0
    }
  }

  async function removeBackground(nodeIds: string[]) {
    if (!canvas.value || backgroundRemovingCount.value || !options.value.background_removal_enabled) return false
    const uniqueNodeIDs = [...new Set(nodeIds.filter(Boolean))]
    if (!uniqueNodeIDs.length || uniqueNodeIDs.length > 20) {
      ElMessage.warning('每次最多选择 20 张已完成图片进行抠图')
      return false
    }
    const currentCanvasID = canvas.value.id
    backgroundRemovingCount.value = uniqueNodeIDs.length
    try {
      await drainCanvasSave()
      let retry = true
      while (canvas.value) {
        const remaining = uniqueNodeIDs.filter((id) => canvas.value?.nodes.some((node) => node.id === id))
        if (!remaining.length) throw new Error('所选图片已经不在当前画布中')
        try {
          backgroundRemovalJob.value = await imageAgentService.createBackgroundRemovalJob(currentCanvasID, {
            node_ids: remaining, version: canvas.value.version, idempotency_key: createClientUUID(),
          })
          await refreshCanvas(currentCanvasID)
          pollBackgroundRemoval(backgroundRemovalJob.value.id, currentCanvasID, canvasEpoch)
          ElMessage.success(`已创建 ${remaining.length} 张图片的智能抠图任务`)
          return true
        } catch (error) {
          if (error instanceof ApiError && error.status === 409 && retry) {
            canvas.value = await imageAgentService.canvasById(currentCanvasID)
            retry = false
            continue
          }
          throw error
        }
      }
      return false
    } catch (error) {
      backgroundRemovingCount.value = 0
      showError(error, '创建智能抠图任务失败')
      return false
    }
  }

  async function pollBackgroundRemoval(id: string, currentCanvasID: string, epoch: number) {
    if (epoch !== canvasEpoch || currentCanvasID !== canvas.value?.id) return
    try {
      const job = await imageAgentService.backgroundRemovalJob(id)
      if (epoch !== canvasEpoch || currentCanvasID !== canvas.value?.id) return
      backgroundRemovalJob.value = job
      await refreshCanvas(currentCanvasID, epoch)
      if (['succeeded', 'partial', 'failed', 'cancelled'].includes(job.status)) {
        backgroundRemovingCount.value = 0
        if (job.status === 'succeeded') ElMessage.success('智能抠图完成，透明 PNG 已放到原图右侧')
        else if (job.status === 'partial') ElMessage.warning(job.error || '部分图片抠图失败')
        else if (job.status === 'failed') ElMessage.error(job.error || '智能抠图失败')
        return
      }
      backgroundRemovalPollTimer = window.setTimeout(() => pollBackgroundRemoval(id, currentCanvasID, epoch), 1500)
    } catch (error) {
      backgroundRemovingCount.value = 0
      showError(error, '读取智能抠图任务状态失败')
    }
  }

  function scheduleCanvasSave(value: { viewport: ImageViewport; nodes: ImageCanvasNode[] }) {
    if (!canvas.value || importing.value) return
    const layout = cloneCanvasLayout(value)
    const baseline = lastScheduledLayout || canvasLayoutFromCanvas(canvas.value)
    if (sameCanvasLayout(baseline, layout)) return

    canvasRevision++
    pendingCanvasPatch = {
      ...layout,
      canvasId: canvas.value.id,
      epoch: canvasEpoch,
      revision: canvasRevision,
      conflictAttempts: 0,
    }
    lastScheduledLayout = cloneCanvasLayout(layout)
    canvas.value = applyLocalCanvasLayout(canvas.value, layout)
    window.clearTimeout(saveTimer)
    saveTimer = window.setTimeout(flushCanvasSave, 550)
  }

  function flushCanvasSave(): Promise<void> {
    if (savePromise) return savePromise
    if (!pendingCanvasPatch || !canvas.value) return Promise.resolve()
    const task = pendingCanvasPatch
    if (!isCurrentCanvasTask(task)) {
      pendingCanvasPatch = undefined
      return Promise.resolve()
    }
    saveInFlight = true
    pendingCanvasPatch = undefined
    savePromise = (async () => {
      try {
        const saved = await imageAgentService.saveCanvasById(task.canvasId, {
          viewport: task.viewport,
          nodes: task.nodes,
          version: canvas.value!.version,
        })
        if (isCurrentCanvasTask(task) && canvas.value) {
          canvas.value.version = Math.max(canvas.value.version, saved.version)
          canvas.value.updated_at = saved.updated_at
        }
      } catch (error) {
        if (error instanceof ApiError && error.status === 409) {
          await recoverCanvasConflict(task)
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

  function isCurrentCanvasTask(task: CanvasSaveTask) {
    return task.epoch === canvasEpoch
      && task.canvasId === canvas.value?.id
  }

  async function recoverCanvasConflict(task: CanvasSaveTask) {
    if (!isCurrentCanvasTask(task) || !canvas.value) return
    const remote = await imageAgentService.canvasById(task.canvasId)
    if (!isCurrentCanvasTask(task) || remote.id !== task.canvasId || !canvas.value) return

    if (task.conflictAttempts >= 1) {
      pendingCanvasPatch = undefined
      canvas.value = remote
      lastScheduledLayout = canvasLayoutFromCanvas(remote)
      ElMessage.warning('画布持续发生版本冲突，已停止自动保存并加载服务器版本')
      return
    }

    const merged = mergeRemoteCanvasWithLocalLayout(remote, canvas.value)
    canvas.value = merged
    const retryLayout = canvasLayoutFromCanvas(merged)
    canvasRevision++
    pendingCanvasPatch = {
      ...retryLayout,
      canvasId: merged.id,
      epoch: canvasEpoch,
      revision: canvasRevision,
      conflictAttempts: task.conflictAttempts + 1,
    }
    lastScheduledLayout = cloneCanvasLayout(retryLayout)
  }

  async function drainCanvasSave() {
    window.clearTimeout(saveTimer)
    while (savePromise || pendingCanvasPatch || saveInFlight) {
      if (savePromise) await savePromise
      else await flushCanvasSave()
      window.clearTimeout(saveTimer)
    }
  }

  async function refreshCanvas(currentCanvasID: string, epoch = canvasEpoch) {
    const remote = await imageAgentService.canvasById(currentCanvasID)
    if (epoch !== canvasEpoch || currentCanvasID !== canvas.value?.id) return

    const local = canvas.value
    const next = local?.id === remote.id
      ? mergeRemoteCanvasWithLocalLayout(remote, local)
      : remote
    canvas.value = next
    lastScheduledLayout = canvasLayoutFromCanvas(next)
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
    const currentCanvasID = canvas.value.id
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
            canvas.value = await imageAgentService.importCanvasAssetById(currentCanvasID, projectID, accepted[index], {
              ...positions[index],
              version: canvas.value!.version,
              origin,
            })
            succeeded++
            break
          } catch (error) {
            if (error instanceof ApiError && error.status === 409 && retryConflict) {
              canvas.value = await imageAgentService.canvasById(currentCanvasID)
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
    if (generating.value || !canvas.value || !form.value.projectId || !form.value.modelId || !form.value.relayId) return
    if (!selectedProject.value?.enabled) {
      ElMessage.warning('项目已停用，历史画布仅可查看')
      return
    }
    generating.value = true
    window.clearTimeout(pollTimer)
    window.clearTimeout(backgroundRemovalPollTimer)
    const projectId = form.value.projectId
    const currentCanvasID = canvas.value.id
    const epoch = canvasEpoch
    try {
      if (!form.value.reversePrompt) await drainCanvasSave()
      currentJob.value = await imageAgentService.createJob({
        canvas_id: currentCanvasID,
        project_id: projectId,
        relay_id: form.value.relayId,
        model_id: form.value.modelId,
        kind: form.value.reversePrompt ? 'reverse_prompt' : 'generate',
        prompt: form.value.prompt.trim(),
        aspect_ratio: form.value.aspectRatio,
        image_size: form.value.imageSize,
        count: form.value.count,
        reference_asset_ids: references.value.map((item) => item.id),
        idempotency_key: createClientUUID(),
        placement_x: center.x,
        placement_y: center.y,
        anchor_node_id: center.anchorNodeId || undefined,
      })
      if (!form.value.reversePrompt) await refreshCanvas(currentCanvasID, epoch)
      pollJob(currentJob.value.id, currentCanvasID, epoch)
    } catch (error) {
      generating.value = false
      showError(error, form.value.reversePrompt ? '创建图片反推任务失败' : '创建生图任务失败')
    }
  }

  async function pollJob(id: string, currentCanvasID: string, epoch: number) {
    if (epoch !== canvasEpoch || currentCanvasID !== canvas.value?.id) return
    try {
      const job = await imageAgentService.job(id)
      if (epoch !== canvasEpoch || currentCanvasID !== canvas.value?.id) return
      currentJob.value = job
      if (job.status === 'succeeded' || job.status === 'partial' || job.status === 'failed' || job.status === 'cancelled') {
        generating.value = false
        if (job.kind === 'reverse_prompt' && job.reversed_prompt) {
          form.value.prompt = job.reversed_prompt
          form.value.reversePrompt = false
          ElMessage.success('图片描述已反推完成')
        } else {
          await refreshCanvas(currentCanvasID, epoch)
		  const resolutionWarnings = [...new Set(job.outputs.map((item) => item.resolution_warning).filter(Boolean))]
		  if (job.status === 'succeeded' && resolutionWarnings.length) ElMessage.warning(resolutionWarnings[0]!)
		  else if (job.status === 'succeeded') ElMessage.success('图片生成完成')
          else if (job.status === 'partial') ElMessage.warning(`部分图片生成成功：${job.error || '可重试失败项'}`)
          else if (job.status === 'failed') ElMessage.error(job.error || '图片生成失败')
        }
        return
      }
      if (job.kind === 'generate') await refreshCanvas(currentCanvasID, epoch)
      pollTimer = window.setTimeout(() => pollJob(id, currentCanvasID, epoch), 1500)
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
    canvasEpoch++
    pendingCanvasPatch = undefined
    window.clearTimeout(saveTimer)
    window.clearTimeout(pollTimer)
  })

  return {
    options, canvas, references, form, loading, uploading, importing, importProgress, deletingNodeId, batchDeletingCount, generating, currentJob, backgroundRemovalJob, backgroundRemovingCount, selectedProject,
    initialize, upload, importToCanvas, addCanvasReference, removeReference, deleteCanvasNode, deleteCanvasNodes, removeBackground, scheduleCanvasSave, submit, applyAction,
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
