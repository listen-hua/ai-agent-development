<script setup lang="ts">
import { Clock, Tickets } from '@element-plus/icons-vue'
import type { MassageMe } from '@/types/domain'

defineProps<{ item: MassageMe; loading?: boolean }>()
const emit = defineEmits<{ respond: [item: MassageMe, action: 'enroll' | 'decline' | 'withdraw'] }>()
const date = (value: string) => new Date(value).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false })
const statusLabel: Record<string, string> = { enrolled: '已报名', declined: '本月不参加', withdrawn: '已退出', completed: '已完成按摩', unserved: '本月未安排到' }
</script>

<template>
  <article class="massage-cycle-card">
    <header><div><small>{{ item.cycle.service_month }}</small><h3>{{ item.cycle.title }}</h3></div><el-tag :type="item.cycle.status === 'signup_open' ? 'success' : 'info'">{{ item.cycle.status === 'signup_open' ? '报名开放' : item.cycle.status === 'in_progress' ? '叫号中' : '已安排' }}</el-tag></header>
    <div class="session-grid"><div v-for="session in item.cycle.sessions" :key="session.id"><Clock /><span>第 {{ session.sequence }} 场</span><strong>{{ date(session.starts_at) }}</strong><small>名额 {{ session.quota }} · 并行 {{ session.concurrent_slots }}</small></div></div>
    <div v-if="item.enrollment" class="queue-status"><Tickets /><div><span>{{ statusLabel[item.enrollment.status] || item.enrollment.status }}</span><strong v-if="item.enrollment.queue_number">{{ item.enrollment.queue_number }} 号</strong><small v-if="item.current_queue_number">当前叫到 {{ item.current_queue_number }} 号</small></div></div>
    <div v-if="item.current_call?.status === 'awaiting_response'" class="calling-alert">现在轮到你，请在飞书卡片中于 3 分钟内确认。</div>
    <div v-else-if="item.current_call?.status === 'accepted'" class="calling-alert accepted">你已接受叫号，请前往按摩室。</div>
    <footer v-if="item.cycle.status === 'signup_open'">
      <template v-if="!item.enrollment || item.enrollment.status !== 'enrolled'"><el-button :loading="loading" @click="emit('respond', item, 'decline')">本月不参加</el-button><el-button type="primary" :loading="loading" @click="emit('respond', item, 'enroll')">接受排号</el-button></template>
      <el-button v-else type="danger" plain :loading="loading" @click="emit('respond', item, 'withdraw')">退出排号</el-button>
    </footer>
  </article>
</template>

<style scoped>
.massage-cycle-card{padding:22px;border:1px solid #ebe7f4;border-radius:20px;background:#fff;box-shadow:0 12px 30px rgba(66,48,104,.07)}header{display:flex;justify-content:space-between;gap:16px}h3{margin:3px 0 0;color:#2f2940;font-size:20px}header small{color:#8a7aaa}.session-grid{display:grid;grid-template-columns:1fr 1fr;gap:12px;margin:18px 0}.session-grid>div{display:grid;grid-template-columns:22px 1fr;gap:3px 8px;padding:14px;border-radius:14px;background:#f7f4fc;color:#7865ad}.session-grid svg{width:18px}.session-grid strong,.session-grid small{grid-column:2}.session-grid strong{color:#3d3550}.session-grid small{color:#8d849c}.queue-status{display:flex;align-items:center;gap:12px;padding:14px 16px;border-radius:14px;background:linear-gradient(110deg,#5d48b5,#8069dc);color:#fff}.queue-status>svg{width:28px}.queue-status div{display:flex;align-items:baseline;gap:12px}.queue-status strong{font-size:24px}.queue-status small{opacity:.8}.calling-alert{margin-top:12px;padding:11px 14px;border-radius:10px;background:#fff2df;color:#a66000}.calling-alert.accepted{background:#e9f8ef;color:#297a48}footer{display:flex;justify-content:flex-end;margin-top:16px}@media(max-width:650px){.session-grid{grid-template-columns:1fr}.queue-status div{flex-wrap:wrap}}
</style>
