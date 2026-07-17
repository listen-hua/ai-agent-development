<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Delete, Picture, Plus, RefreshLeft, RefreshRight, Upload } from '@element-plus/icons-vue'

const props = defineProps<{ aspectRatio: string; imageUrl?: string }>()
const emit = defineEmits<{ upload: [file: File]; clear: [] }>()
const canvas = ref<HTMLCanvasElement>()
const fileInput = ref<HTMLInputElement>()
const zoom = ref(74)
const dimensions = computed(() => {
  const [width, height] = props.aspectRatio.split(':').map(Number)
  const ratio = width > 0 && height > 0 ? width / height : 1
  if (ratio >= 1) return { width: 960, height: Math.round(960 / ratio) }
  return { width: Math.round(760 * ratio), height: 760 }
})
const canvasStyle = computed(() => ({ aspectRatio: `${dimensions.value.width} / ${dimensions.value.height}`, width: `${zoom.value}%` }))

async function draw() {
  await nextTick()
  const target = canvas.value
  if (!target) return
  target.width = dimensions.value.width
  target.height = dimensions.value.height
  const context = target.getContext('2d')
  if (!context) return
  context.fillStyle = '#ffffff'
  context.fillRect(0, 0, target.width, target.height)
  if (!props.imageUrl) return
  const image = new Image()
  image.onload = () => {
    const scale = Math.min(target.width / image.width, target.height / image.height)
    const width = image.width * scale
    const height = image.height * scale
    context.drawImage(image, (target.width - width) / 2, (target.height - height) / 2, width, height)
  }
  image.src = props.imageUrl
}
function chooseImage(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (file) emit('upload', file)
  input.value = ''
}
watch(() => [props.aspectRatio, props.imageUrl], draw)
onMounted(draw)
onBeforeUnmount(() => { if (canvas.value) canvas.value.width = 0 })
</script>

<template>
  <section class="image-canvas-stage">
    <header class="canvas-toolbar">
      <div><el-button text :icon="RefreshLeft" disabled /><el-button text :icon="RefreshRight" disabled /><span class="toolbar-separator" /><el-button text :icon="Plus" disabled>添加文字</el-button></div>
      <div><el-slider v-model="zoom" :min="35" :max="95" :show-tooltip="false" /><span>{{ zoom }}%</span></div>
    </header>
    <div class="canvas-workspace">
      <div class="artboard-shell" :style="canvasStyle">
        <canvas ref="canvas" />
        <div v-if="!imageUrl" class="canvas-empty">
          <span><el-icon><Picture /></el-icon></span>
          <strong>AI 画布</strong>
          <p>输入描述生成图片，或先上传一张参考图。</p>
          <input ref="fileInput" hidden type="file" accept="image/jpeg,image/png,image/webp" @change="chooseImage">
          <el-button type="primary" plain :icon="Upload" @click="fileInput?.click()">上传参考图</el-button>
        </div>
      </div>
    </div>
    <footer class="canvas-statusbar">
      <span>{{ dimensions.width }} × {{ dimensions.height }} · {{ aspectRatio }}</span>
      <el-button v-if="imageUrl" text type="danger" :icon="Delete" @click="emit('clear')">移除参考图</el-button>
      <span v-else>基础画布模式</span>
    </footer>
  </section>
</template>

<style scoped>
.image-canvas-stage { min-width: 0; min-height: 0; overflow: hidden; border: 1px solid #dfe3ea; border-radius: 14px; background: #eceef2; display: grid; grid-template-rows: 48px minmax(0, 1fr) 38px; box-shadow: 0 10px 34px rgba(28, 40, 65, .07); }
.canvas-toolbar { padding: 0 12px; border-bottom: 1px solid #e2e5ea; background: rgba(255,255,255,.96); display: flex; align-items: center; justify-content: space-between; }
.canvas-toolbar > div { display: flex; align-items: center; gap: 3px; }
.canvas-toolbar .el-button { margin: 0; }
.toolbar-separator { width: 1px; height: 20px; margin: 0 4px; background: #e3e6eb; }
.canvas-toolbar :deep(.el-slider) { width: 110px; margin-right: 10px; }
.canvas-toolbar div:last-child span { color: #7d8799; font-size: 10px; }
.canvas-workspace { min-height: 0; overflow: auto; padding: 34px; display: grid; place-items: center; background-color: #eef0f4; background-image: linear-gradient(45deg, #e6e8ed 25%, transparent 25%), linear-gradient(-45deg, #e6e8ed 25%, transparent 25%), linear-gradient(45deg, transparent 75%, #e6e8ed 75%), linear-gradient(-45deg, transparent 75%, #e6e8ed 75%); background-size: 18px 18px; background-position: 0 0, 0 9px, 9px -9px, -9px 0; }
.artboard-shell { position: relative; max-width: 94%; max-height: 94%; background: white; box-shadow: 0 12px 40px rgba(31, 43, 67, .18); }
.artboard-shell canvas { display: block; width: 100%; height: 100%; }
.canvas-empty { position: absolute; inset: 0; padding: 24px; display: flex; align-items: center; justify-content: center; flex-direction: column; text-align: center; }
.canvas-empty > span { width: 52px; height: 52px; border-radius: 15px; color: #7c66c7; background: #f2edff; display: grid; place-items: center; font-size: 24px; }
.canvas-empty strong { margin-top: 14px; color: #374157; font-size: 15px; }
.canvas-empty p { margin: 6px 0 14px; color: #8b94a5; font-size: 11px; }
.canvas-statusbar { padding: 0 13px; border-top: 1px solid #e1e4e9; background: white; display: flex; align-items: center; justify-content: space-between; color: #9098a7; font-size: 9px; }
@media (max-width: 720px) { .canvas-workspace { padding: 18px; }.artboard-shell { width: 94% !important; }.canvas-toolbar > div:first-child .el-button:last-child { display: none; } }
</style>
