<script setup lang="ts">
import { onMounted, ref } from 'vue'
import type { Citation, Message } from '@/types/domain'
import { useChat } from '@/composables/chat/useChat'
import ConversationList from '@/components/chat/ConversationList.vue'
import ChatTranscript from '@/components/chat/ChatTranscript.vue'
import ChatComposer from '@/components/chat/ChatComposer.vue'
import CitationDrawer from '@/components/chat/CitationDrawer.vue'

const chat = useChat(); const citation = ref<Citation | null>(null); const drawerOpen = ref(false)
function showCitation(value: Citation) { citation.value = value; drawerOpen.value = true }
function retry(message: Message) { if (!message.id) void chat.send(message.content); else chat.retry(message) }
onMounted(async () => { await chat.loadConversations(); if (!chat.activeId.value) await chat.newConversation() })
</script>
<template><div class="chat-page"><ConversationList :conversations="chat.conversations.value" :active-id="chat.activeId.value" @select="chat.selectConversation" @create="chat.newConversation" @remove="chat.removeConversation" /><section class="chat-workspace"><header class="chat-header"><div><span class="live-indicator" />已连接制度知识库、个人提醒、会议室与按摩排号</div><small>所有行政操作均需由你确认</small></header><ChatTranscript :messages="chat.messages.value" :stage="chat.stage.value" @citation="showCitation" @retry="retry" @feedback="chat.feedback" @confirm-reminder="chat.confirmReminder" @cancel-reminder="chat.cancelReminder" @confirm-meeting="chat.confirmMeeting" @cancel-meeting="chat.cancelMeeting" @meeting-draft-updated="chat.applyMeetingDraftResult" @massage-action="chat.confirmMassageAction" /><ChatComposer :disabled="chat.sending.value" @send="chat.send" @stop="chat.stop" /></section><CitationDrawer v-model="drawerOpen" :citation="citation" /></div></template>
