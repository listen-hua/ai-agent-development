<script setup lang="ts">
import { ref, watch } from 'vue'
import { UploadFilled } from '@element-plus/icons-vue'
import ACLEditor from './ACLEditor.vue'
import type { ACL, DirectoryOptions } from '@/types/domain'
import { isACLValid } from '@/utils/acl'
const props = defineProps<{ modelValue: boolean; uploading: boolean; options: DirectoryOptions }>(); const emit = defineEmits<{ 'update:modelValue': [value: boolean]; upload: [file: File, acl: ACL] }>(); const file = ref<File>(); const acl = ref<ACL>({ scope: 'all' })
watch(() => props.modelValue, (open) => { if (open) { file.value = undefined; acl.value = { scope: 'all' } } })
function select(raw: { raw?: File }) { file.value = raw.raw }
</script>
<template><el-dialog :model-value="modelValue" title="上传制度文件" width="min(680px, 94vw)" :close-on-click-modal="!uploading" @update:model-value="emit('update:modelValue', $event)"><el-upload drag :auto-upload="false" :limit="1" accept=".pdf,.doc,.docx,.ppt,.pptx,.xls,.xlsx,.txt,.md,.html" @change="select"><el-icon class="el-icon--upload"><UploadFilled /></el-icon><div class="el-upload__text">拖入制度文件，或 <em>点击选择</em></div><template #tip><div class="el-upload__tip">单文件不超过 32 MB；扫描件需要 OCR 服务可用。</div></template></el-upload><div class="upload-acl"><span>发布后的默认可见范围</span></div><ACLEditor v-model="acl" :options="options" /><template #footer><el-button @click="emit('update:modelValue', false)">取消</el-button><el-button type="primary" :loading="uploading" :disabled="!file || !isACLValid(acl)" @click="file && emit('upload', file, acl)">上传并解析</el-button></template></el-dialog></template>
