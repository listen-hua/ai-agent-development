<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Refresh } from '@element-plus/icons-vue'
import PageHeader from '@/components/common/PageHeader.vue'
import MetricCard from '@/components/common/MetricCard.vue'
import RoleEditorDrawer from '@/components/users/RoleEditorDrawer.vue'
import UserTable from '@/components/users/UserTable.vue'
import { useUserAdmin } from '@/composables/users/useUserAdmin'
import type { Role, User } from '@/types/domain'

const { users, loading, saving, syncing, load, updateRoles, sync } = useUserAdmin()
const drawerOpen = ref(false)
const selected = ref<User>()
const adminCount = computed(() => users.value.filter((user) => user.roles.some((role) => role !== 'employee')).length)
const unsyncedCount = computed(() => users.value.filter((user) => !user.organization_synced_at).length)

function edit(user: User) { selected.value = user; drawerOpen.value = true }
async function save(roles: Role[]) {
  if (!selected.value) return
  selected.value = await updateRoles(selected.value, roles)
  drawerOpen.value = false
}

onMounted(load)
</script>

<template>
  <section class="admin-page">
    <PageHeader eyebrow="IDENTITY & ACCESS" title="用户与权限" description="飞书组织属性决定文档范围，系统角色决定后台管理能力。">
      <el-button type="primary" :icon="Refresh" :loading="syncing" @click="sync">同步飞书通讯录</el-button>
    </PageHeader>
    <div class="metric-grid user-metrics">
      <MetricCard label="已登录用户" :value="users.length" note="用户首次登录后进入目录" />
      <MetricCard label="后台管理员" :value="adminCount" note="至少保留一名超级管理员" tone="violet" />
      <MetricCard label="待同步组织信息" :value="unsyncedCount" note="需要通讯录读取权限" tone="amber" />
      <MetricCard label="正常账号" :value="users.filter((user) => user.status === 'active').length" note="停用账号无法继续访问" tone="green" />
    </div>
    <div class="section-heading"><div><h2>公司用户</h2><p>管理员角色不会根据飞书职务自动授予，所有变更都会写入审计日志。</p></div></div>
    <UserTable :users="users" :loading="loading" @edit="edit" />
    <RoleEditorDrawer v-model="drawerOpen" :user="selected" :saving="saving" @save="save" />
  </section>
</template>
