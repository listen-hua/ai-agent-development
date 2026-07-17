<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Plus } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import PageHeader from '@/components/common/PageHeader.vue'
import AgentConfigForm from '@/components/agent/AgentConfigForm.vue'
import AgentProfileDrawer from '@/components/agent/AgentProfileDrawer.vue'
import AgentProfileGrid from '@/components/agent/AgentProfileGrid.vue'
import ConfigTimeline from '@/components/agent/ConfigTimeline.vue'
import { agentRegistryService, configService } from '@/services/admin'
import type { AgentConfig, AgentConfigVersion, AgentProfile, AgentProfileInput } from '@/types/domain'

const profiles = ref<AgentProfile[]>([])
const versions = ref<AgentConfigVersion[]>([])
const loadingProfiles = ref(false)
const savingProfile = ref(false)
const savingConfig = ref(false)
const drawerOpen = ref(false)
const editingProfile = ref<AgentProfile>()
const current = computed(() => versions.value.find(item => item.status === 'published')?.config || versions.value[0]?.config)

async function loadProfiles() {
  loadingProfiles.value = true
  try { profiles.value = await agentRegistryService.list() } finally { loadingProfiles.value = false }
}
async function loadVersions() { versions.value = await configService.list() }
function createProfile() { editingProfile.value = undefined; drawerOpen.value = true }
function editProfile(profile: AgentProfile) { editingProfile.value = profile; drawerOpen.value = true }
async function saveProfile(input: AgentProfileInput) {
  savingProfile.value = true
  try {
    if (editingProfile.value) await agentRegistryService.update(editingProfile.value.id, input)
    else await agentRegistryService.create(input)
    drawerOpen.value = false
    ElMessage.success('Agent 配置已保存，API Key 明文不会返回前端')
    await loadProfiles()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '保存 Agent 配置失败')
  } finally { savingProfile.value = false }
}
async function saveConfig(config: AgentConfig) {
  savingConfig.value = true
  try { await configService.create(config); ElMessage.success('已保存新的行政助手高级配置草稿'); await loadVersions() } finally { savingConfig.value = false }
}
async function publish(id: string) {
  await ElMessageBox.confirm('系统将使用固定评测集验证该配置。当前演示环境会直接发布，是否继续？', '评测并发布', { type: 'warning' })
  await configService.publish(id)
  ElMessage.success('新配置已发布，后续会话立即生效')
  await loadVersions()
}
onMounted(() => Promise.all([loadProfiles(), loadVersions()]))
</script>

<template>
  <section class="admin-page">
    <PageHeader eyebrow="MULTI-AGENT REGISTRY" title="Agent 配置" description="每个小 Agent 独立绑定一个模型和 API Key；密钥加密保存且不会在页面回显明文。">
      <el-button type="primary" :icon="Plus" @click="createProfile">新增 Agent</el-button>
    </PageHeader>
    <div class="section-heading agent-section-heading"><div><h2>小 Agent 与模型路由</h2><p>行政助手使用阿里云，AI 生图使用 Gemini；后续可以继续添加其他业务 Agent。</p></div><el-tag type="success" effect="plain" round>{{ profiles.filter(item => item.enabled).length }} 个已启用</el-tag></div>
    <AgentProfileGrid :agents="profiles" :loading="loadingProfiles" @edit="editProfile" />

    <div class="section-heading advanced-config-heading"><div><h2>行政助手高级参数</h2><p>保留原有生成、Embedding、Rerank、检索和 Prompt 版本配置。</p></div></div>
    <div v-if="current" class="config-layout">
      <AgentConfigForm :config="current" :saving="savingConfig" @save="saveConfig" />
      <ConfigTimeline :versions="versions" @publish="publish" />
    </div>
    <el-skeleton v-else :rows="8" animated />

    <AgentProfileDrawer v-model="drawerOpen" :profile="editingProfile" :saving="savingProfile" @save="saveProfile" />
  </section>
</template>

<style scoped>
.agent-section-heading { margin-top: 0; }
.advanced-config-heading { margin-top: 38px; padding-top: 28px; border-top: 1px solid #e2e6ed; }
</style>
