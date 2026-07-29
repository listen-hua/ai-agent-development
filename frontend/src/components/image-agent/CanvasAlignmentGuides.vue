<script setup lang="ts">
import { computed } from 'vue'
import type { ImageViewport } from '@/types/image-agent'
import type { CanvasSnapGuide } from '@/utils/imageCanvasLayout'

const props = defineProps<{ guides: CanvasSnapGuide[]; viewport: ImageViewport }>()

const screenGuides = computed(() => props.guides.map((guide) => ({
  ...guide,
  x1: guide.x1 * props.viewport.zoom + props.viewport.x,
  y1: guide.y1 * props.viewport.zoom + props.viewport.y,
  x2: guide.x2 * props.viewport.zoom + props.viewport.x,
  y2: guide.y2 * props.viewport.zoom + props.viewport.y,
})))
</script>

<template>
  <svg v-if="screenGuides.length" class="alignment-guides" aria-hidden="true">
    <g v-for="(guide, index) in screenGuides" :key="index" :class="guide.kind">
      <line :x1="guide.x1" :y1="guide.y1" :x2="guide.x2" :y2="guide.y2" />
      <g v-if="guide.label" class="guide-label">
        <rect
          :x="(guide.x1 + guide.x2) / 2 - 11"
          :y="(guide.y1 + guide.y2) / 2 - 9"
          width="22"
          height="18"
          rx="5"
        />
        <text :x="(guide.x1 + guide.x2) / 2" :y="(guide.y1 + guide.y2) / 2 + 4">{{ guide.label }}</text>
      </g>
    </g>
  </svg>
</template>

<style scoped>
.alignment-guides { position: absolute; inset: 0; z-index: 45; width: 100%; height: 100%; overflow: visible; pointer-events: none; }
.alignment-guides line { stroke: #9a54d0; stroke-width: 1.25; vector-effect: non-scaling-stroke; }
.alignment-guides .alignment line { stroke-dasharray: 5 4; }
.alignment-guides .gap line { stroke-width: 1.5; }
.guide-label rect { fill: #8750b5; }
.guide-label text { fill: white; font-size: 10px; font-weight: 700; text-anchor: middle; }
</style>
