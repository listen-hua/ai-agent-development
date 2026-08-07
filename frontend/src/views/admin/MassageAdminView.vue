<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { Delete, Edit, Plus } from '@element-plus/icons-vue'
import PageHeader from '@/components/common/PageHeader.vue'
import MassageCycleEditor from '@/components/massage-admin/MassageCycleEditor.vue'
import MassageOperations from '@/components/massage-admin/MassageOperations.vue'
import MassageStatisticsTable from '@/components/massage-admin/MassageStatisticsTable.vue'
import { useMassageAdmin } from '@/composables/massage/useMassageAdmin'
import { massageService } from '@/services/massage'
import type { MassageCycle } from '@/types/domain'

const admin = useMassageAdmin()
const editorOpen = ref(false)
const editing = ref<MassageCycle>()
function create() { editing.value = undefined; editorOpen.value = true }
function edit(cycle: MassageCycle) { editing.value = cycle; editorOpen.value = true }
async function save(value: Parameters<typeof admin.save>[0]) { if (await admin.save(value, editing.value)) editorOpen.value = false }
function exportCSV() { if (admin.selected.value) window.open(massageService.csvURL(admin.selected.value.id), '_blank', 'noopener') }
onMounted(async () => { await Promise.all([admin.load(), admin.loadDirectory()]); admin.startPolling() })
onUnmounted(admin.stopPolling)
</script>

<template>
  <section class="admin-page massage-admin">
    <PageHeader eyebrow="WELLNESS QUEUE" title="按摩排号与叫号" description="创建月度两场按摩服务，通过飞书报名，并在现场按排号自动补叫。"><el-button type="primary" :icon="Plus" @click="create">新建批次</el-button></PageHeader>
    <div class="layout">
      <aside><article v-for="cycle in admin.cycles.value" :key="cycle.id" :class="{ active: admin.selectedId.value === cycle.id }" @click="admin.select(cycle.id)"><div><strong>{{ cycle.title }}</strong><span>{{ cycle.service_month }} · {{ cycle.enrolled_count }}/{{ cycle.eligible_count }} 人报名</span></div><el-tag size="small">{{ cycle.status }}</el-tag></article><el-empty v-if="!admin.cycles.value.length" description="暂无按摩批次" /></aside>
      <main v-if="admin.selected.value"><div class="cycle-actions"><el-button :icon="Edit" :disabled="!['draft', 'scheduled', 'signup_open', 'in_progress'].includes(admin.selected.value.status)" @click="edit(admin.selected.value)">编辑</el-button><el-button type="danger" plain :icon="Delete" :loading="admin.busy.value" @click="admin.remove(admin.selected.value)">删除批次</el-button><el-button v-if="admin.selected.value.status === 'draft'" type="primary" @click="admin.publish(admin.selected.value.id)">发布报名</el-button></div><MassageOperations :cycle="admin.selected.value" :statistics="admin.statistics.value" :busy="admin.busy.value" @session-action="admin.sessionAction" @call-action="admin.callAction" @resend="admin.resend" @export="exportCSV" /><MassageStatisticsTable :value="admin.statistics.value" :loading="admin.loading.value" /></main>
      <el-empty v-else description="请选择或创建按摩批次" />
    </div>
    <MassageCycleEditor v-model="editorOpen" :cycle="editing" :directory="admin.directory.value" :saving="admin.saving.value" @save="save" />
  </section>
</template>

<style scoped>
.massage-admin{max-width:1480px}.layout{display:grid;grid-template-columns:300px minmax(0,1fr);gap:18px}.layout>aside{padding:10px;border:1px solid #e8e5ee;border-radius:18px;background:#fff}.layout>aside article{display:flex;align-items:center;justify-content:space-between;gap:8px;padding:13px;border-radius:12px;cursor:pointer}.layout>aside article.active{background:#f0ebfb}.layout>aside article>div{display:flex;min-width:0;flex-direction:column}.layout>aside strong{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:#332b42}.layout>aside span{margin-top:4px;color:#8d8495;font-size:12px}.layout>main{display:grid;gap:16px;min-width:0}.cycle-actions{display:flex;justify-content:flex-end}@media(max-width:900px){.layout{grid-template-columns:1fr}.layout>aside{display:flex;overflow:auto}.layout>aside article{min-width:250px}}
</style>
