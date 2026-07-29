<script setup lang="ts">
import { computed } from 'vue'
import { Picture } from '@element-plus/icons-vue'
import { renderNotificationMarkdown } from '@/utils/notificationMarkdown'
import type { NotificationImage } from '@/types/domain'

const props = defineProps<{ title: string; content: string; images: NotificationImage[] }>()
const html = computed(() => renderNotificationMarkdown(props.content || '通知正文将在这里预览。'))
</script>

<template>
  <div class="notification-preview">
    <span>飞书卡片预览</span>
    <article>
      <header>{{ title || '通知标题' }}</header>
      <div class="preview-rich-text" v-html="html" />
      <div v-if="images.length" class="preview-images">
        <div v-for="image in images" :key="image.image_key">
          <img v-if="image.preview_url" :src="image.preview_url" :alt="image.alt">
          <span v-else><el-icon><Picture /></el-icon>{{ image.name }}</span>
        </div>
      </div>
      <footer>来自：微光 Shimmer · 企业 AI Agent 平台</footer>
    </article>
  </div>
</template>

<style scoped>
.notification-preview { margin-top: 21px; }
.notification-preview > span { color: #748095; font-size: 13px; }
.notification-preview article { margin-top: 8px; overflow: hidden; border: 1px solid #dfe4ec; border-radius: 13px; background: #fff; box-shadow: 0 8px 22px rgba(38, 53, 82, .06); }
.notification-preview header { padding: 13px 17px; color: #fff; font-size: 15px; font-weight: 700; background: linear-gradient(120deg, #4164d7, #627ce0); }
.preview-rich-text { min-height: 70px; padding: 16px 17px 8px; color: #4c596f; font-size: 14px; line-height: 1.7; }
.preview-rich-text :deep(p) { margin: 0 0 8px; }
.preview-rich-text :deep(h2), .preview-rich-text :deep(h3), .preview-rich-text :deep(h4) { margin: 5px 0 8px; color: #303d54; }
.preview-rich-text :deep(ul), .preview-rich-text :deep(ol) { margin: 5px 0 10px; padding-left: 20px; }
.preview-rich-text :deep(a) { color: #4569dc; }
.preview-rich-text :deep(code) { padding: 1px 4px; border-radius: 4px; background: #eef1f6; }
.preview-images { display: grid; gap: 8px; padding: 0 17px 12px; }
.preview-images img { display: block; width: 100%; max-height: 260px; border-radius: 8px; object-fit: contain; background: #f2f4f8; }
.preview-images span { min-height: 58px; display: flex; gap: 7px; align-items: center; justify-content: center; border-radius: 8px; background: #f2f4f8; color: #7e899c; font-size: 13px; }
.notification-preview footer { padding: 10px 17px 13px; color: #99a2b1; font-size: 12px; }
</style>
