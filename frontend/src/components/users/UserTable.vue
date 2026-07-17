<script setup lang="ts">
import { EditPen, UserFilled } from '@element-plus/icons-vue'
import type { Role, User } from '@/types/domain'

defineProps<{ users: User[]; loading: boolean }>()
const emit = defineEmits<{ edit: [user: User] }>()
const roleLabels: Record<Role, string> = { employee: '员工', knowledge_admin: '知识管理员', notification_admin: '通知管理员', auditor: '审计员', super_admin: '超级管理员' }
function roleLabel(role: string) { return roleLabels[role as Role] || role }
function date(value?: string) { return value ? new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }).format(new Date(value)) : '尚未同步' }
</script>

<template>
  <div class="data-table-wrap">
    <el-table :data="users" v-loading="loading" row-key="id">
      <el-table-column label="员工" min-width="200">
        <template #default="{ row }"><div class="directory-user"><el-avatar :size="34" :src="row.avatar_url"><UserFilled /></el-avatar><div><strong>{{ row.name }}</strong><small>{{ row.feishu_open_id }}</small></div></div></template>
      </el-table-column>
      <el-table-column label="职务 / 部门" min-width="220">
        <template #default="{ row }"><div class="directory-org"><strong>{{ row.job_title || '未设置职务' }}</strong><small>{{ row.department_ids.length ? row.department_ids.join('、') : '未同步部门' }}</small></div></template>
      </el-table-column>
      <el-table-column label="系统角色" min-width="260">
        <template #default="{ row }"><div class="role-tags"><el-tag v-for="role in row.roles" :key="role" :type="role === 'super_admin' ? 'danger' : role === 'employee' ? 'info' : 'primary'" effect="light" round>{{ roleLabel(role) }}</el-tag></div></template>
      </el-table-column>
      <el-table-column label="组织同步" width="150"><template #default="{ row }">{{ date(row.organization_synced_at) }}</template></el-table-column>
      <el-table-column label="状态" width="90"><template #default="{ row }"><el-tag :type="row.status === 'active' ? 'success' : 'danger'" effect="plain" round>{{ row.status === 'active' ? '正常' : '停用' }}</el-tag></template></el-table-column>
      <el-table-column align="right" width="110"><template #default="{ row }"><el-button text :icon="EditPen" @click="emit('edit', row)">设置角色</el-button></template></el-table-column>
    </el-table>
  </div>
</template>
