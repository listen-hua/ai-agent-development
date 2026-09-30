import { computed, onBeforeUnmount, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { directoryService } from '@/services/admin'
import { ApiError } from '@/services/api'
import { massageService, type MassageCycleInput } from '@/services/massage'
import type { DirectoryOptions, MassageCycle, MassageStatistic } from '@/types/domain'

export function useMassageAdmin() {
  const cycles = ref<MassageCycle[]>([])
  const selectedId = ref('')
  const statistics = ref<MassageStatistic>()
  const directory = ref<DirectoryOptions>({ departments: [], job_titles: [], users: [] })
  const loading = ref(false)
  const saving = ref(false)
  const busy = ref(false)
  let timer = 0
  const selected = computed(() => cycles.value.find(cycle => cycle.id === selectedId.value))

  async function load() {
    loading.value = true
    try {
      cycles.value = await massageService.list()
      if (!cycles.value.some(cycle => cycle.id === selectedId.value)) selectedId.value = cycles.value[0]?.id || ''
      if (selectedId.value) await loadStatistics()
	  else statistics.value = undefined
    } finally { loading.value = false }
  }
  async function loadDirectory() { directory.value = await directoryService.options() }
  async function loadStatistics() { if (selectedId.value) statistics.value = await massageService.statistics(selectedId.value) }
  async function save(input: MassageCycleInput, cycle?: MassageCycle) {
    saving.value = true
    try {
      const result = cycle ? await massageService.update(cycle.id, input) : await massageService.create(input)
      ElMessage.success(cycle ? '批次已更新' : '批次已创建')
      await load()
      selectedId.value = result.id
      return true
    } catch (error) {
      ElMessage.error(error instanceof ApiError ? (error.detail || error.message) : error instanceof Error ? error.message : '保存失败')
      return false
    } finally { saving.value = false }
  }
  async function publish(id: string) {
    await ElMessageBox.confirm('发布后将冻结参与范围，并按计划时间发送飞书报名卡片。', '发布按摩报名', { type: 'warning' })
    await massageService.publish(id)
    ElMessage.success('报名批次已发布')
    await load()
  }
  async function remove(cycle: MassageCycle) {
    await ElMessageBox.confirm(`删除“${cycle.title}”后，报名名单、排号、叫号记录和待发送通知都会永久删除，无法恢复。`, '删除按摩批次', { type: 'error', confirmButtonText: '确认删除', cancelButtonText: '取消' })
    busy.value = true
    try {
      await massageService.remove(cycle.id)
      ElMessage.success('按摩批次已删除')
      if (selectedId.value === cycle.id) selectedId.value = ''
      await load()
    } finally { busy.value = false }
  }
  async function resend() {
    if (!selected.value) return
    const result = await massageService.resend(selected.value.id)
    ElMessage.success(`已创建 ${result.count} 条补发任务`)
  }
  async function sessionAction(id: string, action: 'start' | 'pause' | 'resume' | 'close') {
    let reason = ''
    if (action === 'start') await ElMessageBox.confirm('请确认按摩师和现场已经准备完成。开始后会立即按并行人数叫号。', '开始叫号', { type: 'warning' })
    if (action === 'close') {
      const result = await ElMessageBox.prompt('请输入提前结束或队列耗尽的原因', '结束场次', { inputValidator: value => Boolean(value.trim()) || '必须填写原因' })
      reason = result.value
    }
    busy.value = true
    try {
      await massageService.sessionAction(id, action, reason)
      ElMessage.success('场次状态已更新')
      await load()
    } finally { busy.value = false }
  }
  async function callAction(id: string, action: 'complete' | 'no-show') {
    if (action === 'no-show') await ElMessageBox.confirm('确认该同事未到场并跳号吗？', '未到场', { type: 'warning' })
    await massageService.callAction(id, action)
    ElMessage.success(action === 'complete' ? '已完成，系统将继续叫号' : '已跳号')
    await load()
  }
  function select(id: string) { selectedId.value = id; void loadStatistics() }
  function stopPolling() { window.clearInterval(timer); timer = 0 }
  function startPolling() {
    stopPolling()
    timer = window.setInterval(() => { if (selected.value?.sessions.some(session => session.status === 'running')) void load() }, 5000)
  }
  onBeforeUnmount(stopPolling)

  return { cycles, selectedId, selected, statistics, directory, loading, saving, busy, load, loadDirectory, loadStatistics, save, publish, remove, resend, sessionAction, callAction, select, startPolling, stopPolling }
}
