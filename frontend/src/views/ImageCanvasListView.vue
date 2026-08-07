<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Delete, Plus } from '@element-plus/icons-vue'
import { useRouter } from 'vue-router'
import ImageCanvasListCard from '@/components/image-agent/ImageCanvasListCard.vue'
import ImageCanvasCreateDialog from '@/components/image-agent/ImageCanvasCreateDialog.vue'
import ImageCanvasTrashDrawer from '@/components/image-agent/ImageCanvasTrashDrawer.vue'
import { useImageCanvasList } from '@/composables/image-agent/useImageCanvasList'
import type { ImageCanvas } from '@/types/image-agent'

const router = useRouter()
const state = useImageCanvasList()
const createOpen = ref(false)
const trashOpen = ref(false)
const open = (canvas: ImageCanvas) => router.push(`/image-agent/canvases/${canvas.id}`)
async function create(name: string) {
  try {
    const canvas = await state.create(name)
    createOpen.value = false
    await open(canvas)
  } catch { /* 错误由画布列表状态统一提示 */ }
}
async function showTrash() { trashOpen.value = true; await state.loadTrash() }
onMounted(state.load)
</script>

<template>
  <section class="canvas-list-page">
    <header><div><small>PERSONAL IMAGE WORKSPACE</small><h1>选择无限画布</h1><p>为不同主题创建独立画布，项目只控制生图功能按键与任务归属。</p></div><div><el-button :icon="Delete" @click="showTrash">回收站</el-button><el-button type="primary" :icon="Plus" @click="createOpen=true">新建画布</el-button></div></header>
    <div v-loading="state.loading.value" class="canvas-grid"><ImageCanvasListCard v-for="canvas in state.canvases.value" :key="canvas.id" :canvas="canvas" :deleting="state.actingId.value===canvas.id" @open="open" @delete="state.remove" /><button class="new-card" @click="createOpen=true"><el-icon><Plus /></el-icon><strong>新建无限画布</strong><span>从一张空白画布开始</span></button></div>
    <el-empty v-if="!state.loading.value&&!state.canvases.value.length" description="还没有画布，创建第一张无限画布吧" />
    <ImageCanvasCreateDialog v-model="createOpen" :saving="state.creating.value" @save="create" />
    <ImageCanvasTrashDrawer v-model="trashOpen" :items="state.deletedCanvases.value" :acting-id="state.actingId.value" @restore="state.restore" />
  </section>
</template>

<style scoped>
.canvas-list-page{min-height:100%;padding:34px;background:linear-gradient(145deg,#f8f6fa,#f0ecf4)}.canvas-list-page>header{display:flex;align-items:flex-end;justify-content:space-between;gap:24px;max-width:1180px;margin:0 auto 26px}.canvas-list-page small{color:#9b74b7;font-size:10px;font-weight:750;letter-spacing:.15em}.canvas-list-page h1{margin:5px 0 7px;color:#332b3d;font-size:28px}.canvas-list-page p{margin:0;color:#8b8291;font-size:13px}.canvas-list-page header>div:last-child{display:flex;gap:9px}.canvas-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(330px,1fr));gap:16px;max-width:1180px;min-height:130px;margin:auto}.new-card{display:flex;align-items:center;justify-content:center;min-height:106px;border:1px dashed #bba7ca;border-radius:18px;color:#8056a3;background:rgba(255,255,255,.55);cursor:pointer;flex-direction:column;gap:4px}.new-card .el-icon{font-size:24px}.new-card span{color:#a59aaa;font-size:11px}.new-card:hover{border-color:#8c62ad;background:#fff}@media(max-width:650px){.canvas-list-page{padding:20px 14px}.canvas-list-page>header{align-items:flex-start;flex-direction:column}.canvas-grid{grid-template-columns:1fr}}
</style>
