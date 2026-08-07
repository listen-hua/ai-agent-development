<script setup lang="ts">
import { reactive, ref } from 'vue'
import { Edit, Plus } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import ACLEditor from '@/components/knowledge/ACLEditor.vue'
import type { DirectoryOptions } from '@/types/domain'
import type { ImageProject, ImageProjectInput } from '@/types/image-agent'

const props = defineProps<{
  projects: ImageProject[]
  directory: DirectoryOptions
  loading: boolean
  saving: boolean
  saveProject: (id: string | undefined, input: ImageProjectInput) => Promise<ImageProject>
}>()
const dialog = ref(false)
const editingId = ref<string>()
const form = reactive<ImageProjectInput>({ project_key: '', name: '', description: '', acl: { scope: 'all' }, enabled: true })

function open(value?: ImageProject) {
  editingId.value = value?.id
  Object.assign(form, value ? {
    project_key: value.project_key, name: value.name, description: value.description,
    acl: JSON.parse(JSON.stringify(value.acl)), enabled: value.enabled,
  } : { project_key: '', name: '', description: '', acl: { scope: 'all' }, enabled: true })
  dialog.value = true
}
async function save() {
  if (props.saving) return
  const projectKey = form.project_key.trim().toLowerCase()
  const name = form.name.trim()
  if (!/^[\p{L}][\p{L}\p{N}_-]{0,49}$/u.test(projectKey)) {
    ElMessage.warning('项目标识需以中文或英文字母开头，只能包含中文、字母、数字、下划线和短横线，最多 50 个字符')
    return
  }
  if (!name) {
    ElMessage.warning('请填写项目名称')
    return
  }
  try {
    await props.saveProject(editingId.value, {
      ...form,
      project_key: projectKey,
      name,
      description: form.description.trim(),
      acl: JSON.parse(JSON.stringify(form.acl)),
    })
    dialog.value = false
  } catch {
    // The composable displays the API error and the dialog stays open for retry.
  }
}
</script>

<template>
  <section class="admin-feature-panel">
    <header><div><h3>项目</h3><p>项目决定员工可访问的画布范围，停用后历史画布只读。</p></div><el-button type="primary" :icon="Plus" @click="open()">新增项目</el-button></header>
    <el-table v-loading="loading" :data="projects" border>
      <el-table-column prop="name" label="项目" min-width="150" />
      <el-table-column prop="project_key" label="标识" width="150" />
      <el-table-column prop="description" label="说明" min-width="220" show-overflow-tooltip />
      <el-table-column label="可见范围" width="130"><template #default="{ row }">{{ row.acl.scope === 'all' ? '公司全员' : '指定范围' }}</template></el-table-column>
      <el-table-column label="状态" width="90"><template #default="{ row }"><el-tag :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? '启用' : '停用' }}</el-tag></template></el-table-column>
      <el-table-column label="操作" width="90"><template #default="{ row }"><el-button text :icon="Edit" @click="open(row)">编辑</el-button></template></el-table-column>
    </el-table>
    <el-dialog v-model="dialog" :title="editingId ? '编辑生图项目' : '新增生图项目'" width="min(720px, 95vw)">
      <el-form label-position="top">
        <div class="form-grid"><el-form-item label="项目标识"><el-input v-model="form.project_key" maxlength="50" placeholder="例如 宣传图_2026" /><small>支持中文、字母、数字、下划线和短横线，需以中文或字母开头。</small></el-form-item><el-form-item label="项目名称（支持中文）"><el-input v-model="form.name" maxlength="100" placeholder="例如 公司宣传图" /></el-form-item></div>
        <el-form-item label="项目说明"><el-input v-model="form.description" type="textarea" :rows="2" /></el-form-item>
        <el-form-item label="启用"><el-switch v-model="form.enabled" /></el-form-item>
        <el-form-item label="可见范围"><ACLEditor v-model="form.acl" :options="directory" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="dialog=false">取消</el-button><el-button type="primary" :loading="saving" @click="save">保存</el-button></template>
    </el-dialog>
  </section>
</template>
