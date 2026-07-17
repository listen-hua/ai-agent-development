<script setup lang="ts">
import { computed } from 'vue'
import { ChatDotRound, User } from '@element-plus/icons-vue'
import type { NotificationRecipientType, NotificationTargetOption } from '@/types/domain'

const props = defineProps<{
  recipientType: NotificationRecipientType
  recipientIds: string[]
  users: NotificationTargetOption[]
  chats: NotificationTargetOption[]
  chatError?: string
}>()
const emit = defineEmits<{
  'update:recipientType': [value: NotificationRecipientType]
  'update:recipientIds': [value: string[]]
}>()

const options = computed(() => props.recipientType === 'user' ? props.users : props.chats)
function changeType(value: NotificationRecipientType) {
  emit('update:recipientType', value)
  emit('update:recipientIds', [])
}
</script>

<template>
  <div class="audience-picker">
    <el-radio-group :model-value="recipientType" @update:model-value="changeType">
      <el-radio-button value="user"><el-icon><User /></el-icon> 指定个人</el-radio-button>
      <el-radio-button value="chat"><el-icon><ChatDotRound /></el-icon> 指定群聊</el-radio-button>
    </el-radio-group>
    <el-alert v-if="recipientType === 'chat' && chatError" class="chat-permission-alert" type="warning" :closable="false" show-icon>
      <template #title>暂时无法读取群聊列表</template>
      {{ chatError }}。请确认应用已开通群组信息权限，并已加入目标群聊。
    </el-alert>
    <el-select
      :model-value="recipientIds"
      multiple
      filterable
      collapse-tags
      collapse-tags-tooltip
      :max-collapse-tags="3"
      :placeholder="recipientType === 'user' ? '搜索并选择员工' : '搜索并选择机器人所在群聊'"
      style="width: 100%"
      @update:model-value="emit('update:recipientIds', $event)"
    >
      <el-option v-for="option in options" :key="option.id" :label="option.name" :value="option.id">
        <div class="target-option">
          <el-avatar :size="25" :src="option.avatar_url">{{ option.name.slice(0, 1) }}</el-avatar>
          <span>{{ option.name }}</span>
          <small>{{ recipientType === 'user' ? '个人' : '群聊' }}</small>
        </div>
      </el-option>
    </el-select>
    <p class="audience-tip">
      已选择 {{ recipientIds.length }} {{ recipientType === 'user' ? '人' : '个群聊' }}。通知不会混合发送到个人和群聊。
    </p>
  </div>
</template>

<style scoped>
.audience-picker { display: flex; flex-direction: column; gap: 12px; width: 100%; }
.audience-picker :deep(.el-radio-button__inner) { display: flex; gap: 6px; align-items: center; }
.chat-permission-alert { align-items: flex-start; }
.target-option { display: grid; grid-template-columns: 25px 1fr auto; gap: 9px; align-items: center; }
.target-option small { color: #9aa3b2; font-size: 11px; }
.audience-tip { margin: -4px 0 0; color: #8a94a6; font-size: 11px; line-height: 1.5; }
</style>
