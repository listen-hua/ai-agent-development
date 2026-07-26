<script setup lang="ts">
import { nextTick, ref, watch } from 'vue'
import { Bell, CopyDocument, Document, Refresh, Star, Warning } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import type { Citation, Message } from '@/types/domain'
import MeetingBookingCard from './MeetingBookingCard.vue'
const props = defineProps<{ messages: Message[]; stage: string }>(); const emit = defineEmits<{ citation: [citation: Citation]; retry: [message: Message]; feedback: [message: Message, positive: boolean]; confirmReminder: [message: Message]; cancelReminder: [message: Message]; confirmMeeting: [message: Message, optionId?: string]; cancelMeeting: [message: Message] }>(); const viewport = ref<HTMLElement>()
watch(() => props.messages.map((item) => item.content).join('|'), async () => { await nextTick(); viewport.value?.scrollTo({ top: viewport.value.scrollHeight, behavior: 'smooth' }) })
async function copy(content: string) { await navigator.clipboard.writeText(content); ElMessage.success('已复制回答') }
function scheduleText(message: Message) { const schedule = message.reminder_action?.schedule; if (!schedule) return ''; if (schedule.type === 'once') return schedule.once_at ? new Date(schedule.once_at).toLocaleString('zh-CN', { hour12: false }) : ''; if (schedule.type === 'daily') return `每天 ${schedule.local_time}`; if (schedule.type === 'workday') return `每个公司工作日 ${schedule.local_time}`; const names = ['一','二','三','四','五','六','日']; return `每${(schedule.weekdays || []).map((day) => `周${names[day - 1]}`).join('、')} ${schedule.local_time}` }
</script>
<template>
  <div ref="viewport" class="transcript">
    <div v-if="!messages.length" class="welcome-state"><div class="welcome-orbit"><div class="welcome-glyph">知</div><span /><span /></div><p class="welcome-kicker">ADMINISTRATIVE INTELLIGENCE</p><h2>今天想了解哪项公司制度？</h2><p>我会先检索你有权查看的制度，再给出带版本和原文引用的回答。</p><div class="suggestion-grid"><button v-for="question in ['年假需要提前多久申请？','差旅费用的报销时限是什么？','病假需要提供哪些材料？','在哪里查看最新办公规范？']" :key="question" @click="$emit('retry', { content: question } as Message)"><Document />{{ question }}</button></div></div>
    <div v-else class="message-stack">
      <article v-for="message in messages" :key="message.id" class="message" :class="message.role">
        <div class="message-avatar">{{ message.role === 'assistant' ? '知' : '我' }}</div>
        <div class="message-body"><div class="message-label">{{ message.role === 'assistant' ? '知行 · 行政助手' : '你' }}</div><div class="message-content" :class="{ pending: message.pending }">{{ message.content }}<span v-if="message.pending && message.content" class="typing-caret" aria-hidden="true" /><span v-if="message.pending && !message.content" class="thinking"><i /><i /><i />{{ stage }}</span></div>
          <div v-if="message.reminder_action" class="reminder-confirmation" :class="message.reminder_action.status"><div class="reminder-confirmation-icon"><Bell /></div><div><strong>{{ message.reminder_action.content || '提醒操作' }}</strong><span v-if="scheduleText(message)">{{ scheduleText(message) }}</span><small>有效期至 {{ new Date(message.reminder_action.expires_at).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }) }}</small></div><div v-if="message.reminder_action.status === 'pending'" class="reminder-confirmation-actions"><el-button size="small" @click="emit('cancelReminder', message)">取消</el-button><el-button type="primary" size="small" @click="emit('confirmReminder', message)">确认</el-button></div><el-tag v-else size="small" :type="message.reminder_action.status === 'confirmed' ? 'success' : 'info'">{{ message.reminder_action.status === 'confirmed' ? '已确认' : '已取消' }}</el-tag></div>
          <MeetingBookingCard v-if="message.meeting_booking_action" :action="message.meeting_booking_action" @confirm="(optionId) => emit('confirmMeeting', message, optionId)" @cancel="emit('cancelMeeting', message)" />
          <div v-if="message.citations?.length" class="citation-chips"><button v-for="(citation, index) in message.citations" :key="citation.id" @click="emit('citation', citation)"><span>{{ index + 1 }}</span>{{ citation.title }} · {{ citation.version }}</button></div>
          <div v-if="message.role === 'assistant' && !message.pending" class="message-tools"><button @click="copy(message.content)"><CopyDocument />复制</button><button @click="emit('feedback', message, true)"><Star />有帮助</button><button @click="emit('retry', message)"><Refresh />重新生成</button></div>
        </div>
      </article>
      <div class="answer-notice"><Warning />回答由 AI 基于当前有效制度生成，重要事项请联系行政复核。</div>
    </div>
  </div>
</template>
