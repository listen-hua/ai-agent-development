<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Refresh } from '@element-plus/icons-vue'
import PageHeader from '@/components/common/PageHeader.vue'
import MetricCard from '@/components/common/MetricCard.vue'
import PermissionPolicyDrawer from '@/components/users/PermissionPolicyDrawer.vue'
import UserTable from '@/components/users/UserTable.vue'
import { useUserAdmin, type PermissionPolicyInput } from '@/composables/users/useUserAdmin'
import type { User } from '@/types/domain'
import { useAuthStore } from '@/stores/auth'

const {
  users,
  loading,
  saving,
  syncing,
  refreshing,
  load,
  loadPermissions,
  updatePermissions,
  refreshPermissions,
  sync,
} = useUserAdmin()
const drawerOpen = ref(false)
const selected = ref<User>()
const auth = useAuthStore()
const router = useRouter()
const adminCount = computed(() => users.value.filter((user) => user.permissions.some((permission) => permission !== 'agent_use')).length)
const iamBoundCount = computed(() => users.value.filter((user) => user.iam_user_id).length)

async function edit(user: User) {
  selected.value = await loadPermissions(user)
  drawerOpen.value = true
}

async function save(input: PermissionPolicyInput) {
  if (!selected.value) return
  selected.value = await updatePermissions(selected.value, input)
  if (auth.user?.id === selected.value.id) {
    auth.acceptResolvedUser(selected.value)
    if (!auth.can('user_manage')) await router.replace('/chat')
  }
  drawerOpen.value = false
}

async function refresh() {
  if (!selected.value) return
  selected.value = await refreshPermissions(selected.value)
}

onMounted(load)
</script>

<template>
  <section class="admin-page">
    <PageHeader
      eyebrow="IDENTITY & ACCESS"
      title="用户与权限"
      description="IAM 负责组织级授权，微光后台负责项目级补充和拒绝；所有入口使用同一份最终权限。"
    >
      <el-button type="primary" :icon="Refresh" :loading="syncing" @click="sync">同步飞书通讯录</el-button>
    </PageHeader>
    <div class="metric-grid user-metrics">
      <MetricCard label="已登录用户" :value="users.length" note="用户首次登录后进入目录" />
      <MetricCard label="后台管理员" :value="adminCount" note="拥有至少一项管理权限" tone="violet" />
      <MetricCard label="已绑定 IAM" :value="iamBoundCount" note="IAM 与飞书身份已合并" tone="amber" />
      <MetricCard label="正常账号" :value="users.filter((user) => user.status === 'active').length" note="停用账号无法继续访问" tone="green" />
    </div>
    <div class="section-heading">
      <div>
        <h2>公司用户</h2>
        <p>本地允许用于临时补充权限，本地拒绝可立即收回 IAM 权限；所有变更均记录审计日志。</p>
      </div>
    </div>
    <UserTable :users="users" :loading="loading" @edit="edit" />
    <PermissionPolicyDrawer
      v-model="drawerOpen"
      :user="selected"
      :saving="saving"
      :refreshing="refreshing"
      @save="save"
      @refresh="refresh"
    />
  </section>
</template>
