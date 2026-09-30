<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { ElMessage } from 'element-plus'
import type { AgentKind, AgentProfile, AgentProfileInput } from '@/types/domain'
import { imageAgentEnabled } from '@/config/features'

const props = defineProps<{ modelValue: boolean; profile?: AgentProfile; saving: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: boolean]; save: [value: AgentProfileInput] }>()
const form = reactive({ agentKey: '', name: '', description: '', kind: 'chat' as AgentKind, provider: 'aliyun', model: 'qwen-plus', apiKey: '', enabled: true })
const editing = computed(() => Boolean(props.profile))
const modelOptions = computed(() => {
	if (form.provider === 'gemini') return imageAgentEnabled ? ['gemini-3.1-flash-image', 'gemini-3-pro-image', 'gemini-3.1-flash-lite-image'] : []
	if (form.provider === 'aliyun') return ['qwen-plus', 'qwen-max', 'qwen-flash']
	if (form.provider === 'openai') return imageAgentEnabled ? ['gpt-5', 'gpt-image-1'] : ['gpt-5']
  return []
})

watch(() => props.modelValue, (open) => {
  if (!open) return
  const value = props.profile
  Object.assign(form, value ? {
    agentKey: value.agent_key, name: value.name, description: value.description, kind: value.kind, provider: value.provider, model: value.model, apiKey: '', enabled: value.enabled,
  } : { agentKey: '', name: '', description: '', kind: 'chat', provider: 'aliyun', model: 'qwen-plus', apiKey: '', enabled: true })
})
watch(() => form.provider, (provider, previous) => {
  if (provider === previous || modelOptions.value.includes(form.model)) return
  form.model = modelOptions.value[0] || ''
})

function submit() {
  if (!/^[a-z][a-z0-9_]{2,49}$/.test(form.agentKey)) {
    ElMessage.warning('Agent 标识只能使用小写字母、数字和下划线，长度为 3–50 位')
    return
  }
  if (!form.name.trim() || !form.provider.trim() || !form.model.trim()) {
    ElMessage.warning('请完整填写 Agent 名称、供应商和模型')
    return
  }
  emit('save', {
    agent_key: form.agentKey,
    name: form.name.trim(),
    description: form.description.trim(),
    kind: form.kind,
    provider: form.provider.trim(),
    model: form.model.trim(),
    api_key: form.apiKey.trim() || undefined,
    enabled: form.enabled,
    settings: props.profile?.settings || {},
  })
}
</script>

<template>
  <el-drawer :model-value="modelValue" :title="editing ? `编辑 ${profile?.name}` : '新增小 Agent'" size="min(520px, 96vw)" :close-on-click-modal="!saving" @update:model-value="emit('update:modelValue', $event)">
    <el-form class="agent-profile-form" label-position="top">
      <div class="agent-form-row">
        <el-form-item label="Agent 名称" required><el-input v-model="form.name" maxlength="50" placeholder="例如：合同审查助手" /></el-form-item>
        <el-form-item label="功能形态" required><el-select v-model="form.kind"><el-option label="对话 Agent" value="chat" /><el-option v-if="imageAgentEnabled" label="画布 Agent" value="image" /></el-select></el-form-item>
      </div>
      <el-form-item label="Agent 唯一标识" required>
        <el-input v-model="form.agentKey" :disabled="editing" placeholder="例如：contract_reviewer" />
        <small>用于后端路由模型和 API Key，创建后不可修改。</small>
      </el-form-item>
      <el-form-item label="功能说明"><el-input v-model="form.description" type="textarea" :rows="3" maxlength="200" show-word-limit /></el-form-item>
      <div class="agent-form-row">
        <el-form-item label="AI 供应商" required>
          <el-select v-model="form.provider" filterable allow-create>
            <el-option label="阿里云百炼" value="aliyun" />
            <el-option label="Google Gemini" value="gemini" />
            <el-option label="OpenAI" value="openai" />
            <el-option label="自定义" value="custom" />
          </el-select>
        </el-form-item>
        <el-form-item label="对应模型" required>
          <el-select v-model="form.model" filterable allow-create default-first-option>
            <el-option v-for="model in modelOptions" :key="model" :label="model" :value="model" />
          </el-select>
        </el-form-item>
      </div>
      <el-form-item label="API Key">
        <el-input v-model="form.apiKey" type="password" show-password autocomplete="new-password" :placeholder="profile?.has_api_key ? `已配置 ${profile.api_key_hint}；留空则保持不变` : '输入该 Agent 独立使用的 API Key'" />
        <small>密钥提交后使用 AES-GCM 加密保存，页面不会返回明文。</small>
      </el-form-item>
      <el-form-item label="启用状态"><el-switch v-model="form.enabled" inline-prompt active-text="启用" inactive-text="停用" /></el-form-item>
      <el-alert type="info" :closable="false" show-icon title="一个小 Agent 只绑定一个主模型；Embedding 和 Rerank 等行政助手专属参数仍在下方高级配置中管理。" />
    </el-form>
    <template #footer><el-button @click="emit('update:modelValue', false)">取消</el-button><el-button type="primary" :loading="saving" @click="submit">保存 Agent</el-button></template>
  </el-drawer>
</template>

<style scoped>
.agent-profile-form { padding: 2px 4px 20px; }
.agent-form-row { display: grid; grid-template-columns: 1fr 1fr; gap: 13px; }
.agent-profile-form :deep(.el-select) { width: 100%; }
.agent-profile-form small { display: block; margin-top: 6px; color: #9099aa; font-size: 12px; line-height: 1.5; }
@media (max-width: 560px) { .agent-form-row { grid-template-columns: 1fr; gap: 0; } }
</style>
