<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  relayName?: string
  modelName?: string
  modelKey?: string
  pixian?: boolean
}>()

const effectiveModelName = computed(() => props.modelName || props.modelKey || '')
const showModelKey = computed(() => Boolean(props.modelKey && props.modelKey !== effectiveModelName.value))
</script>

<template>
  <div class="generation-source-badge" aria-label="图片生成来源">
    <span v-if="relayName"><small>中转站</small><strong>{{ relayName }}</strong></span>
    <span v-if="effectiveModelName"><small>模型</small><strong>{{ effectiveModelName }}</strong></span>
    <span v-if="showModelKey" class="model-key"><small>模型 ID</small><code>{{ modelKey }}</code></span>
    <span v-if="pixian" class="pixian-mark"><small>处理</small><strong>Pixian 智能抠图</strong></span>
  </div>
</template>

<style scoped>
.generation-source-badge { position: absolute; z-index: 5; top: 8px; left: 8px; max-width: calc(100% - 56px); padding: 7px 9px; border: 1px solid rgba(255, 255, 255, .2); border-radius: 8px; color: white; background: rgba(31, 25, 42, .84); box-shadow: 0 4px 14px rgba(26, 20, 35, .18); display: flex; flex-direction: column; gap: 3px; line-height: 1.2; pointer-events: none; backdrop-filter: blur(6px); opacity: 0; transform: translateY(-2px); transition: opacity .18s ease, transform .18s ease; }
.generation-source-badge span { min-width: 0; display: flex; align-items: baseline; gap: 6px; }
.generation-source-badge small { flex: none; color: rgba(255, 255, 255, .7); font-size: 9px; }
.generation-source-badge strong, .generation-source-badge code { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 10px; }
.generation-source-badge code { color: #d8c6ef; font-family: ui-monospace, SFMono-Regular, Consolas, monospace; }
.pixian-mark strong { color: #d7f0ff; }
</style>
