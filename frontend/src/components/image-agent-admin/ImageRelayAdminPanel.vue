<script setup lang="ts">
import { reactive, ref } from 'vue'
import { Connection, Edit, Plus, Refresh } from '@element-plus/icons-vue'
import type { ImageRelay, ImageRelayInput } from '@/types/image-agent'

const props = defineProps<{ relays: ImageRelay[]; loading: boolean; saving: boolean; acting: string }>()
const emit = defineEmits<{
  save: [id: string | undefined, input: ImageRelayInput]
  test: [id: string]
  sync: [id: string]
}>()
const dialog = ref(false)
const editingId = ref<string>()
const form = reactive<ImageRelayInput>({
  relay_key: '', name: '', base_url: 'https://', api_key: '', enabled: true,
  timeout_seconds: 120, allowed_output_hosts: [],
})
const hosts = ref('')

function open(value?: ImageRelay) {
  editingId.value = value?.id
  Object.assign(form, value ? {
    relay_key: value.relay_key, name: value.name, base_url: value.base_url, api_key: '',
    enabled: value.enabled, timeout_seconds: value.timeout_seconds, allowed_output_hosts: value.allowed_output_hosts,
  } : {
    relay_key: '', name: '', base_url: 'https://', api_key: '', enabled: true,
    timeout_seconds: 120, allowed_output_hosts: [],
  })
  hosts.value = form.allowed_output_hosts.join(', ')
  dialog.value = true
}

function save() {
  emit('save', editingId.value, {
    ...form,
    allowed_output_hosts: hosts.value.split(',').map((item) => item.trim()).filter(Boolean),
  })
  dialog.value = false
}
</script>

<template>
  <section class="admin-feature-panel">
    <header><div><h3>中转站</h3><p>密钥仅加密保存在服务端，前端只显示尾号。</p></div><el-button type="primary" :icon="Plus" @click="open()">新增中转站</el-button></header>
    <el-table v-loading="loading" :data="relays" border>
      <el-table-column prop="name" label="名称" min-width="130" />
      <el-table-column prop="base_url" label="Base URL" min-width="220" show-overflow-tooltip />
      <el-table-column label="API Key" width="130"><template #default="{ row }"><el-tag :type="row.has_api_key ? 'success' : 'warning'">{{ row.has_api_key ? row.api_key_hint : '未配置' }}</el-tag></template></el-table-column>
      <el-table-column label="状态" width="90"><template #default="{ row }"><el-tag :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? '启用' : '停用' }}</el-tag></template></el-table-column>
      <el-table-column label="操作" width="290" fixed="right"><template #default="{ row }">
        <el-button text :icon="Edit" @click="open(row)">编辑</el-button>
        <el-button text :icon="Connection" :loading="acting === `test:${row.id}`" @click="emit('test', row.id)">测试连接</el-button>
        <el-button text :icon="Refresh" :loading="acting === `sync:${row.id}`" @click="emit('sync', row.id)">同步模型</el-button>
      </template></el-table-column>
    </el-table>

    <el-dialog v-model="dialog" :title="editingId ? '编辑中转站' : '新增中转站'" width="min(600px, 94vw)">
      <el-form label-position="top">
        <div class="form-grid"><el-form-item label="标识"><el-input v-model="form.relay_key" placeholder="例如 xgapi" /></el-form-item><el-form-item label="名称"><el-input v-model="form.name" /></el-form-item></div>
        <el-form-item label="OpenAI 兼容 Base URL"><el-input v-model="form.base_url" placeholder="https://example.com/v1" /></el-form-item>
        <el-form-item :label="editingId ? 'API Key（留空表示不替换）' : 'API Key'"><el-input v-model="form.api_key" type="password" show-password autocomplete="new-password" /></el-form-item>
        <div class="form-grid"><el-form-item label="超时（秒）"><el-input-number v-model="form.timeout_seconds" :min="10" :max="600" /></el-form-item><el-form-item label="启用"><el-switch v-model="form.enabled" /></el-form-item></div>
        <el-form-item label="允许的图片响应域名（逗号分隔）"><el-input v-model="hosts" placeholder="cdn.example.com, images.example.com" /><small>远程图片 URL 只有匹配这些域名时才会下载。</small></el-form-item>
      </el-form>
      <template #footer><el-button @click="dialog = false">取消</el-button><el-button type="primary" :loading="saving" @click="save">保存</el-button></template>
    </el-dialog>
  </section>
</template>
