<script setup lang="ts">
import { reactive, ref } from 'vue'
import { Delete, Edit, Plus } from '@element-plus/icons-vue'
import { ElMessageBox } from 'element-plus'
import PromptActionPreviewPicker from './PromptActionPreviewPicker.vue'
import { imagePromptActionPreviewURL } from '@/services/image-agent'
import type {
  ImageProject,
  ImagePromptAction,
  ImagePromptActionInput,
  ImagePromptActionPreviewChange,
  ImagePromptActionSaveResult,
} from '@/types/image-agent'

const props = defineProps<{
  actions: ImagePromptAction[]
  projects: ImageProject[]
  loading: boolean
  saving: boolean
  saveAction: (
    id: string | undefined,
    input: ImagePromptActionInput,
    preview: ImagePromptActionPreviewChange,
  ) => Promise<ImagePromptActionSaveResult>
}>()
const emit = defineEmits<{ remove: [id: string] }>()

const dialog = ref(false)
const editingId = ref<string>()
const editingAction = ref<ImagePromptAction>()
const preview = ref<ImagePromptActionPreviewChange>({ remove: false })
const form = reactive<ImagePromptActionInput>({
  action_key: '',
  prompt_template: '',
  project_id: '',
  enabled: true,
  sort_order: 0,
})

const projectName = (id?: string) => id
  ? props.projects.find((item) => item.id === id)?.name || id
  : '全局'

function open(value?: ImagePromptAction) {
  editingId.value = value?.id
  editingAction.value = value
  preview.value = { remove: false }
  Object.assign(form, value ? {
    action_key: value.action_key,
    prompt_template: value.prompt_template,
    project_id: value.project_id || '',
    enabled: value.enabled,
    sort_order: value.sort_order,
  } : {
    action_key: '',
    prompt_template: '',
    project_id: '',
    enabled: true,
    sort_order: 0,
  })
  dialog.value = true
}

async function save() {
  const result = await props.saveAction(
    editingId.value,
    { ...form, project_id: form.project_id || undefined },
    preview.value,
  )
  editingId.value = result.action.id
  editingAction.value = result.action
  if (result.preview_error) return
  dialog.value = false
}

async function remove(id: string) {
  await ElMessageBox.confirm('删除后员工端将不再显示这个功能按键，是否继续？', '删除功能按键', { type: 'warning' })
  emit('remove', id)
}
</script>

<template>
  <section class="admin-feature-panel">
    <header>
      <div>
        <h3>功能按键</h3>
        <p>项目级相同 action_key 会覆盖全局配置；含卡通化字段时员工需调节后手动生成。</p>
      </div>
      <el-button type="primary" :icon="Plus" @click="open()">新增按键</el-button>
    </header>

    <el-table v-loading="loading" :data="actions" border>
      <el-table-column label="预览" width="92">
        <template #default="{ row }">
          <img
            v-if="row.has_preview"
            class="table-preview"
            :src="imagePromptActionPreviewURL(row.id, row.updated_at)"
            :alt="`${row.name}预览图`"
            loading="lazy"
          >
          <span v-else class="no-preview">未设置</span>
        </template>
      </el-table-column>
      <el-table-column prop="action_key" label="按键标识" width="180" />
      <el-table-column label="范围" width="130"><template #default="{ row }">{{ projectName(row.project_id) }}</template></el-table-column>
      <el-table-column prop="prompt_template" label="提示词模板" min-width="280" show-overflow-tooltip />
      <el-table-column label="状态" width="80"><template #default="{ row }"><el-tag :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? '启用' : '停用' }}</el-tag></template></el-table-column>
      <el-table-column label="操作" width="150"><template #default="{ row }"><el-button text :icon="Edit" @click="open(row)">编辑</el-button><el-button text type="danger" :icon="Delete" @click="remove(row.id)">删除</el-button></template></el-table-column>
    </el-table>

    <el-dialog v-model="dialog" :title="editingId ? '编辑功能按键' : '新增功能按键'" width="min(720px, 95vw)" top="4vh" class="prompt-action-dialog" destroy-on-close>
      <el-form label-position="top">
        <el-form-item label="按键标识">
          <el-input v-model="form.action_key" placeholder="例如 cartoonize" />
          <small>使用小写字母、数字、下划线或短横线；员工端默认显示此标识。</small>
        </el-form-item>
        <div class="form-grid">
          <el-form-item label="作用范围"><el-select v-model="form.project_id" clearable placeholder="全局"><el-option v-for="project in projects" :key="project.id" :label="project.name" :value="project.id" /></el-select></el-form-item>
          <el-form-item label="排序"><el-input-number v-model="form.sort_order" :min="0" :max="999" /></el-form-item>
        </div>
        <el-form-item label="提示词模板"><el-input v-model="form.prompt_template" type="textarea" :rows="10" placeholder='例如：{"cartoonization_strength": 0, "prompt": "..."}' /></el-form-item>
        <el-form-item label="按钮预览图">
          <PromptActionPreviewPicker v-model="preview" :action="editingAction" />
        </el-form-item>
        <el-form-item label="启用"><el-switch v-model="form.enabled" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialog = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </section>
</template>

<style scoped>
.table-preview { display: block; width: 58px; height: 42px; border: 1px solid #ebe6ef; border-radius: 8px; object-fit: contain; background: #f7f5f8; }
.no-preview { color: #aaa3b0; font-size: 12px; }
:global(.prompt-action-dialog .el-dialog__body) { max-height: calc(92vh - 138px); overflow-y: auto; }
</style>
