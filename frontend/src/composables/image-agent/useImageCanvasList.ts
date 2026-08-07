import { ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { imageAgentService } from '@/services/image-agent'
import { ApiError } from '@/services/api'
import type { ImageCanvas } from '@/types/image-agent'

export function useImageCanvasList() {
  const canvases = ref<ImageCanvas[]>([])
  const deletedCanvases = ref<ImageCanvas[]>([])
  const loading = ref(false)
  const creating = ref(false)
  const actingId = ref('')

  async function load() {
    loading.value = true
    try { canvases.value = await imageAgentService.canvases() }
    catch (error) { showError(error, '读取画布列表失败') }
    finally { loading.value = false }
  }
  async function loadTrash() {
    try { deletedCanvases.value = await imageAgentService.deletedCanvases() }
    catch (error) { showError(error, '读取回收站失败') }
  }
  async function create(name: string) {
    creating.value = true
    try {
      const canvas = await imageAgentService.createCanvas(name.trim())
      canvases.value.unshift(canvas)
      ElMessage.success('画布已创建')
      return canvas
    } catch (error) {
      showError(error, '创建画布失败')
      throw error
    } finally { creating.value = false }
  }
  async function remove(canvas: ImageCanvas) {
    try {
      await ElMessageBox.confirm(`“${canvas.name}”将移入回收站并保留 30 天。`, '删除画布', { type: 'warning', confirmButtonText: '移入回收站' })
    } catch {
      return
    }
    actingId.value = canvas.id
    try {
      await imageAgentService.removeCanvas(canvas.id)
      canvases.value = canvases.value.filter(item => item.id !== canvas.id)
      ElMessage.success('画布已移入回收站')
    } catch (error) {
      showError(error, '删除画布失败')
    } finally { actingId.value = '' }
  }
  async function restore(canvas: ImageCanvas) {
    actingId.value = canvas.id
    try {
      const restored = await imageAgentService.restoreCanvas(canvas.id)
      deletedCanvases.value = deletedCanvases.value.filter(item => item.id !== canvas.id)
      canvases.value.unshift(restored)
      ElMessage.success('画布已恢复')
    } catch (error) {
      showError(error, '恢复画布失败')
    } finally { actingId.value = '' }
  }
  return { canvases, deletedCanvases, loading, creating, actingId, load, loadTrash, create, remove, restore }
}

function showError(error: unknown, fallback: string) {
  ElMessage.error(error instanceof ApiError ? (error.detail || error.message) : error instanceof Error ? error.message : fallback)
}
