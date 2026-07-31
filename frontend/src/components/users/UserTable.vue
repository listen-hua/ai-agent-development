<script setup lang="ts">
import { EditPen, UserFilled, Warning } from '@element-plus/icons-vue'
import type { PermissionKey, User } from '@/types/domain'

defineProps<{ users: User[]; loading: boolean }>()
const emit = defineEmits<{ edit: [user: User] }>()

const permissionLabels: Record<PermissionKey, string> = {
  agent_use: '使用微光',
  knowledge_manage: '制度知识库',
  agent_manage: 'Agent 配置',
  image_manage: '生图管理',
  notification_manage: '通知中心',
  calendar_manage: '工作日历',
  audit_view: '质量审计',
  user_manage: '用户权限',
}

function date(value?: string) {
  return value
    ? new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }).format(new Date(value))
    : '尚未同步'
}
</script>

<template>
  <div class="data-table-wrap">
    <el-table :data="users" v-loading="loading" row-key="id">
      <el-table-column label="员工" min-width="200">
        <template #default="{ row }">
          <div class="directory-user">
            <el-avatar :size="34" :src="row.avatar_url"><UserFilled /></el-avatar>
            <div><strong>{{ row.name }}</strong><small>{{ row.feishu_open_id }}</small></div>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="职务 / 部门" min-width="200">
        <template #default="{ row }">
          <div class="directory-org">
            <strong>{{ row.job_title || '未设置职务' }}</strong>
            <small>{{ row.department_ids.length ? row.department_ids.join('、') : '未同步部门' }}</small>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="最终有效权限" min-width="320">
        <template #default="{ row }">
          <div class="role-tags">
            <el-tag v-for="permission in row.permissions.slice(0, 4)" :key="permission" :type="permission === 'user_manage' ? 'danger' : 'primary'" effect="light" round>
              {{ permissionLabels[permission as PermissionKey] }}
            </el-tag>
            <el-tag v-if="row.permissions.length > 4" type="info" effect="plain" round>+{{ row.permissions.length - 4 }}</el-tag>
            <span v-if="!row.permissions.length" class="empty-permissions">无权限</span>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="IAM 同步" width="160">
        <template #default="{ row }">
          <div class="sync-cell">
            <span>{{ date(row.permission_synced_at) }}</span>
            <el-tooltip v-if="row.permission_error" :content="row.permission_error">
              <el-icon color="var(--el-color-danger)"><Warning /></el-icon>
            </el-tooltip>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag :type="row.status === 'active' ? 'success' : 'danger'" effect="plain" round>{{ row.status === 'active' ? '正常' : '停用' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column align="right" width="120">
        <template #default="{ row }">
          <el-button text :icon="EditPen" @click="emit('edit', row)">设置权限</el-button>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<style scoped>
.sync-cell { display: flex; align-items: center; gap: 6px; color: var(--el-text-color-secondary); font-size: 12px; }
.empty-permissions { color: var(--el-color-danger); font-size: 12px; }
</style>
