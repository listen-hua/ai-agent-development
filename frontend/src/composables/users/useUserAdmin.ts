import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { userAdminService } from '@/services/admin'
import type { Role, User } from '@/types/domain'

export function useUserAdmin() {
  const users = ref<User[]>([])
  const loading = ref(false)
  const saving = ref(false)
  const syncing = ref(false)

  async function load() {
    loading.value = true
    try { users.value = await userAdminService.list() } finally { loading.value = false }
  }
  async function updateRoles(user: User, roles: Role[]) {
    saving.value = true
    try {
      const updated = await userAdminService.updateRoles(user.id, roles)
      const index = users.value.findIndex((item) => item.id === user.id)
      if (index >= 0) users.value[index] = updated
      ElMessage.success('管理员角色已更新')
      return updated
    } finally { saving.value = false }
  }
  async function sync() {
    syncing.value = true
    try {
      const result = await userAdminService.sync()
      ElMessage.success(`通讯录同步完成：成功 ${result.succeeded}，失败 ${result.failed}`)
      await load()
    } finally { syncing.value = false }
  }

  return { users, loading, saving, syncing, load, updateRoles, sync }
}
