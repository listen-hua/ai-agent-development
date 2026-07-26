<script setup lang="ts">
import { ref, watch } from 'vue'
import type { Role, User } from '@/types/domain'

const props = defineProps<{ modelValue: boolean; user?: User; saving: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: boolean]; save: [roles: Role[]] }>()
const roles = ref<Role[]>([])
const adminRoles: Array<{ value: Role; label: string; description: string }> = [
  { value: 'knowledge_admin', label: '知识管理员', description: '管理制度、文档权限和 Agent 配置' },
  { value: 'notification_admin', label: '通知管理员', description: '起草、审批和发送飞书通知' },
  { value: 'image_admin', label: '生图管理员', description: '管理生图中转站、模型、项目和功能按键' },
  { value: 'auditor', label: '审计员', description: '查看审计记录与质量指标，不自动获得受限文档正文' },
  { value: 'super_admin', label: '超级管理员', description: '管理用户角色并拥有紧急文档访问权限' },
]
watch(() => [props.modelValue, props.user] as const, ([open, user]) => {
  if (open) roles.value = user?.roles.filter((role) => role !== 'employee') || []
}, { deep: true })
function submit() { emit('save', ['employee', ...roles.value]) }
</script>

<template>
  <el-drawer :model-value="modelValue" title="设置系统角色" size="min(480px, 94vw)" :close-on-click-modal="!saving" @update:model-value="emit('update:modelValue', $event)">
    <div v-if="user" class="role-editor">
      <div class="role-user-summary"><el-avatar :size="42" :src="user.avatar_url" /><div><strong>{{ user.name }}</strong><span>{{ user.job_title || '未设置职务' }}</span></div></div>
      <el-alert title="职位用于文档权限匹配；管理员权限必须在这里单独授予，不会由飞书职务自动推导。" type="info" :closable="false" show-icon />
      <el-checkbox-group v-model="roles" class="role-choice-list">
        <label v-for="role in adminRoles" :key="role.value"><el-checkbox :value="role.value"><strong>{{ role.label }}</strong></el-checkbox><span>{{ role.description }}</span></label>
      </el-checkbox-group>
    </div>
    <template #footer><el-button @click="emit('update:modelValue', false)">取消</el-button><el-button type="primary" :loading="saving" @click="submit">保存角色</el-button></template>
  </el-drawer>
</template>
