<script setup lang="ts">
import { Download, Promotion, RefreshRight, VideoPause, VideoPlay } from '@element-plus/icons-vue'
import type { MassageCycle, MassageStatistic } from '@/types/domain'

defineProps<{ cycle: MassageCycle; statistics?: MassageStatistic; busy?: boolean }>()
const emit = defineEmits<{
  sessionAction: [id: string, action: 'start' | 'pause' | 'resume' | 'close']
  callAction: [id: string, action: 'complete' | 'no-show']
  resend: []
  export: []
}>()
const date = (value: string) => new Date(value).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false })
</script>

<template>
  <section class="operations">
    <header><div><h2>实时叫号控制台</h2><p>{{ cycle.title }} · {{ cycle.service_month }}</p></div><div><el-button :icon="RefreshRight" :disabled="cycle.status === 'draft'" @click="emit('resend')">补发未响应员工</el-button><el-button :icon="Download" @click="emit('export')">导出统计</el-button></div></header>
    <div class="session-board">
      <article v-for="session in cycle.sessions" :key="session.id">
        <div class="session-title"><span>第 {{ session.sequence }} 场</span><el-tag>{{ session.status }}</el-tag></div>
        <strong>{{ date(session.starts_at) }}</strong><p>完成 {{ session.completed_count }} / {{ session.quota }} · 并行 {{ session.concurrent_slots }}</p>
        <div class="actions"><el-button v-if="session.status === 'scheduled'" type="primary" :icon="VideoPlay" :loading="busy" @click="emit('sessionAction', session.id, 'start')">开始叫号</el-button><el-button v-if="session.status === 'running'" :icon="VideoPause" :loading="busy" @click="emit('sessionAction', session.id, 'pause')">暂停</el-button><el-button v-if="session.status === 'paused'" type="primary" :icon="VideoPlay" :loading="busy" @click="emit('sessionAction', session.id, 'resume')">恢复</el-button><el-button v-if="session.status === 'running' || session.status === 'paused'" type="danger" plain :loading="busy" @click="emit('sessionAction', session.id, 'close')">结束场次</el-button></div>
      </article>
    </div>
    <div v-if="statistics?.calls.some(call => call.status === 'accepted')" class="active-calls">
      <h3>已接受，等待行政确认</h3>
      <article v-for="call in statistics.calls.filter(item => item.status === 'accepted')" :key="call.id"><div><strong>{{ call.queue_number }} 号 · {{ call.user_name }}</strong><span>{{ call.responded_at ? date(call.responded_at) : '' }}</span></div><el-button type="danger" plain @click="emit('callAction', call.id, 'no-show')">未到场并跳号</el-button><el-button type="success" :icon="Promotion" @click="emit('callAction', call.id, 'complete')">完成并叫下一位</el-button></article>
    </div>
  </section>
</template>

<style scoped>
.operations{padding:20px;border:1px solid #e9e5f0;border-radius:18px;background:#fff}.operations>header{display:flex;justify-content:space-between;align-items:flex-start;gap:16px}.operations h2,.operations h3{margin:0;color:#302943}.operations p{margin:5px 0;color:#8a8295}.session-board{display:grid;grid-template-columns:1fr 1fr;gap:14px;margin-top:16px}.session-board>article{padding:16px;border-radius:15px;background:#f8f5fc}.session-title{display:flex;justify-content:space-between;margin-bottom:10px}.session-board strong{color:#473b63}.actions{display:flex;flex-wrap:wrap;gap:8px}.active-calls{margin-top:18px}.active-calls>article{display:flex;align-items:center;gap:10px;margin-top:9px;padding:12px 14px;border-radius:12px;background:#fff7e8}.active-calls>article>div{display:flex;flex:1;flex-direction:column}.active-calls span{font-size:12px;color:#938779}@media(max-width:760px){.operations>header{flex-direction:column}.session-board{grid-template-columns:1fr}.active-calls>article{align-items:flex-start;flex-wrap:wrap}}
</style>
