<script setup lang="ts">
import { ref, watch } from 'vue'
import ACLEditor from './ACLEditor.vue'
import type { ACL, DirectoryOptions, KnowledgeDocument } from '@/types/domain'
import { cloneACL, isACLValid } from '@/utils/acl'

const props = defineProps<{ modelValue: boolean; document?: KnowledgeDocument; options: DirectoryOptions; saving: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: boolean]; save: [value: ACL] }>()
const acl = ref<ACL>({ scope: 'all' })

watch(() => [props.modelValue, props.document] as const, ([open, document]) => {
  if (open) acl.value = cloneACL(document?.acl)
}, { deep: true })
</script>

<template>
  <el-dialog :model-value="modelValue" :title="`设置可见范围${document ? ` · ${document.title}` : ''}`" width="min(680px, 94vw)" :close-on-click-modal="!saving" @update:model-value="emit('update:modelValue', $event)">
    <ACLEditor v-model="acl" :options="options" />
    <template #footer>
      <el-button @click="emit('update:modelValue', false)">取消</el-button>
      <el-button type="primary" :loading="saving" :disabled="!isACLValid(acl)" @click="emit('save', acl)">保存权限</el-button>
    </template>
  </el-dialog>
</template>
