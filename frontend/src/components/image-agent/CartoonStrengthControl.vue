<script setup lang="ts">
import { computed } from 'vue'
import { cartoonStrengthLabels } from '@/utils/imagePrompt'

const props = defineProps<{ modelValue: number }>()
const emit = defineEmits<{ 'update:modelValue': [value: number] }>()
const marks = { 0: '0', 0.25: '.25', 0.5: '.5', 0.75: '.75', 1: '1' }
const description = computed(() => cartoonStrengthLabels[props.modelValue] || '')
</script>

<template>
  <section class="strength-control">
    <header><span>卡通化程度</span><strong>{{ description }}</strong></header>
    <el-slider
      :model-value="modelValue"
      :min="0"
      :max="1"
      :step="0.25"
      :marks="marks"
      show-stops
      @update:model-value="emit('update:modelValue', Number($event))"
    />
  </section>
</template>

<style scoped>
.strength-control { padding: 12px 12px 22px; border: 1px solid #eadff8; border-radius: 12px; background: #fbf8ff; }
.strength-control header { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.strength-control span { color: #564968; font-size: 14px; font-weight: 650; }
.strength-control strong { color: #8964b5; font-size: 12px; font-weight: 550; }
.strength-control :deep(.el-slider) { margin: 13px 6px 0; width: calc(100% - 12px); }
.strength-control :deep(.el-slider__marks-text) { color: #9b91a8; font-size: 11px; }
</style>
