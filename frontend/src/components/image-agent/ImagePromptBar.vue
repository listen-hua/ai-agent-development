<script setup lang="ts">
import { ref } from 'vue'
import { MagicStick } from '@element-plus/icons-vue'

defineProps<{ configured: boolean; model?: string }>()
const emit = defineEmits<{ generate: [value: { prompt: string; aspectRatio: string; imageSize: string }]; 'update:aspectRatio': [value: string] }>()
const prompt = ref('')
const aspectRatio = ref('1:1')
const imageSize = ref('1K')
function submit() {
  if (!prompt.value.trim()) return
  emit('generate', { prompt: prompt.value.trim(), aspectRatio: aspectRatio.value, imageSize: imageSize.value })
}
function updateRatio(value: string) { aspectRatio.value = value; emit('update:aspectRatio', value) }
</script>

<template>
  <section class="image-prompt-bar">
    <el-input v-model="prompt" type="textarea" :rows="2" maxlength="1200" resize="none" placeholder="描述你想生成的画面，例如：极简蓝紫色科技海报，中央是一座未来城市，16:9，无文字……" @keydown.ctrl.enter.prevent="submit" />
    <div class="prompt-controls">
      <div>
        <el-select :model-value="aspectRatio" size="small" @update:model-value="updateRatio"><el-option v-for="ratio in ['1:1','16:9','9:16','4:3','3:4']" :key="ratio" :label="ratio" :value="ratio" /></el-select>
        <el-select v-model="imageSize" size="small"><el-option label="1K 预览" value="1K" /><el-option label="2K" value="2K" /><el-option label="4K" value="4K" /></el-select>
        <span>{{ model || '未配置模型' }}</span>
      </div>
      <el-button type="primary" :icon="MagicStick" :disabled="!configured || !prompt.trim()" @click="submit">{{ configured ? '开始生成' : '请先配置 Gemini Key' }}</el-button>
    </div>
  </section>
</template>

<style scoped>
.image-prompt-bar { border: 1px solid #dfe3eb; border-radius: 14px; padding: 11px 12px; background: white; box-shadow: 0 9px 28px rgba(29, 42, 70, .07); }
.image-prompt-bar :deep(.el-textarea__inner) { border: 0; box-shadow: none; padding: 5px 5px 10px; line-height: 1.6; }
.prompt-controls { padding-top: 9px; border-top: 1px solid #edf0f4; display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.prompt-controls > div { min-width: 0; display: flex; align-items: center; gap: 7px; }
.prompt-controls :deep(.el-select) { width: 96px; }
.prompt-controls span { overflow: hidden; color: #929bab; font-size: 9px; text-overflow: ellipsis; white-space: nowrap; }
@media (max-width: 620px) { .prompt-controls { align-items: stretch; flex-direction: column; }.prompt-controls .el-button { width: 100%; }.prompt-controls span { display: none; } }
</style>
