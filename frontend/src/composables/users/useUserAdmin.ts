import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { userAdminService } from '@/services/admin'
import type { PermissionKey, User } from '@/types/domain'

export interface PermissionPolicyInput {
  allow_keys: PermissionKey[]
  deny_keys: PermissionKey[]
  version: number
  reason: string
}

export function useUserAdmin() {
  const users = ref<User[]>([])
  const loading = ref(false)
  const saving = ref(false)
  const syncing = ref(false)
  const refreshing = ref(false)

  function replaceUser(updated: User) {
    const index = users.value.findIndex((item) => item.id === updated.id)
    if (index >= 0) users.value[index] = updated
  }

  async function load() {
    loading.value = true
    try {
      users.value = await userAdminService.list()
    } finally {
      loading.value = false
    }
  }

  async function loadPermissions(user: User) {
    const updated = await userAdminService.permissions(user.id)
    replaceUser(updated)
    return updated
  }

  async function updatePermissions(user: User, input: PermissionPolicyInput) {
    saving.value = true
    try {
      const updated = await userAdminService.updatePermissions(user.id, input)
      replaceUser(updated)
      ElMessage.success('本地权限覆盖已更新')
      return updated
    } finally {
      saving.value = false
    }
  }

  async function refreshPermissions(user: User) {
    refreshing.value = true
    try {
      const updated = await userAdminService.refreshPermissions(user.id)
      replaceUser(updated)
      if (updated.permission_error) ElMessage.warning('IAM 暂时不可用，当前显示故障降级后的权限')
      else ElMessage.success('IAM 权限已重新同步')
      return updated
    } finally {
      refreshing.value = false
    }
  }

  async function sync() {
    syncing.value = true
    try {
      const result = await userAdminService.sync()
      ElMessage.success(`通讯录同步完成：成功 ${result.succeeded}，失败 ${result.failed}`)
      await load()
    } finally {
      syncing.value = false
    }
  }

  return {
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
  }
}
