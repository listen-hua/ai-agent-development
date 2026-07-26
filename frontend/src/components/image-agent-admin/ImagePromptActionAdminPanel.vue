<script setup lang="ts">
import { reactive, ref } from 'vue'
import { Delete, Edit, Plus } from '@element-plus/icons-vue'
import { ElMessageBox } from 'element-plus'
import type { ImageProject, ImagePromptAction, ImagePromptActionInput } from '@/types/image-agent'

const props = defineProps<{ actions: ImagePromptAction[]; projects: ImageProject[]; loading: boolean; saving: boolean }>()
const emit = defineEmits<{ save: [id: string | undefined, input: ImagePromptActionInput]; remove: [id: string] }>()
const dialog = ref(false)
const editingId = ref<string>()
const form = reactive<ImagePromptActionInput>({ action_key: '', prompt_template: '', project_id: '', enabled: true, sort_order: 0 })
const projectName = (id?: string) => id ? props.projects.find((item) => item.id === id)?.name || id : '全局'

function open(value?: ImagePromptAction) {
  editingId.value = value?.id
  Object.assign(form, value ? {
    action_key: value.action_key, prompt_template: value.prompt_template,
    project_id: value.project_id || '', enabled: value.enabled, sort_order: value.sort_order,
  } : { action_key: '', prompt_template: '', project_id: '', enabled: true, sort_order: 0 })
  dialog.value = true
}
function save() { emit('save', editingId.value, { ...form, project_id: form.project_id || undefined }); dialog.value = false }
async function remove(id: string) {
  await ElMessageBox.confirm('删除后员工端将不再显示这个功能按键，是否继续？', '删除功能按键', { type: 'warning' })
  emit('remove', id)
}
</script>

<template>
  <section class="admin-feature-panel">
    <header><div><h3>功能按键</h3><p>项目级相同 action_key 会覆盖全局配置；含卡通化字段时员工需调节后手动生成。</p></div><el-button type="primary" :icon="Plus" @click="open()">新增按键</el-button></header>
    <el-table v-loading="loading" :data="actions" border>
      <el-table-column prop="action_key" label="按键标识" width="180" />
      <el-table-column label="范围" width="130"><template #default="{ row }">{{ projectName(row.project_id) }}</template></el-table-column>
      <el-table-column prop="prompt_template" label="提示词模板" min-width="280" show-overflow-tooltip />
      <el-table-column label="状态" width="80"><template #default="{ row }"><el-tag :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? '启用' : '停用' }}</el-tag></template></el-table-column>
      <el-table-column label="操作" width="150"><template #default="{ row }"><el-button text :icon="Edit" @click="open(row)">编辑</el-button><el-button text type="danger" :icon="Delete" @click="remove(row.id)">删除</el-button></template></el-table-column>
    </el-table>
    <el-dialog v-model="dialog" :title="editingId ? '编辑功能按键' : '新增功能按键'" width="min(680px, 95vw)">
      <el-form label-position="top">
        <el-form-item label="按键标识"><el-input v-model="form.action_key" placeholder="例如 cartoonize" /><small>使用小写字母、数字、下划线或短横线；员工端默认显示此标识。</small></el-form-item>
        <div class="form-grid"><el-form-item label="作用范围"><el-select v-model="form.project_id" clearable placeholder="全局"><el-option v-for="project in projects" :key="project.id" :label="project.name" :value="project.id" /></el-select></el-form-item><el-form-item label="排序"><el-input-number v-model="form.sort_order" :min="0" :max="999" /></el-form-item></div>
        <el-form-item label="提示词模板"><el-input v-model="form.prompt_template" type="textarea" :rows="10" placeholder='例如：{"cartoonization_strength": 0, "prompt": "..."}' /></el-form-item>
        <el-form-item label="启用"><el-switch v-model="form.enabled" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="dialog=false">取消</el-button><el-button type="primary" :loading="saving" @click="save">保存</el-button></template>
    </el-dialog>
  </section>
</template>
