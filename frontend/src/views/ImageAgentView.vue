<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { Brush, Connection, Files, Key, Picture } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import ImageCanvasStage from '@/components/image-agent/ImageCanvasStage.vue'
import ImagePromptBar from '@/components/image-agent/ImagePromptBar.vue'
import { agentRegistryService } from '@/services/admin'
import type { AgentProfile } from '@/types/domain'

const agents = ref<AgentProfile[]>([])
const loading = ref(true)
const aspectRatio = ref('1:1')
const referenceURL = ref('')
const imageAgent = computed(() => agents.value.find(item => item.agent_key === 'image_generator' && item.enabled))
const configured = computed(() => Boolean(imageAgent.value?.has_api_key))

async function load() {
  try { agents.value = await agentRegistryService.available() }
  catch (error) { ElMessage.error(error instanceof Error ? error.message : '读取 AI 生图配置失败') }
  finally { loading.value = false }
}
function uploadReference(file: File) {
  if (!file.type.startsWith('image/')) { ElMessage.warning('请选择图片文件'); return }
  if (file.size > 10 * 1024 * 1024) { ElMessage.warning('参考图不能超过 10 MB'); return }
  clearReference()
  referenceURL.value = URL.createObjectURL(file)
  ElMessage.success('参考图已放入基础画布')
}
function clearReference() {
  if (referenceURL.value) URL.revokeObjectURL(referenceURL.value)
  referenceURL.value = ''
}
function generate() {
  ElMessage.info('多 Agent 模型与密钥路由已完成；Gemini 图片生成任务将在下一阶段接入')
}
onMounted(load)
onBeforeUnmount(clearReference)
</script>

<template>
  <section class="image-agent-page">
    <aside class="image-agent-sidebar">
      <header><span><el-icon><Picture /></el-icon></span><div><strong>AI 生图</strong><small>Canvas Agent</small></div></header>
      <div class="image-tool-list"><button class="active"><el-icon><Brush /></el-icon><span>生成画布</span></button><button disabled><el-icon><Files /></el-icon><span>项目素材</span></button></div>
      <div v-loading="loading" class="image-model-card">
        <small>当前模型路由</small>
        <strong>{{ imageAgent?.name || 'AI 生图 Agent' }}</strong>
        <p><el-icon><Connection /></el-icon>{{ imageAgent ? `${imageAgent.provider} / ${imageAgent.model}` : 'Agent 未启用' }}</p>
        <p :class="{ ready: configured }"><el-icon><Key /></el-icon>{{ configured ? `密钥 ${imageAgent?.api_key_hint || '已配置'}` : 'Gemini API Key 待配置' }}</p>
        <el-tag :type="configured ? 'success' : 'warning'" size="small" round>{{ configured ? '画布已就绪' : '仅预览模式' }}</el-tag>
      </div>
      <div class="image-agent-note"><strong>基础版本</strong><p>当前先完成入口、画布和每个 Agent 独立模型/API Key 配置。异步生图、图层编辑和导出随后接入。</p></div>
    </aside>
    <main class="image-agent-workspace">
      <div class="image-agent-heading"><div><span>GEMINI CANVAS</span><h1>把想法变成画面</h1></div><el-tag effect="plain" round>Nano Banana 2</el-tag></div>
      <ImageCanvasStage :aspect-ratio="aspectRatio" :image-url="referenceURL" @upload="uploadReference" @clear="clearReference" />
      <ImagePromptBar :configured="configured" :model="imageAgent?.model" @update:aspect-ratio="aspectRatio = $event" @generate="generate" />
    </main>
  </section>
</template>

<style scoped>
.image-agent-page { height: 100%; min-height: 0; overflow: hidden; display: grid; grid-template-columns: 220px minmax(0, 1fr); background: #f5f6f9; }
.image-agent-sidebar { min-height: 0; overflow-y: auto; padding: 22px 15px; border-right: 1px solid #e2e5ec; background: #fff; }
.image-agent-sidebar > header { display: flex; align-items: center; gap: 10px; padding: 0 6px 22px; }
.image-agent-sidebar > header > span { width: 38px; height: 38px; border-radius: 11px; color: white; background: linear-gradient(145deg, #a45bc5, #6751c7); display: grid; place-items: center; font-size: 18px; }
.image-agent-sidebar header div { display: flex; flex-direction: column; }.image-agent-sidebar header strong { color: #303b52; font-size: 14px; }.image-agent-sidebar header small { color: #9a82aa; font-size: 9px; text-transform: uppercase; letter-spacing: .12em; }
.image-tool-list { display: flex; flex-direction: column; gap: 5px; }
.image-tool-list button { height: 40px; border: 0; border-radius: 9px; padding: 0 11px; background: transparent; color: #7c8698; display: flex; align-items: center; gap: 9px; cursor: pointer; font-size: 11px; text-align: left; }
.image-tool-list button.active { color: #7150b3; background: #f4effb; font-weight: 600; }.image-tool-list button:disabled { cursor: default; opacity: .48; }
.image-model-card { min-height: 160px; margin-top: 22px; border: 1px solid #e7e1ee; border-radius: 12px; padding: 14px; background: linear-gradient(145deg, #fcfaff, #fff); }
.image-model-card > small { color: #9b91a6; font-size: 9px; }.image-model-card > strong { display: block; margin: 5px 0 12px; color: #3a3443; font-size: 12px; }
.image-model-card p { overflow: hidden; margin: 7px 0; color: #8c8395; display: flex; align-items: center; gap: 5px; font-size: 9px; text-overflow: ellipsis; white-space: nowrap; }.image-model-card p.ready { color: #27815e; }.image-model-card .el-tag { margin-top: 8px; }
.image-agent-note { margin-top: 16px; border-radius: 10px; padding: 12px; background: #f3f4f7; }.image-agent-note strong { color: #647087; font-size: 10px; }.image-agent-note p { margin: 5px 0 0; color: #9199a8; font-size: 9px; line-height: 1.6; }
.image-agent-workspace { min-width: 0; min-height: 0; overflow: hidden; padding: 20px 26px 22px; display: grid; grid-template-rows: 47px minmax(0, 1fr) auto; gap: 12px; }
.image-agent-heading { display: flex; align-items: center; justify-content: space-between; }.image-agent-heading > div { display: flex; align-items: baseline; gap: 10px; }.image-agent-heading span { color: #8f65b4; font-size: 8px; font-weight: 700; letter-spacing: .14em; }.image-agent-heading h1 { margin: 0; color: #29354b; font-size: 18px; }
@media (max-width: 850px) { .image-agent-page { grid-template-columns: 1fr; }.image-agent-sidebar { display: none; }.image-agent-workspace { padding: 15px; } }
</style>
