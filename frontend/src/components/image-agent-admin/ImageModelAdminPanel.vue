<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { Edit, Search } from '@element-plus/icons-vue'
import type { ImageCount, ImageModel, ImageModelInput, ImageRelay, ImageSize } from '@/types/image-agent'
import { filterImageModels } from '@/utils/imageModelSearch'

const props = defineProps<{ relays: ImageRelay[]; models: ImageModel[]; loading: boolean; saving: boolean }>()
const emit = defineEmits<{ save: [id: string, input: ImageModelInput] }>()
const filterRelay = ref('')
const searchQuery = ref('')
const dialog = ref(false)
const editingId = ref('')
const form = reactive<ImageModelInput>({
  display_name: '', protocol: 'chat_completions', enabled: false, supports_reference: false,
  supports_reverse: true, supported_sizes: ['1K'], max_count: 1,
})
const visible = computed(() => filterImageModels(props.models, props.relays, filterRelay.value, searchQuery.value))
const relayName = (id: string) => props.relays.find((item) => item.id === id)?.name || id

function open(model: ImageModel) {
  editingId.value = model.id
  Object.assign(form, {
    display_name: model.display_name, protocol: model.protocol, enabled: model.enabled,
    supports_reference: model.supports_reference, supports_reverse: model.supports_reverse,
    supported_sizes: [...model.supported_sizes], max_count: model.max_count,
  })
  dialog.value = true
}
function save() { emit('save', editingId.value, { ...form }); dialog.value = false }
</script>

<template>
  <section class="admin-feature-panel">
    <header><div><h3>模型</h3><p>从中转站同步模型后，必须明确启用并配置生图协议与能力。</p></div>
      <div class="model-filters">
        <el-input v-model="searchQuery" clearable :prefix-icon="Search" class="model-search" placeholder="搜索模型名称或模型 ID" />
        <el-select v-model="filterRelay" clearable placeholder="全部中转站" class="relay-filter"><el-option v-for="relay in relays" :key="relay.id" :label="relay.name" :value="relay.id" /></el-select>
      </div>
    </header>
    <div v-if="searchQuery || filterRelay" class="model-result-count">找到 {{ visible.length }} 个模型，共 {{ models.length }} 个</div>
    <el-table v-loading="loading" :data="visible" empty-text="没有找到匹配的模型" border>
      <el-table-column prop="display_name" label="模型" min-width="220" show-overflow-tooltip />
      <el-table-column label="中转站" width="130"><template #default="{ row }">{{ relayName(row.relay_id) }}</template></el-table-column>
      <el-table-column label="协议" width="150"><template #default="{ row }">{{ row.protocol === 'chat_completions' ? 'Chat Completions' : 'Images Generations' }}</template></el-table-column>
      <el-table-column label="能力" min-width="190"><template #default="{ row }"><el-tag v-if="row.supports_reference" size="small">参考图</el-tag><el-tag v-if="row.supports_reverse" size="small" type="info">图片反推</el-tag><span class="cell-note">{{ row.supported_sizes.join('/') }} · 最多 {{ row.max_count }} 张</span></template></el-table-column>
      <el-table-column label="开放" width="80"><template #default="{ row }"><el-tag :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? '是' : '否' }}</el-tag></template></el-table-column>
      <el-table-column label="操作" width="90"><template #default="{ row }"><el-button text :icon="Edit" @click="open(row)">配置</el-button></template></el-table-column>
    </el-table>
    <el-dialog v-model="dialog" title="配置生图模型" width="min(600px, 94vw)">
      <el-form label-position="top">
        <el-form-item label="员工端显示名称"><el-input v-model="form.display_name" /></el-form-item>
        <el-form-item label="生图协议"><el-radio-group v-model="form.protocol"><el-radio-button value="chat_completions">Chat Completions</el-radio-button><el-radio-button value="images_generations">Images Generations</el-radio-button></el-radio-group></el-form-item>
        <div class="form-grid"><el-form-item label="开放给员工"><el-switch v-model="form.enabled" /></el-form-item><el-form-item label="最大生成数量"><el-select v-model="form.max_count"><el-option v-for="count in ([1,2,4,8] as ImageCount[])" :key="count" :label="`${count} 张`" :value="count" /></el-select></el-form-item></div>
        <el-form-item label="分辨率档位"><el-checkbox-group v-model="form.supported_sizes"><el-checkbox v-for="size in (['1K','2K','4K'] as ImageSize[])" :key="size" :value="size">{{ size }}</el-checkbox></el-checkbox-group></el-form-item>
        <div class="form-grid"><el-form-item label="支持参考图"><el-switch v-model="form.supports_reference" /></el-form-item><el-form-item label="支持图片反推"><el-switch v-model="form.supports_reverse" /></el-form-item></div>
        <el-alert v-if="form.protocol === 'images_generations' && form.supports_reference" title="首期 Images 协议不开放参考图编辑；需要参考图时请选择 Chat 协议。" type="warning" :closable="false" />
      </el-form>
      <template #footer><el-button @click="dialog=false">取消</el-button><el-button type="primary" :loading="saving" @click="save">保存</el-button></template>
    </el-dialog>
  </section>
</template>

<style scoped>
.model-filters { display: flex; align-items: center; justify-content: flex-end; gap: 10px; }
.model-search { width: 300px; }
.relay-filter { width: 180px; }
.model-result-count { margin: -4px 0 10px; color: #8d95a3; font-size: 11px; text-align: right; }
@media (max-width: 820px) {
  .model-filters { width: 100%; align-items: stretch; flex-direction: column; }
  .model-search, .relay-filter { width: 100%; }
  .model-result-count { text-align: left; }
}
</style>
