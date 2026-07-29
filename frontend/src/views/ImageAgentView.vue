<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Connection, Picture, Refresh } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import InfiniteImageCanvas from '@/components/image-agent/InfiniteImageCanvas.vue'
import ImageOperationPanel from '@/components/image-agent/ImageOperationPanel.vue'
import { useImageWorkspace } from '@/composables/image-agent/useImageWorkspace'
import type { ImagePromptAction } from '@/types/image-agent'

const {
  options, canvas, references, form, loading, uploading, importing, importProgress, generating, currentJob, selectedProject,
  initialize, upload, importToCanvas, addCanvasReference, removeReference, scheduleCanvasSave, submit, applyAction,
} = useImageWorkspace()
const canvasRef = ref<InstanceType<typeof InfiniteImageCanvas>>()
const statusText = computed(() => {
  if (importing.value) return importProgress.value || '正在导入图片'
  if (!currentJob.value) return '画布已同步'
  const labels: Record<string, string> = {
    pending: '任务排队中', running: 'AI 正在生成', retry: '任务自动重试中', partial: '部分生成成功',
    succeeded: '生成已完成', failed: '生成失败', cancelled: '任务已取消',
  }
  return labels[currentJob.value.status] || currentJob.value.status
})

function generationContext() {
  return canvasRef.value?.generationContext() || { x: 0, y: 0, anchorNodeId: '' }
}

function submitCurrent() {
  return submit(generationContext())
}

function useAction(action: ImagePromptAction) {
  return applyAction(action, generationContext())
}

function notifyImportBlocked() {
  if (importing.value) ElMessage.info('图片正在导入，请稍候')
  else if (!selectedProject.value) ElMessage.warning('请先选择一个生图项目')
  else ElMessage.warning('当前项目不可导入图片')
}

function notifyUnsupportedDrop() {
  ElMessage.warning('请拖入本地 JPG、PNG 或 WEBP 图片文件，不支持网页图片链接')
}

onMounted(initialize)
</script>

<template>
  <section class="image-agent-page">
    <header class="image-page-header">
      <div class="image-title">
        <span><el-icon><Picture /></el-icon></span>
        <div><small>IMAGE GENERATION AGENT</small><h1>AI 无限画布</h1></div>
      </div>
      <div class="image-runtime">
        <span><i :class="{ working: generating || importing }" />{{ statusText }}</span>
        <span><el-icon><Connection /></el-icon>{{ options.relays.length }} 个中转站 · {{ options.models.length }} 个可用模型</span>
        <el-button text :icon="Refresh" :loading="loading" :disabled="importing" @click="initialize">刷新</el-button>
      </div>
    </header>

    <div class="image-workspace">
      <InfiniteImageCanvas
        ref="canvasRef"
        :canvas="canvas"
        :loading="loading"
        :readonly="!selectedProject?.enabled"
        :importing="importing"
        :import-progress="importProgress"
        @change="scheduleCanvasSave"
        @import-images="({ files, point, origin }) => importToCanvas(files, point, origin)"
        @import-blocked="notifyImportBlocked"
        @unsupported-drop="notifyUnsupportedDrop"
      />
      <ImageOperationPanel
        v-model="form"
        :relays="options.relays"
        :models="options.models"
        :projects="options.projects"
        :actions="options.prompt_actions"
        :references="references"
        :busy="generating || importing"
        :locked="importing"
        :uploading="uploading"
        @upload="upload"
        @remove-reference="removeReference"
        @canvas-reference="addCanvasReference"
        @submit="submitCurrent"
        @action="useAction"
      />
    </div>
  </section>
</template>

<style scoped>
.image-agent-page { height: 100%; min-height: 0; overflow: hidden; padding: 16px 18px 18px; background: linear-gradient(145deg, #f7f6f8 0%, #f1eef4 100%); display: grid; grid-template-rows: 54px minmax(0, 1fr); gap: 12px; }
.image-page-header { min-width: 0; display: flex; align-items: center; justify-content: space-between; gap: 18px; }
.image-title { display: flex; align-items: center; gap: 10px; }
.image-title > span { width: 38px; height: 38px; border-radius: 12px; color: white; background: linear-gradient(145deg, #9564b8, #6440a3); display: grid; place-items: center; font-size: 20px; box-shadow: 0 8px 20px rgba(101, 64, 154, .24); }
.image-title div { display: flex; flex-direction: column; }.image-title small { color: #9b7caf; font-size: 10px; font-weight: 750; letter-spacing: .14em; }.image-title h1 { margin: 2px 0 0; color: #342c3d; font-size: 19px; line-height: 1.1; }
.image-runtime { min-width: 0; display: flex; align-items: center; gap: 15px; color: #88818d; font-size: 11px; }
.image-runtime > span { display: flex; align-items: center; gap: 5px; white-space: nowrap; }.image-runtime i { width: 7px; height: 7px; border-radius: 50%; background: #56a478; box-shadow: 0 0 0 3px rgba(86, 164, 120, .12); }
.image-runtime i.working { background: #9462bd; box-shadow: 0 0 0 4px rgba(148, 98, 189, .14); animation: pulse 1.1s ease-in-out infinite; }
.image-runtime .el-button { color: #756d7c; font-size: 12px; }
.image-workspace { min-width: 0; min-height: 0; overflow: hidden; display: grid; grid-template-columns: minmax(0, 1fr) 390px; gap: 12px; }
@keyframes pulse { 50% { opacity: .35; transform: scale(.8); } }
@media (max-width: 1120px) {
  .image-agent-page { height: auto; min-height: 100%; overflow: auto; }
  .image-workspace { overflow: visible; grid-template-columns: 1fr; grid-template-rows: minmax(560px, 66vh) auto; }
  .image-runtime > span:nth-child(2) { display: none; }
}
@media (max-width: 640px) {
  .image-agent-page { padding: 10px; }.image-runtime > span { display: none; }
}
</style>
