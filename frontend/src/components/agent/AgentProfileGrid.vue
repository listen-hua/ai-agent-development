<script setup lang="ts">
import { ChatDotRound, EditPen, Key, Picture } from '@element-plus/icons-vue'
import type { AgentProfile } from '@/types/domain'

defineProps<{ agents: AgentProfile[]; loading: boolean }>()
const emit = defineEmits<{ edit: [agent: AgentProfile] }>()
const providerLabels: Record<string, string> = { aliyun: '阿里云百炼', gemini: 'Google Gemini', openai: 'OpenAI', custom: '自定义接口' }
</script>

<template>
  <div v-loading="loading" class="agent-profile-grid">
    <article v-for="agent in agents" :key="agent.id" class="agent-profile-card" :class="{ disabled: !agent.enabled }">
      <header>
        <span class="agent-kind-icon" :class="agent.kind"><el-icon><Picture v-if="agent.kind === 'image'" /><ChatDotRound v-else /></el-icon></span>
        <div><strong>{{ agent.name }}</strong><small>{{ agent.agent_key }}</small></div>
        <el-tag :type="agent.enabled ? 'success' : 'info'" size="small" round>{{ agent.enabled ? '已启用' : '已停用' }}</el-tag>
      </header>
      <p>{{ agent.description || '暂未填写 Agent 说明' }}</p>
      <dl>
        <div><dt>模型供应商</dt><dd>{{ providerLabels[agent.provider] || agent.provider }}</dd></div>
        <div><dt>对应模型</dt><dd>{{ agent.model }}</dd></div>
        <div><dt>API Key</dt><dd :class="{ ready: agent.has_api_key }"><el-icon><Key /></el-icon>{{ agent.has_api_key ? agent.api_key_hint || '已配置' : '待配置' }}</dd></div>
      </dl>
      <footer>
        <span>{{ agent.credential_source === 'environment' ? '部署环境密钥' : '后台加密密钥' }}</span>
        <el-button size="small" plain :icon="EditPen" @click="emit('edit', agent)">编辑配置</el-button>
      </footer>
    </article>
    <el-empty v-if="!loading && !agents.length" description="还没有配置小 Agent" />
  </div>
</template>

<style scoped>
.agent-profile-grid { min-height: 180px; display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px; }
.agent-profile-card { min-width: 0; border: 1px solid #e3e7ef; border-radius: 15px; padding: 18px; background: #fff; box-shadow: 0 7px 24px rgba(30, 44, 72, .04); transition: transform .18s, box-shadow .18s; }
.agent-profile-card:hover { transform: translateY(-2px); box-shadow: 0 12px 30px rgba(30, 44, 72, .08); }
.agent-profile-card.disabled { opacity: .68; }
.agent-profile-card header { display: grid; grid-template-columns: 42px minmax(0, 1fr) auto; gap: 11px; align-items: center; }
.agent-kind-icon { width: 42px; height: 42px; border-radius: 12px; display: grid; place-items: center; color: #4d66d5; background: #edf1ff; font-size: 20px; }
.agent-kind-icon.image { color: #9c50bc; background: #f7edfb; }
.agent-profile-card header > div { min-width: 0; display: flex; flex-direction: column; }
.agent-profile-card strong { color: #2d3950; font-size: 15px; }
.agent-profile-card small { margin-top: 2px; color: #9aa2b1; font-size: 11px; }
.agent-profile-card > p { min-height: 38px; margin: 15px 0; color: #7b8597; font-size: 13px; line-height: 1.7; }
.agent-profile-card dl { margin: 0; border-radius: 10px; padding: 4px 12px; background: #f8f9fb; }
.agent-profile-card dl > div { min-height: 36px; display: grid; grid-template-columns: 90px minmax(0, 1fr); align-items: center; border-bottom: 1px solid #e9ecf1; }
.agent-profile-card dl > div:last-child { border-bottom: 0; }
.agent-profile-card dt { color: #939baa; font-size: 12px; }
.agent-profile-card dd { overflow: hidden; margin: 0; color: #4f5b70; font-size: 13px; text-align: right; text-overflow: ellipsis; white-space: nowrap; }
.agent-profile-card dd .el-icon { margin-right: 4px; vertical-align: -2px; }
.agent-profile-card dd.ready { color: #23865f; }
.agent-profile-card footer { min-height: 47px; padding-top: 13px; display: flex; align-items: flex-end; justify-content: space-between; gap: 8px; }
.agent-profile-card footer span { color: #9aa2b1; font-size: 11px; }
.agent-profile-grid :deep(.el-empty) { grid-column: 1 / -1; }
@media (max-width: 820px) { .agent-profile-grid { grid-template-columns: 1fr; } }
</style>
