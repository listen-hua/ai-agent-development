<script setup lang="ts">
import { computed, ref } from 'vue'
import { Search, Sunny } from '@element-plus/icons-vue'
import { filterFeishuEmojis, type FeishuEmoji } from '@/constants/feishuEmojis'

withDefaults(defineProps<{ label?: string }>(), { label: '表情' })
const emit = defineEmits<{ select: [emoji: FeishuEmoji] }>()

const visible = ref(false)
const query = ref('')
const failedCodes = ref(new Set<string>())
const items = computed(() => filterFeishuEmojis(query.value))

function selectEmoji(item: FeishuEmoji) {
  emit('select', item)
  visible.value = false
  query.value = ''
}

function imageFailed(code: string) {
  const next = new Set(failedCodes.value)
  next.add(code)
  failedCodes.value = next
}
</script>

<template>
  <el-popover
    v-model:visible="visible"
    placement="bottom-start"
    :width="360"
    trigger="click"
    popper-class="feishu-emoji-popper"
  >
    <template #reference>
      <el-button text :icon="Sunny" title="插入飞书表情">{{ label }}</el-button>
    </template>
    <div class="emoji-picker">
      <el-input v-model="query" clearable :prefix-icon="Search" placeholder="搜索表情名称或代码" />
      <div v-if="items.length" class="emoji-grid">
        <button
          v-for="item in items"
          :key="item.code"
          type="button"
          :title="`${item.name} · :${item.code}:`"
          @click="selectEmoji(item)"
        >
          <img v-if="!failedCodes.has(item.code)" :src="item.imageUrl" :alt="item.name" @error="imageFailed(item.code)">
          <span v-else class="emoji-fallback">{{ item.name.slice(0, 1) }}</span>
          <small>{{ item.name }}</small>
        </button>
      </div>
      <el-empty v-else :image-size="48" description="没有匹配的表情" />
      <p>插入后显示为飞书短代码，发送时由飞书客户端渲染。</p>
    </div>
  </el-popover>
</template>

<style scoped>
.emoji-picker { display: grid; gap: 10px; }
.emoji-grid { display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); gap: 5px; max-height: 300px; overflow-y: auto; padding: 2px; }
.emoji-grid button { min-width: 0; padding: 7px 3px 5px; border: 1px solid transparent; border-radius: 9px; color: #5b6474; background: transparent; cursor: pointer; display: grid; place-items: center; gap: 3px; }
.emoji-grid button:hover, .emoji-grid button:focus-visible { border-color: #d9cced; outline: none; background: #f4effa; }
.emoji-grid img, .emoji-fallback { width: 34px; height: 34px; object-fit: contain; }
.emoji-fallback { border-radius: 50%; color: #76569a; background: #eee6f7; display: grid; place-items: center; font-weight: 700; }
.emoji-grid small { width: 100%; overflow: hidden; font-size: 10px; text-align: center; text-overflow: ellipsis; white-space: nowrap; }
.emoji-picker p { margin: 0; color: #9299a6; font-size: 11px; line-height: 1.5; }
</style>
