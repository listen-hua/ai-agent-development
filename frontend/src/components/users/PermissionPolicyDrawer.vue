<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { Refresh } from '@element-plus/icons-vue'
import type { PermissionKey, User } from '@/types/domain'
import type { PermissionPolicyInput } from '@/composables/users/useUserAdmin'
import { imageAgentEnabled } from '@/config/features'

type LocalState = 'inherit' | 'allow' | 'deny'

const props = defineProps<{
  modelValue: boolean
  user?: User
  saving: boolean
  refreshing: boolean
}>()
const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  save: [input: PermissionPolicyInput]
  refresh: []
}>()

const permissionRows: Array<{ key: PermissionKey; label: string; description: string }> = [
  { key: 'agent_use', label: '使用微光', description: '行政问答、提醒、会议室和个人历史' },
  { key: 'knowledge_manage', label: '制度知识库', description: '资料源、文档版本、发布与权限范围' },
  { key: 'agent_manage', label: 'Agent 配置', description: '模型、Prompt、评测、发布与回滚' },
  { key: 'image_manage', label: '生图管理', description: '中转站、模型、项目和快捷提示词' },
  { key: 'notification_manage', label: '通知中心', description: '通知草稿、审批、定时与发送' },
  { key: 'calendar_manage', label: '工作日历', description: '工作日历和会议室管理' },
  { key: 'audit_view', label: '质量与审计', description: '运行指标、问答审计和操作日志' },
  { key: 'user_manage', label: '用户与权限', description: '维护本地允许和拒绝策略' },
]
const visiblePermissionRows = computed(() => permissionRows.filter((row) => imageAgentEnabled || row.key !== 'image_manage'))

const templates = computed<Array<{ label: string; keys: PermissionKey[] }>>(() => [
  { label: '员工', keys: ['agent_use'] },
  { label: '知识管理员', keys: ['agent_use', 'knowledge_manage', 'agent_manage', 'audit_view'] },
  { label: '通知管理员', keys: ['agent_use', 'notification_manage', 'calendar_manage', 'audit_view'] },
	...(imageAgentEnabled ? [{ label: '生图管理员', keys: ['agent_use', 'image_manage'] as PermissionKey[] }] : []),
  { label: '审计员', keys: ['agent_use', 'audit_view'] },
  { label: '超级管理员', keys: permissionRows.filter((row) => imageAgentEnabled || row.key !== 'image_manage').map((row) => row.key) },
])

const states = reactive<Record<PermissionKey, LocalState>>(emptyStates())
const reason = ref('')
const policyVersion = computed(() => props.user?.local_permission_policy?.version || 0)

watch(
  () => [props.modelValue, props.user] as const,
  ([open, user]) => {
    if (!open || !user) return
    Object.assign(states, emptyStates())
    for (const key of user.permission_sources?.local_allow || []) states[key] = 'allow'
    for (const key of user.permission_sources?.local_deny || []) states[key] = 'deny'
    reason.value = ''
  },
  { deep: true },
)

function emptyStates(): Record<PermissionKey, LocalState> {
  return Object.fromEntries(permissionRows.map((row) => [row.key, 'inherit'])) as Record<PermissionKey, LocalState>
}

function applyTemplate(keys: PermissionKey[]) {
  const allowed = new Set(keys)
  for (const row of visiblePermissionRows.value) states[row.key] = allowed.has(row.key) ? 'allow' : 'inherit'
}

function submit() {
  emit('save', {
    allow_keys: permissionRows.filter((row) => states[row.key] === 'allow').map((row) => row.key),
    deny_keys: permissionRows.filter((row) => states[row.key] === 'deny').map((row) => row.key),
    version: policyVersion.value,
    reason: reason.value.trim(),
  })
}

function draftAllowed(key: PermissionKey) {
  if (!props.user || states[key] === 'deny') return false
  return states[key] === 'allow' || props.user.permission_sources.iam.includes(key)
}

function date(value?: string) {
  return value
    ? new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value))
    : '尚未同步'
}
</script>

<template>
  <el-drawer
    :model-value="modelValue"
    title="用户权限策略"
    size="min(820px, 96vw)"
    :close-on-click-modal="!saving"
    @update:model-value="emit('update:modelValue', $event)"
  >
    <div v-if="user" class="permission-editor">
      <div class="user-summary">
        <el-avatar :size="44" :src="user.avatar_url" />
        <div>
          <strong>{{ user.name }}</strong>
          <span>{{ user.job_title || '未设置职务' }} · IAM {{ user.iam_user_id || '未绑定' }} · 飞书 {{ user.feishu_user_id || user.feishu_open_id }}</span>
        </div>
        <el-button :icon="Refresh" :loading="refreshing" @click="emit('refresh')">同步 IAM</el-button>
      </div>

      <el-alert
        title="最终权限 =（IAM 授予 ∪ 本地允许）− 本地拒绝。本地拒绝优先级最高。"
        type="info"
        :closable="false"
        show-icon
      />

      <div class="sync-status">
        <span>IAM 同步：{{ date(user.permission_synced_at) }}</span>
        <el-tag v-if="user.permission_error" type="danger" effect="plain">同步降级</el-tag>
        <span v-if="user.local_permission_policy?.updated_at">
          本地修改：{{ date(user.local_permission_policy.updated_at) }}
          · {{ user.local_permission_policy.updated_by || '系统迁移' }}
        </span>
      </div>
      <p v-if="user.permission_error" class="permission-error">{{ user.permission_error }}</p>
      <p v-if="user.local_permission_policy?.reason" class="policy-reason">最近原因：{{ user.local_permission_policy.reason }}</p>

      <div class="template-row">
        <span>快捷模板</span>
        <el-button v-for="template in templates" :key="template.label" size="small" @click="applyTemplate(template.keys)">
          {{ template.label }}
        </el-button>
      </div>

      <div class="permission-matrix">
        <div class="matrix-head">
          <span>权限</span><span>IAM 状态</span><span>本地覆盖</span><span>最终结果</span>
        </div>
        <div v-for="row in visiblePermissionRows" :key="row.key" class="matrix-row">
          <div class="permission-copy"><strong>{{ row.label }}</strong><small>{{ row.description }}</small></div>
          <el-tag :type="user.permission_sources.iam.includes(row.key) ? 'success' : 'info'" effect="plain">
            {{ user.permission_sources.iam.includes(row.key) ? '已授予' : '未授予' }}
          </el-tag>
          <el-radio-group v-model="states[row.key]" size="small">
            <el-radio-button value="inherit">继承 IAM</el-radio-button>
            <el-radio-button value="allow">本地允许</el-radio-button>
            <el-radio-button value="deny">本地拒绝</el-radio-button>
          </el-radio-group>
          <el-tag :type="draftAllowed(row.key) ? 'success' : 'danger'">
            {{ draftAllowed(row.key) ? '允许' : '拒绝' }}
          </el-tag>
        </div>
      </div>

      <el-form label-position="top">
        <el-form-item label="变更原因（必填）">
          <el-input v-model="reason" type="textarea" :rows="3" maxlength="300" show-word-limit placeholder="说明授权或收回权限的原因" />
        </el-form-item>
      </el-form>
    </div>
    <template #footer>
      <el-button @click="emit('update:modelValue', false)">取消</el-button>
      <el-button type="primary" :loading="saving" :disabled="!reason.trim()" @click="submit">保存权限</el-button>
    </template>
  </el-drawer>
</template>

<style scoped>
.permission-editor { display: grid; gap: 18px; }
.user-summary { display: grid; grid-template-columns: auto 1fr auto; align-items: center; gap: 12px; }
.user-summary div { display: grid; gap: 4px; }
.user-summary span, .permission-copy small, .sync-status { color: var(--el-text-color-secondary); font-size: 12px; }
.sync-status { display: flex; align-items: center; flex-wrap: wrap; gap: 12px; }
.permission-error, .policy-reason { margin: -10px 0 0; font-size: 12px; overflow-wrap: anywhere; }
.permission-error { color: var(--el-color-danger); }
.policy-reason { color: var(--el-text-color-secondary); }
.template-row { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; }
.template-row > span { margin-right: 4px; color: var(--el-text-color-secondary); font-size: 13px; }
.permission-matrix { overflow: hidden; border: 1px solid var(--el-border-color-lighter); border-radius: 14px; }
.matrix-head, .matrix-row { display: grid; grid-template-columns: minmax(170px, 1.2fr) 90px minmax(300px, 1.5fr) 72px; align-items: center; gap: 12px; padding: 12px 14px; }
.matrix-head { background: var(--el-fill-color-light); color: var(--el-text-color-secondary); font-size: 12px; font-weight: 600; }
.matrix-row + .matrix-row { border-top: 1px solid var(--el-border-color-lighter); }
.permission-copy { display: grid; gap: 3px; }
@media (max-width: 760px) {
  .permission-matrix { overflow-x: auto; }
  .matrix-head, .matrix-row { min-width: 700px; }
}
</style>
