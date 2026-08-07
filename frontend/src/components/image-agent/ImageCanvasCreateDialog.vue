<script setup lang="ts">
import { ref, watch } from 'vue'

const props = defineProps<{ modelValue: boolean; saving?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: boolean]; save: [name: string] }>()
const name = ref('')
watch(() => props.modelValue, open => { if (open) name.value = '' })
function submit() { if (!props.saving && name.value.trim()) emit('save', name.value.trim()) }
</script>

<template>
  <el-dialog :model-value="modelValue" title="新建无限画布" width="430px" @update:model-value="emit('update:modelValue',$event)">
    <el-form label-position="top" @submit.prevent="submit"><el-form-item label="画布名称"><el-input v-model="name" maxlength="80" show-word-limit autofocus placeholder="例如：八月新品宣传图" @keyup.enter="submit" /></el-form-item></el-form>
    <template #footer><el-button @click="emit('update:modelValue',false)">取消</el-button><el-button type="primary" :loading="saving" :disabled="!name.trim()" @click="submit">创建并打开</el-button></template>
  </el-dialog>
</template>
