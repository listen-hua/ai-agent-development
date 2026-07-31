<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Refresh } from '@element-plus/icons-vue'
import PageHeader from '@/components/common/PageHeader.vue'
import ImageRelayAdminPanel from '@/components/image-agent-admin/ImageRelayAdminPanel.vue'
import ImageModelAdminPanel from '@/components/image-agent-admin/ImageModelAdminPanel.vue'
import ImageProjectAdminPanel from '@/components/image-agent-admin/ImageProjectAdminPanel.vue'
import ImagePromptActionAdminPanel from '@/components/image-agent-admin/ImagePromptActionAdminPanel.vue'
import { useImageAgentAdmin } from '@/composables/image-agent/useImageAgentAdmin'

const tab = ref('relays')
const admin = useImageAgentAdmin()
onMounted(admin.load)
</script>

<template>
  <section class="admin-page image-admin-page">
    <PageHeader eyebrow="IMAGE AGENT GOVERNANCE" title="生图管理" description="管理中转站密钥、可用模型、公司项目和员工快捷提示词。">
      <el-button :icon="Refresh" :loading="admin.loading.value" @click="admin.load">刷新</el-button>
    </PageHeader>
    <el-tabs v-model="tab" class="image-admin-tabs">
      <el-tab-pane label="中转站" name="relays">
        <ImageRelayAdminPanel
          :relays="admin.relays.value" :loading="admin.loading.value" :saving="admin.saving.value" :acting="admin.acting.value"
          @save="admin.saveRelay" @test="admin.testRelay" @sync="admin.syncModels"
        />
      </el-tab-pane>
      <el-tab-pane label="模型" name="models">
        <ImageModelAdminPanel :relays="admin.relays.value" :models="admin.models.value" :loading="admin.loading.value" :saving="admin.saving.value" @save="admin.saveModel" />
      </el-tab-pane>
      <el-tab-pane label="项目" name="projects">
        <ImageProjectAdminPanel :projects="admin.projects.value" :directory="admin.directory.value" :loading="admin.loading.value" :saving="admin.saving.value" @save="admin.saveProject" />
      </el-tab-pane>
      <el-tab-pane label="功能按键" name="actions">
        <ImagePromptActionAdminPanel :actions="admin.promptActions.value" :projects="admin.projects.value" :loading="admin.loading.value" :saving="admin.saving.value" :save-action="admin.savePromptAction" @remove="admin.deletePromptAction" />
      </el-tab-pane>
    </el-tabs>
  </section>
</template>

<style scoped>
.image-admin-tabs { margin-top: 16px; }
.image-admin-tabs :deep(.el-tabs__content) { overflow: visible; }
.image-admin-page :deep(.admin-feature-panel) { padding: 18px; border: 1px solid #e3e6ed; border-radius: 14px; background: white; }
.image-admin-page :deep(.admin-feature-panel > header) { margin-bottom: 16px; display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
.image-admin-page :deep(.admin-feature-panel > header h3) { margin: 0; color: #344057; font-size: 16px; }
.image-admin-page :deep(.admin-feature-panel > header p) { margin: 5px 0 0; color: #8f97a6; font-size: 12px; }
.image-admin-page :deep(.form-grid) { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
.image-admin-page :deep(.cell-note) { margin-left: 6px; color: #8d95a3; font-size: 11px; }
.image-admin-page :deep(.el-form-item small) { display: block; margin-top: 5px; color: #969dab; font-size: 11px; }
@media (max-width: 720px) { .image-admin-page :deep(.form-grid) { grid-template-columns: 1fr; } }
</style>
