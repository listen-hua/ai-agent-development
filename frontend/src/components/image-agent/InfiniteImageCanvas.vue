<script setup lang="ts">
import { nextTick, ref, shallowRef, watch } from 'vue'
import { VueFlow, useVueFlow, type Node, type NodeDragEvent, type ViewportTransform } from '@vue-flow/core'
import { Background } from '@vue-flow/background'
import { Controls } from '@vue-flow/controls'
import { MiniMap } from '@vue-flow/minimap'
import ImageCanvasNode from './ImageCanvasNode.vue'
import type { ImageCanvas, ImageCanvasNode as DomainNode, ImageViewport } from '@/types/image-agent'
import '@vue-flow/core/dist/style.css'
import '@vue-flow/core/dist/theme-default.css'
import '@vue-flow/controls/dist/style.css'
import '@vue-flow/minimap/dist/style.css'

const props = defineProps<{ canvas?: ImageCanvas; loading?: boolean; readonly?: boolean }>()
const emit = defineEmits<{
  change: [value: { viewport: ImageViewport; nodes: DomainNode[] }]
}>()
type CanvasNodeData = { assetId?: string; status: 'pending' | 'ready' | 'failed'; error?: string; width: number; height: number }
type CanvasFlowNode = Node<CanvasNodeData>
const flowNodes = shallowRef<CanvasFlowNode[]>([])
const viewport = ref<ImageViewport>({ x: 0, y: 0, zoom: 1 })
const canvasElement = ref<HTMLElement>()
const { setViewport, getViewport, setCenter } = useVueFlow({ id: 'image-agent-canvas' })

function toFlowNode(node: DomainNode): CanvasFlowNode {
  return {
    id: node.id,
    type: 'imageNode',
    position: { x: node.x, y: node.y },
    style: { width: `${node.width}px`, height: `${node.height}px`, zIndex: node.z_index },
    data: { assetId: node.asset_id, status: node.status, error: node.error, width: node.width, height: node.height },
    draggable: !props.readonly,
    selectable: true,
  }
}

function syncFromCanvas() {
  flowNodes.value = (props.canvas?.nodes || []).map(toFlowNode)
  viewport.value = props.canvas?.viewport || { x: 0, y: 0, zoom: 1 }
  nextTick(() => setViewport(viewport.value, { duration: 0 }))
}

function domainNodes(): DomainNode[] {
  const source = new Map((props.canvas?.nodes || []).map((node) => [node.id, node]))
  const result: DomainNode[] = []
  flowNodes.value.forEach((node, index) => {
    const original = source.get(node.id)
    if (!original) return
    result.push({
      ...original,
      x: node.position.x,
      y: node.position.y,
      width: node.data?.width ?? original.width,
      height: node.data?.height ?? original.height,
      z_index: index + 1,
    })
  })
  return result
}

function notifyChange(nextViewport = viewport.value) {
  viewport.value = nextViewport
  emit('change', { viewport: nextViewport, nodes: domainNodes() })
}

function onDragStop(event: NodeDragEvent) {
  const index = flowNodes.value.findIndex((node) => node.id === event.node.id)
  if (index >= 0) flowNodes.value[index] = { ...flowNodes.value[index], position: event.node.position }
  notifyChange()
}

function onViewportEnd(value: ViewportTransform) {
  notifyChange({ x: value.x, y: value.y, zoom: value.zoom })
}

async function onMiniMapClick(value: { position: { x: number; y: number } }) {
  const current = getViewport()
  await setCenter(value.position.x, value.position.y, { zoom: current.zoom, duration: 260 })
  const next = getViewport()
  notifyChange({ x: next.x, y: next.y, zoom: next.zoom })
}

function contentCenter() {
  const current = getViewport()
  const rect = canvasElement.value?.getBoundingClientRect()
  return {
    x: ((rect?.width || 900) / 2 - current.x) / current.zoom,
    y: ((rect?.height || 600) / 2 - current.y) / current.zoom,
  }
}

defineExpose({ contentCenter })
watch(() => props.canvas, syncFromCanvas, { deep: true, immediate: true })
</script>

<template>
  <section ref="canvasElement" v-loading="loading" class="infinite-canvas">
    <VueFlow
      id="image-agent-canvas"
      v-model:nodes="flowNodes"
      :min-zoom="0.1"
      :max-zoom="2"
      :nodes-draggable="!readonly"
      :nodes-connectable="false"
      :elements-selectable="true"
      :zoom-on-double-click="false"
      :default-viewport="viewport"
      @node-drag-stop="onDragStop"
      @viewport-change-end="onViewportEnd"
    >
      <Background pattern-color="#d8d4df" :gap="24" :size="1" />
      <template #node-imageNode="nodeProps"><ImageCanvasNode :data="nodeProps.data" /></template>
      <Controls position="top-left" :show-interactive="false" />
      <MiniMap
        position="bottom-right"
        pannable
        zoomable
        aria-label="画布导航地图，点击定位或拖动视野"
        :mask-color="'rgba(35, 29, 45, .14)'"
        :node-color="(node) => node.data.status === 'failed' ? '#d97979' : node.data.status === 'pending' ? '#a98bc8' : '#7353a5'"
        @click="onMiniMapClick"
      />
      <div v-if="!loading && !flowNodes.length" class="canvas-empty">
        <span>∞</span><strong>这是一张无限画布</strong>
        <p>在右侧描述画面并生成，图片会出现在当前视野中心。</p>
      </div>
      <div v-if="readonly" class="readonly-badge">项目已停用 · 历史画布只读</div>
    </VueFlow>
  </section>
</template>

<style scoped>
.infinite-canvas { min-width: 0; min-height: 0; overflow: hidden; border: 1px solid #ddd9e4; border-radius: 16px; background: #f4f2f6; box-shadow: 0 12px 38px rgba(33, 27, 46, .08); }
.infinite-canvas :deep(.vue-flow) { background: #f7f6f8; }
.infinite-canvas :deep(.vue-flow__node) { border: 0; padding: 0; background: transparent; }
.infinite-canvas :deep(.vue-flow__node.selected .canvas-image-node) { outline: 2px solid #8c65bd; outline-offset: 3px; }
.infinite-canvas :deep(.vue-flow__controls) { overflow: hidden; border: 1px solid #e1dde7; border-radius: 10px; box-shadow: 0 7px 22px rgba(31, 25, 43, .1); }
.infinite-canvas :deep(.vue-flow__controls button) { width: 32px; height: 32px; border: 0; color: #675778; background: white; display: grid; place-items: center; cursor: pointer; }
.infinite-canvas :deep(.vue-flow__minimap) { width: 150px; height: 100px; overflow: hidden; border: 1px solid rgba(99, 80, 123, .18); border-radius: 11px; background: rgba(255,255,255,.92); box-shadow: 0 8px 24px rgba(36, 29, 48, .12); cursor: crosshair; transition: border-color .18s ease, box-shadow .18s ease; }
.infinite-canvas :deep(.vue-flow__minimap:hover) { border-color: rgba(113, 79, 153, .5); box-shadow: 0 10px 28px rgba(77, 51, 106, .2); }
.infinite-canvas :deep(.vue-flow__minimap-mask) { cursor: grab; }
.infinite-canvas :deep(.vue-flow__minimap-mask:active) { cursor: grabbing; }
.canvas-empty { position: absolute; inset: 0; pointer-events: none; display: flex; align-items: center; justify-content: center; flex-direction: column; text-align: center; }
.canvas-empty span { color: #7652a3; font: 500 52px/1 Georgia, serif; }
.canvas-empty strong { margin-top: 8px; color: #4d4458; font-size: 15px; }
.canvas-empty p { margin: 7px 0; color: #96909d; font-size: 11px; }
.readonly-badge { position: absolute; top: 14px; right: 14px; padding: 7px 10px; border: 1px solid #ead7a8; border-radius: 8px; color: #8c6b23; background: rgba(255, 249, 230, .94); font-size: 10px; }
</style>
