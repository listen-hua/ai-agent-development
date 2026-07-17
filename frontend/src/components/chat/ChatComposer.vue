<script setup lang="ts">
import { nextTick, ref } from 'vue'
import { Promotion, VideoPause } from '@element-plus/icons-vue'
const props = defineProps<{ disabled: boolean }>(); const emit = defineEmits<{ send: [content: string]; stop: [] }>(); const content = ref(''); const textarea = ref<HTMLTextAreaElement>()
function submit() { const value = content.value.trim(); if (!value || props.disabled) return; emit('send', value); content.value = ''; void nextTick(() => resize()) }
function resize() { if (!textarea.value) return; textarea.value.style.height = 'auto'; textarea.value.style.height = `${Math.min(textarea.value.scrollHeight, 144)}px` }
function keydown(event: KeyboardEvent) { if (event.key === 'Enter' && !event.shiftKey) { event.preventDefault(); submit() } }
</script>
<template>
  <div class="composer-wrap"><div class="composer"><textarea ref="textarea" v-model="content" rows="1" maxlength="2000" placeholder="询问休假、报销、办公规范等制度问题…" :disabled="disabled" @input="resize" @keydown="keydown" /><el-button v-if="disabled" class="send-button stop" circle :icon="VideoPause" @click="emit('stop')" /><el-button v-else class="send-button" type="primary" circle :icon="Promotion" :disabled="!content.trim()" @click="submit" /></div><div class="composer-meta"><span>Enter 发送 · Shift + Enter 换行</span><span>AI 回答仅作制度查询辅助，请以引用原文为准</span></div></div>
</template>

