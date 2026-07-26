import { api } from './api'
import type { Conversation, Message, RunEvent } from '@/types/domain'

export const chatService = {
  listConversations: () => api.get<Conversation[]>('/api/v1/conversations'),
  createConversation: () => api.post<Conversation>('/api/v1/conversations'),
  deleteConversation: (id: string) => api.delete(`/api/v1/conversations/${id}`),
  resetContext: (id: string) => api.post<void>(`/api/v1/conversations/${id}/context/reset`),
  listMessages: (id: string) => api.get<Message[]>(`/api/v1/conversations/${id}/messages`),
  sendMessage: (id: string, content: string) => api.post<{ run_id: string }>(`/api/v1/conversations/${id}/messages`, { content }),
  cancelRun: (runId: string) => api.post<void>(`/api/v1/runs/${runId}/cancel`),
  feedback: (messageId: string, positive: boolean) => api.post<void>(`/api/v1/messages/${messageId}/feedback`, { positive }),
  stream(runId: string, onEvent: (event: RunEvent) => void, onConnectionError: () => void) {
    const source = new EventSource(`/api/v1/runs/${runId}/events`, { withCredentials: true })
    const types: RunEvent['type'][] = ['status', 'delta', 'citation', 'done', 'error']
    types.forEach((type) => source.addEventListener(type, (raw) => { const event = JSON.parse((raw as MessageEvent).data) as RunEvent; onEvent(event); if (type === 'done' || type === 'error') source.close() }))
    source.onerror = () => { source.close(); onConnectionError() }
    return () => source.close()
  },
}
