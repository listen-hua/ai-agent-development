<script setup lang="ts">
import { computed, ref } from 'vue'
import { Delete, MagicStick, PictureRounded, Promotion } from '@element-plus/icons-vue'
import CartoonStrengthControl from './CartoonStrengthControl.vue'
import ReferenceImageSlots from './ReferenceImageSlots.vue'
import { cartoonStrength, replaceCartoonStrength } from '@/utils/imagePrompt'
import type {
  ImageAsset, ImageCount, ImageFormState, ImageModel, ImageProject, ImagePromptAction,
  ImageRatio, ImageRelay, ImageSize,
} from '@/types/image-agent'

const props = defineProps<{
  modelValue: ImageFormState
  relays: ImageRelay[]
  models: ImageModel[]
  projects: ImageProject[]
  actions: ImagePromptAction[]
  references: ImageAsset[]
  busy?: boolean
  locked?: boolean
  uploading?: boolean
}>()
const emit = defineEmits<{
  'update:modelValue': [value: ImageFormState]
  upload: [files: File[]]
  'remove-reference': [id: string]
  'canvas-reference': [assetId: string, index?: number]
  submit: []
  action: [action: ImagePromptAction]
}>()

const module = ref<'text' | 'workflow'>('text')
const selectedModel = computed(() => props.models.find((item) => item.id === props.modelValue.modelId))
const relayModels = computed(() => props.models.filter((item) => item.relay_id === props.modelValue.relayId))
const strength = computed(() => cartoonStrength(props.modelValue.prompt))
const sizes: Array<{ value: ImageSize; label: string; detail: string }> = [
  { value: '1K', label: '标准', detail: '1024 级' },
  { value: '2K', label: '高清', detail: '2048 级' },
  { value: '4K', label: '超清', detail: '4096 级' },
]
const ratios: ImageRatio[] = ['1:1', '16:9', '9:16', '4:3', '3:4']
const counts: ImageCount[] = [1, 2, 4, 8]

function update(patch: Partial<ImageFormState>) {
  const next = { ...props.modelValue, ...patch }
  if (patch.relayId !== undefined && !props.models.some((item) => item.relay_id === patch.relayId && item.id === next.modelId)) {
    next.modelId = props.models.find((item) => item.relay_id === patch.relayId)?.id || ''
  }
  emit('update:modelValue', next)
}

function updateStrength(value: number) {
  update({ prompt: replaceCartoonStrength(props.modelValue.prompt, value) })
}
</script>

<template>
  <aside class="operation-panel">
    <div class="module-tabs">
      <button :class="{ active: module === 'text' }" @click="module = 'text'"><el-icon><PictureRounded /></el-icon>文生图</button>
      <button :class="{ active: module === 'workflow' }" @click="module = 'workflow'"><el-icon><Promotion /></el-icon>工作流<small>COMING</small></button>
    </div>

    <div v-if="module === 'workflow'" class="workflow-placeholder">
      <span><el-icon><Promotion /></el-icon></span>
      <strong>工作流即将开放</strong>
      <p>后续可在画布上编排生图、重绘、放大和批处理节点。首期仅开放文生图。</p>
    </div>

    <div v-else class="text-to-image-form" :class="{ locked }" :aria-busy="locked">
      <section class="form-section description-section">
        <header><strong>文本描述</strong><div>
          <el-select :model-value="modelValue.relayId" placeholder="选择中转站" @update:model-value="update({ relayId: String($event) })">
            <el-option v-for="relay in relays" :key="relay.id" :label="relay.name" :value="relay.id" />
          </el-select>
          <el-select :model-value="modelValue.modelId" placeholder="选择模型" @update:model-value="update({ modelId: String($event) })">
            <el-option v-for="model in relayModels" :key="model.id" :label="model.display_name" :value="model.id" />
          </el-select>
        </div></header>
        <div class="prompt-editor">
          <el-input
            :model-value="modelValue.prompt"
            type="textarea"
            :rows="7"
            maxlength="4000"
            show-word-limit
            resize="none"
            placeholder="描述主体、场景、构图、光线、材质、色彩和风格……"
            @update:model-value="update({ prompt: String($event) })"
            @keydown.ctrl.enter.prevent="emit('submit')"
          />
          <footer>
            <label><el-switch :model-value="modelValue.reversePrompt" :disabled="!selectedModel?.supports_reverse" @update:model-value="update({ reversePrompt: Boolean($event) })" />图片反推</label>
            <el-button text :icon="Delete" @click="update({ prompt: '' })">一键清除</el-button>
          </footer>
        </div>
      </section>

      <ReferenceImageSlots
        :assets="references"
        :disabled="uploading || locked"
        @upload="emit('upload', $event)"
        @remove="emit('remove-reference', $event)"
        @canvas-drop="(assetId, index) => emit('canvas-reference', assetId, index)"
      />

      <section class="form-section">
        <header><strong>画面比例</strong></header>
        <div class="choice-grid ratio-grid">
          <button v-for="ratio in ratios" :key="ratio" :class="{ active: modelValue.aspectRatio === ratio }" @click="update({ aspectRatio: ratio })">{{ ratio }}</button>
        </div>
      </section>

      <section class="form-section">
        <header><strong>分辨率</strong><small>实际像素由模型结合比例决定</small></header>
        <div class="choice-grid size-grid">
          <button
            v-for="size in sizes"
            :key="size.value"
            :disabled="!selectedModel?.supported_sizes.includes(size.value)"
            :class="{ active: modelValue.imageSize === size.value }"
            @click="update({ imageSize: size.value })"
          ><strong>{{ size.label }}</strong><span>{{ size.detail }}</span></button>
        </div>
      </section>

      <section class="form-section">
        <header><strong>生成数量</strong><small>每张图片为独立子任务</small></header>
        <div class="choice-grid count-grid">
          <button
            v-for="count in counts"
            :key="count"
            :disabled="count > (selectedModel?.max_count || 1)"
            :class="{ active: modelValue.count === count }"
            @click="update({ count })"
          >{{ count }} 张</button>
        </div>
      </section>

      <section class="form-section">
        <header><strong>项目</strong></header>
        <el-select :model-value="modelValue.projectId" style="width:100%" @update:model-value="update({ projectId: String($event) })">
          <el-option v-for="project in projects" :key="project.id" :label="project.name" :value="project.id">
            <span>{{ project.name }}</span><small class="project-status">{{ project.enabled ? '可生成' : '只读' }}</small>
          </el-option>
        </el-select>
      </section>

      <section class="form-section action-section">
        <header><strong>功能按键</strong><small>点击后覆盖文本描述</small></header>
        <div v-if="actions.length" class="prompt-actions">
          <el-button v-for="action in actions" :key="action.id" plain round @click="emit('action', action)">{{ action.name }}</el-button>
        </div>
        <el-empty v-else :image-size="42" description="管理员尚未配置快捷提示词" />
      </section>

      <CartoonStrengthControl v-if="strength !== null" :model-value="strength" @update:model-value="updateStrength" />

      <el-button
        class="generate-button"
        type="primary"
        size="large"
        :icon="MagicStick"
        :loading="busy"
        :disabled="!modelValue.modelId || !modelValue.projectId || (!modelValue.reversePrompt && !modelValue.prompt.trim()) || (modelValue.reversePrompt && !references.length)"
        @click="emit('submit')"
      >{{ modelValue.reversePrompt ? '反推描述' : '生成图片' }}</el-button>
    </div>
  </aside>
</template>

<style scoped>
.operation-panel { min-height: 0; overflow-y: auto; border: 1px solid #e2dee8; border-radius: 16px; background: white; box-shadow: 0 12px 36px rgba(33, 26, 46, .07); scrollbar-width: thin; scrollbar-color: #c7b8d9 transparent; }
.module-tabs { position: sticky; top: 0; z-index: 4; padding: 10px; border-bottom: 1px solid #eeeaf1; background: rgba(255,255,255,.96); backdrop-filter: blur(12px); display: grid; grid-template-columns: 1fr 1fr; gap: 7px; }
.module-tabs button { position: relative; height: 42px; border: 0; border-radius: 10px; color: #878090; background: transparent; display: flex; align-items: center; justify-content: center; gap: 7px; font-size: 14px; cursor: pointer; }
.module-tabs button.active { color: #6d479a; background: #f2ebf9; font-weight: 650; }.module-tabs small { position: absolute; top: 3px; right: 5px; color: #b0a9b8; font-size: 10px; }
.text-to-image-form { padding: 15px; display: grid; gap: 17px; }
.text-to-image-form.locked { pointer-events: none; opacity: .72; transition: opacity .18s ease; }
.form-section { display: grid; gap: 8px; }
.form-section > header { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.form-section > header > strong { color: #3d4657; font-size: 14px; }.form-section > header > small { color: #9ba2ae; font-size: 11px; }
.description-section > header > div { width: 67%; display: grid; grid-template-columns: .8fr 1.2fr; gap: 5px; }
.description-section :deep(.el-select__wrapper), .form-section > :deep(.el-select .el-select__wrapper) { min-height: 30px; font-size: 12px; }
.prompt-editor { overflow: hidden; border: 1px solid #dfd9e7; border-radius: 12px; }
.prompt-editor :deep(.el-textarea__inner) { border: 0; border-radius: 0; box-shadow: none; padding: 11px; font-size: 13px; line-height: 1.6; }
.prompt-editor footer { height: 37px; padding: 0 7px 0 11px; border-top: 1px solid #eeeaf2; display: flex; align-items: center; justify-content: space-between; }
.prompt-editor footer label { color: #6f6877; display: flex; align-items: center; gap: 7px; font-size: 12px; }
.prompt-editor footer :deep(.el-switch) { --el-switch-on-color: #8056ad; transform: scale(.8); transform-origin: left center; margin-right: -7px; }
.prompt-editor footer .el-button { color: #9b6f7c; font-size: 12px; }
.choice-grid { display: grid; gap: 6px; }.ratio-grid { grid-template-columns: repeat(5, 1fr); }.size-grid { grid-template-columns: repeat(3, 1fr); }.count-grid { grid-template-columns: repeat(4, 1fr); }
.choice-grid button { min-width: 0; height: 34px; border: 1px solid #e3dee8; border-radius: 9px; color: #777080; background: white; cursor: pointer; font-size: 12px; }
.choice-grid button.active { border-color: #8b64b7; color: #72459f; background: #f7f1fc; box-shadow: inset 0 0 0 1px #8b64b7; }
.choice-grid button:disabled { opacity: .36; cursor: not-allowed; }.size-grid button { height: 48px; display: flex; justify-content: center; flex-direction: column; gap: 2px; }
.size-grid button strong { font-size: 12px; }.size-grid button span { color: #a3a0a8; font-size: 10px; }
.project-status { float: right; color: #9a92a4; font-size: 11px; }
.prompt-actions { display: flex; flex-wrap: wrap; gap: 6px; }.prompt-actions .el-button { margin: 0; font-size: 11px; }
.action-section :deep(.el-empty) { padding: 4px 0; }.action-section :deep(.el-empty__description) { margin-top: 3px; }.action-section :deep(.el-empty__description p) { font-size: 11px; }
.generate-button { width: 100%; margin-top: 2px; border: 0; border-radius: 11px; background: linear-gradient(135deg, #8253ad, #6540a2); box-shadow: 0 8px 18px rgba(105, 65, 157, .22); }
.workflow-placeholder { min-height: 420px; padding: 50px 30px; display: flex; align-items: center; justify-content: center; flex-direction: column; text-align: center; }
.workflow-placeholder > span { width: 58px; height: 58px; border-radius: 18px; color: #7951a5; background: #f2ecf8; display: grid; place-items: center; font-size: 26px; }
.workflow-placeholder strong { margin-top: 17px; color: #4c4356; font-size: 16px; }.workflow-placeholder p { max-width: 260px; color: #96909d; font-size: 12px; line-height: 1.7; }
@media (max-width: 1120px) { .operation-panel { max-height: 540px; }.description-section > header { align-items: stretch; flex-direction: column; }.description-section > header > div { width: 100%; } }
</style>
