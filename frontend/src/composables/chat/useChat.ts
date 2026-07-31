import { computed, onBeforeUnmount, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { chatService } from '@/services/chat'
import { reminderService } from '@/services/reminder'
import { meetingService } from '@/services/meeting'
import type { Citation, Conversation, Message, RunEvent } from '@/types/domain'
import { createClientUUID } from '@/utils/clientId'
import { createTextStreamController, type TextStreamController } from '@/utils/textStream'

export function useChat() {
  const conversations = ref<Conversation[]>([])
  const activeId = ref('')
  const messages = ref<Message[]>([])
  const sending = ref(false)
  const stage = ref('')
  let stopStream: (() => void) | undefined
  let textStream: TextStreamController | undefined
  let activeRunId = ''
  let stopRequested = false

  const activeConversation = computed(() => conversations.value.find((item) => item.id === activeId.value))

  async function loadConversations() {
    conversations.value = await chatService.listConversations()
    if (!activeId.value && conversations.value.length) await selectConversation(conversations.value[0].id)
  }

  async function newConversation() {
    const value = await chatService.createConversation()
    conversations.value.unshift(value)
    activeId.value = value.id
    messages.value = []
  }

  async function selectConversation(id: string) {
    if (sending.value) return
    activeId.value = id
    messages.value = (await chatService.listMessages(id)) || []
  }

  async function removeConversation(id: string) {
    await chatService.deleteConversation(id)
    conversations.value = conversations.value.filter((item) => item.id !== id)
    if (activeId.value !== id) return
    activeId.value = ''
    messages.value = []
    if (conversations.value.length) await selectConversation(conversations.value[0].id)
  }

  async function send(content: string) {
    if (!activeId.value) await newConversation()
    const conversationId = activeId.value
    const now = new Date().toISOString()
    messages.value.push({ id: createClientUUID(), conversation_id: conversationId, role: 'user', content, citations: [], created_at: now })
    const assistant: Message = {
      id: createClientUUID(), conversation_id: conversationId, role: 'assistant', content: '', citations: [], created_at: now, pending: true,
    }
    messages.value.push(assistant)
    sending.value = true
    stopRequested = false
    stage.value = '正在理解问题'
    textStream?.cancel()
    textStream = createTextStreamController((value) => { assistant.content = value })
    try {
      const { run_id } = await chatService.sendMessage(conversationId, content)
      activeRunId = run_id
      if (stopRequested) {
        activeRunId = ''
        await chatService.cancelRun(run_id).catch(() => {
          ElMessage.warning('生成已在当前页面停止，但服务端取消请求未成功')
        })
        return
      }
      stopStream = chatService.stream(
        run_id,
        (event) => handleEvent(event, assistant),
        () => { if (sending.value) failAssistant(assistant, '连接中断，请重新发送') },
      )
    } catch (error) {
      failAssistant(assistant, error instanceof Error ? error.message : '发送失败')
    }
  }

  function handleEvent(event: RunEvent, assistant: Message) {
    if (event.type === 'status') {
      const labels: Record<string, string> = {
        contextualizing: '正在结合上下文理解问题',
        retrieving: '正在检索已授权制度',
        interpreting_reminder: '正在核对提醒时间',
        interpreting_meeting: '正在校验会议室与参会人忙闲',
        generating: '正在生成有依据的回答',
      }
      stage.value = labels[event.metadata?.stage || ''] || '正在处理'
      return
    }
    if (event.type === 'delta') {
      textStream?.push(event.delta || '')
      return
    }
    if (event.type === 'citation' && event.citation) {
      assistant.citations.push(event.citation)
      return
    }
    if (event.type === 'done') {
      const finalText = event.message?.content || assistant.content
      if (textStream) {
        textStream.finish(finalText, () => finishAssistant(assistant, event.message))
      } else {
        assistant.content = finalText
        finishAssistant(assistant, event.message)
      }
      return
    }
    if (event.type === 'error') failAssistant(assistant, event.error || '生成失败')
  }

  function finishAssistant(assistant: Message, message?: Message) {
    if (message) {
      assistant.id = message.id
      assistant.conversation_id = message.conversation_id
      assistant.citations = message.citations
      assistant.model = message.model
	  assistant.reminder_action = message.reminder_action
	  assistant.meeting_booking_action = message.meeting_booking_action
      assistant.created_at = message.created_at
    }
    assistant.pending = false
    sending.value = false
    stage.value = ''
    stopStream = undefined
    textStream = undefined
    activeRunId = ''
    void loadConversations()
  }

  function failAssistant(assistant: Message, reason: string) {
    textStream?.cancel()
    stopStream?.()
    textStream = undefined
    stopStream = undefined
    activeRunId = ''
    assistant.pending = false
    assistant.content = reason
    sending.value = false
    stage.value = ''
    ElMessage.error(reason)
  }

  function stop() {
    stopRequested = true
    const runId = activeRunId
    activeRunId = ''
    stopStream?.()
    textStream?.cancel()
    stopStream = undefined
    textStream = undefined
    sending.value = false
    stage.value = ''
    const last = messages.value.at(-1)
    if (last?.pending) {
      last.pending = false
      last.content ||= '已停止生成。'
    }
    if (runId) {
      void chatService.cancelRun(runId).catch(() => {
        ElMessage.warning('生成已在当前页面停止，但服务端取消请求未成功')
      })
    }
  }

  function retry(message: Message) {
    const index = messages.value.indexOf(message)
    if (index < 0) {
      if (message.content) void send(message.content)
      return
    }
    const previous = messages.value.slice(0, index).reverse().find((item) => item.role === 'user')
    if (previous) void send(previous.content)
  }

  async function feedback(message: Message, positive: boolean) {
    try {
      await chatService.feedback(message.id, positive)
      ElMessage.success(positive ? '感谢反馈，这会帮助我们改进回答' : '已记录问题，行政知识管理员会复核')
    } catch (error) {
      ElMessage.error(error instanceof Error ? error.message : '反馈提交失败')
    }
  }

  async function confirmReminder(message: Message) {
    const action = message.reminder_action
    if (!action || action.status !== 'pending') return
    const result = await reminderService.confirm(action.id)
    message.reminder_action = result.action
    const labels = { create: '提醒已设置，我会通过飞书机器人私聊通知你。', update: '提醒已更新。', pause: '提醒已暂停。', resume: '提醒已恢复。', delete: '提醒已删除。' }
    message.content = labels[action.action]
    ElMessage.success('提醒操作已确认')
  }

  async function cancelReminder(message: Message) {
    const action = message.reminder_action
    if (!action || action.status !== 'pending') return
    message.reminder_action = await reminderService.cancelAction(action.id)
    message.content = '已取消本次提醒操作。'
    ElMessage.info('已取消')
  }

  async function confirmMeeting(message: Message, optionId?: string) {
    const action = message.meeting_booking_action
    if (!action || action.status !== 'pending') return
    action.status = 'processing'
    try {
      const result = await meetingService.confirmAction(action.id, optionId)
      message.meeting_booking_action = result.action
      if (action.intent === 'cancel') message.content = '会议室预约已取消，飞书日程和会后提醒已同步取消。'
      else if (action.intent === 'reschedule') message.content = `改期成功：${result.booking?.room_name || '会议室'}。原预约已释放。`
      else message.content = `预约成功：${result.booking?.room_name || '会议室'}。会后会通过飞书提醒你归还会议室。`
      ElMessage.success(action.intent === 'cancel' ? '预约已取消' : '会议室预约成功')
    } catch (error) {
      action.status = 'pending'
      const reason = error instanceof Error ? error.message : '会议室操作失败'
      ElMessage.error(reason)
    }
  }

  async function cancelMeeting(message: Message) {
    const action = message.meeting_booking_action
    if (!action || action.status !== 'pending') return
    message.meeting_booking_action = await meetingService.cancelAction(action.id)
    message.content = '已取消本次会议室操作，未创建或修改任何飞书日程。'
    ElMessage.info('已取消')
  }

  onBeforeUnmount(() => {
    stopStream?.()
    textStream?.cancel()
    if (activeRunId) void chatService.cancelRun(activeRunId).catch(() => undefined)
  })

  return {
    conversations, activeId, activeConversation, messages, sending, stage,
    loadConversations, newConversation, selectConversation, removeConversation, send, stop, retry, feedback, confirmReminder, cancelReminder, confirmMeeting, cancelMeeting,
  }
}

export type SelectedCitation = Citation | null
