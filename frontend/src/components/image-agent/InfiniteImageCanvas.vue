<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'
import {
  VueFlow, useVueFlow, type Node, type NodeDragEvent, type NodeMouseEvent, type ViewportTransform,
} from '@vue-flow/core'
import { Background } from '@vue-flow/background'
import { Controls } from '@vue-flow/controls'
import { MiniMap } from '@vue-flow/minimap'
import CanvasAlignmentGuides from './CanvasAlignmentGuides.vue'
import ImageCanvasNode from './ImageCanvasNode.vue'
import type { ImageCanvas, ImageCanvasNode as DomainNode, ImageViewport } from '@/types/image-agent'
import {
  collectTransferFiles, hasExternalFiles, isEditableTarget, type CanvasImportOrigin,
} from '@/utils/imageCanvasImport'
import {
  calculateCanvasSnap, type CanvasLayoutNode, type CanvasSnapGuide,
} from '@/utils/imageCanvasLayout'
import '@vue-flow/core/dist/style.css'
import '@vue-flow/core/dist/theme-default.css'
import '@vue-flow/controls/dist/style.css'
import '@vue-flow/minimap/dist/style.css'

const props = defineProps<{
  canvas?: ImageCanvas
  loading?: boolean
  readonly?: boolean
  importing?: boolean
  importProgress?: string
}>()
const emit = defineEmits<{
  change: [value: { viewport: ImageViewport; nodes: DomainNode[] }]
  'import-images': [value: { files: File[]; point: { x: number; y: number }; origin: CanvasImportOrigin }]
  'import-blocked': []
  'unsupported-drop': []
}>()
type CanvasNodeData = { assetId?: string; status: 'pending' | 'ready' | 'failed'; error?: string; width: number; height: number }
type CanvasFlowNode = Node<CanvasNodeData>
const flowNodes = shallowRef<CanvasFlowNode[]>([])
const viewport = ref<ImageViewport>({ x: 0, y: 0, zoom: 1 })
const canvasElement = ref<HTMLElement>()
const dragDepth = ref(0)
const dragActive = ref(false)
const snapGuides = ref<CanvasSnapGuide[]>([])
const activeSnap = ref({ x: false, y: false })
const primarySelectedNodeID = ref('')
const {
  setViewport, getViewport, setCenter, screenToFlowCoordinate, getSelectedNodes,
} = useVueFlow({ id: 'image-agent-canvas' })

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

function syncNodesFromCanvas() {
  flowNodes.value = (props.canvas?.nodes || []).map(toFlowNode)
}

function syncViewportFromCanvas() {
  const nextViewport = props.canvas?.viewport || { x: 0, y: 0, zoom: 1 }
  viewport.value = { ...nextViewport }
  nextTick(() => setViewport(nextViewport, { duration: 0 }))
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
  if (props.importing) return
  viewport.value = nextViewport
  emit('change', { viewport: nextViewport, nodes: domainNodes() })
}

function onNodeDrag(event: NodeDragEvent) {
  const draggedIDs = new Set(event.nodes.map((node) => node.id))
  const moving = event.nodes.map((node) => layoutNode(node.id, node.position.x, node.position.y, node.data))
  const stationary = flowNodes.value
    .filter((node) => !draggedIDs.has(node.id))
    .map((node) => layoutNode(node.id, node.position.x, node.position.y, node.data))
  const bypass = 'altKey' in event.event && event.event.altKey
  const snap = bypass
    ? { dx: 0, dy: 0, guides: [], snappedX: false, snappedY: false }
    : calculateCanvasSnap(moving, stationary, getViewport().zoom, activeSnap.value)
  const positions = new Map(event.nodes.map((node) => [node.id, {
    x: node.position.x + snap.dx,
    y: node.position.y + snap.dy,
  }]))
  flowNodes.value = flowNodes.value.map((node) => {
    const position = positions.get(node.id)
    return position ? { ...node, position } : node
  })
  snapGuides.value = snap.guides
  activeSnap.value = { x: snap.snappedX, y: snap.snappedY }
}

function onDragStop(event: NodeDragEvent) {
  onNodeDrag(event)
  notifyChange()
  snapGuides.value = []
  activeSnap.value = { x: false, y: false }
}

function layoutNode(id: string, x: number, y: number, data?: Partial<CanvasNodeData>): CanvasLayoutNode {
  return { id, x, y, width: data?.width || 360, height: data?.height || 360 }
}

function onNodeClick(event: NodeMouseEvent) {
  primarySelectedNodeID.value = event.node.id
}

function onPaneClick() {
  primarySelectedNodeID.value = ''
}

function onViewportChange(value: ViewportTransform) {
  viewport.value = { x: value.x, y: value.y, zoom: value.zoom }
}

function onViewportEnd(value: ViewportTransform) {
  onViewportChange(value)
  notifyChange(viewport.value)
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

function generationContext() {
  const center = contentCenter()
  const selected = getSelectedNodes.value
  let anchorNodeId = selected.find((node) => node.id === primarySelectedNodeID.value)?.id || ''
  if (!anchorNodeId && selected.length) {
    const selectedIDs = new Set(selected.map((node) => node.id))
    anchorNodeId = [...flowNodes.value].reverse().find((node) => selectedIDs.has(node.id))?.id || ''
  }
  return { ...center, anchorNodeId }
}

function canImport() {
  return Boolean(props.canvas && !props.loading && !props.readonly && !props.importing)
}

function emitImport(files: File[], point: { x: number; y: number }, origin: CanvasImportOrigin) {
  if (!files.length) return
  if (!canImport()) {
    emit('import-blocked')
    return
  }
  emit('import-images', { files, point, origin })
}

function onPaste(event: ClipboardEvent) {
  if (isEditableTarget(event.target) || isEditableTarget(document.activeElement)) return
  const files = collectTransferFiles(event.clipboardData)
  if (!files.length) return
  event.preventDefault()
  emitImport(files, contentCenter(), 'paste')
}

function onDragEnter(event: DragEvent) {
  if (!hasExternalFiles(event.dataTransfer)) return
  event.preventDefault()
  if (!canImport()) return
  dragDepth.value++
  dragActive.value = true
}

function onDragOver(event: DragEvent) {
  if (event.dataTransfer?.types.includes('application/x-image-asset-id')) return
  const types = Array.from(event.dataTransfer?.types || [])
  if (!types.some((type) => type === 'Files' || type === 'text/uri-list' || type === 'text/html')) return
  event.preventDefault()
  if (event.dataTransfer) event.dataTransfer.dropEffect = hasExternalFiles(event.dataTransfer) && canImport() ? 'copy' : 'none'
}

function onDragLeave(event: DragEvent) {
  if (!hasExternalFiles(event.dataTransfer)) return
  dragDepth.value = Math.max(0, dragDepth.value - 1)
  if (dragDepth.value === 0) dragActive.value = false
}

function onDrop(event: DragEvent) {
  dragDepth.value = 0
  dragActive.value = false
  if (event.dataTransfer?.types.includes('application/x-image-asset-id')) return
  event.preventDefault()
  const files = collectTransferFiles(event.dataTransfer)
  if (!files.length) {
    emit('unsupported-drop')
    return
  }
  const point = screenToFlowCoordinate({ x: event.clientX, y: event.clientY })
  emitImport(files, point, 'drop')
}

defineExpose({ contentCenter, generationContext })
watch(
  [() => props.canvas?.nodes, () => props.readonly],
  syncNodesFromCanvas,
  { deep: true, immediate: true },
)
watch(
  () => props.canvas?.id,
  (canvasID, previousCanvasID) => {
    if (canvasID === previousCanvasID) return
    syncViewportFromCanvas()
  },
  { immediate: true },
)
onMounted(() => document.addEventListener('paste', onPaste))
onBeforeUnmount(() => document.removeEventListener('paste', onPaste))
</script>

<template>
  <section
    ref="canvasElement"
    v-loading="loading"
    class="infinite-canvas"
    @dragenter="onDragEnter"
    @dragover="onDragOver"
    @dragleave="onDragLeave"
    @drop="onDrop"
  >
    <VueFlow
      id="image-agent-canvas"
      v-model:nodes="flowNodes"
      :min-zoom="0.1"
      :max-zoom="2"
      :nodes-draggable="!readonly && !importing"
      :nodes-connectable="false"
      :elements-selectable="!importing"
      :pan-on-drag="!importing"
      :zoom-on-scroll="!importing"
      :zoom-on-pinch="!importing"
      :zoom-on-double-click="false"
      :default-viewport="viewport"
      @node-click="onNodeClick"
      @pane-click="onPaneClick"
      @node-drag="onNodeDrag"
      @node-drag-stop="onDragStop"
      @viewport-change="onViewportChange"
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
        <p>在右侧描述画面并生成，或按 Ctrl/Cmd + V 粘贴、从外部拖入图片。</p>
      </div>
      <div v-if="readonly" class="readonly-badge">项目已停用 · 历史画布只读</div>
    </VueFlow>
    <CanvasAlignmentGuides :guides="snapGuides" :viewport="viewport" />
    <div v-if="!readonly && !importing" class="snap-hint">智能吸附已开启 · 按住 Alt 临时关闭</div>
    <div v-if="dragActive" class="canvas-drop-overlay">
      <div><strong>松开以添加到画布</strong><span>支持 JPG、PNG、WEBP，单张不超过 10 MB</span></div>
    </div>
    <div v-if="importing" class="canvas-import-overlay">
      <div class="import-spinner" /><strong>{{ importProgress || '正在导入图片…' }}</strong>
    </div>
  </section>
</template>

<style scoped>
.infinite-canvas { position: relative; min-width: 0; min-height: 0; overflow: hidden; border: 1px solid #ddd9e4; border-radius: 16px; background: #f4f2f6; box-shadow: 0 12px 38px rgba(33, 27, 46, .08); }
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
.canvas-empty strong { margin-top: 8px; color: #4d4458; font-size: 16px; }
.canvas-empty p { margin: 7px 0; color: #96909d; font-size: 13px; }
.readonly-badge { position: absolute; top: 14px; right: 14px; padding: 7px 10px; border: 1px solid #ead7a8; border-radius: 8px; color: #8c6b23; background: rgba(255, 249, 230, .94); font-size: 12px; }
.snap-hint { position: absolute; z-index: 6; left: 55px; top: 13px; padding: 6px 9px; border: 1px solid rgba(124, 89, 158, .14); border-radius: 8px; color: #807387; background: rgba(255, 255, 255, .82); backdrop-filter: blur(8px); font-size: 10px; pointer-events: none; }
.canvas-drop-overlay, .canvas-import-overlay { position: absolute; inset: 0; z-index: 50; display: grid; place-items: center; background: rgba(244, 239, 249, .76); backdrop-filter: blur(3px); }
.canvas-drop-overlay { pointer-events: none; border: 2px dashed #8b61b7; border-radius: 15px; }
.canvas-drop-overlay div { min-width: 280px; padding: 24px 30px; border: 1px solid rgba(122, 83, 162, .2); border-radius: 16px; color: #5d3c7e; background: rgba(255, 255, 255, .92); box-shadow: 0 16px 42px rgba(74, 49, 99, .15); display: flex; align-items: center; flex-direction: column; gap: 7px; }
.canvas-drop-overlay strong { font-size: 16px; }
.canvas-drop-overlay span { color: #8f819b; font-size: 12px; }
.canvas-import-overlay { pointer-events: auto; background: rgba(248, 246, 250, .5); }
.canvas-import-overlay strong { margin-top: 58px; padding: 8px 13px; border-radius: 9px; color: #5d4674; background: rgba(255, 255, 255, .92); font-size: 12px; box-shadow: 0 8px 24px rgba(61, 44, 78, .12); }
.import-spinner { position: absolute; width: 34px; height: 34px; border: 3px solid rgba(126, 87, 166, .2); border-top-color: #8055ac; border-radius: 50%; animation: import-spin .8s linear infinite; }
@keyframes import-spin { to { transform: rotate(360deg); } }
@media (max-width: 760px) { .snap-hint { display: none; } }
</style>
