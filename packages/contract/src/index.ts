export const EVENTS = {
  chatDelta: 'chat:delta',
  chatDone: 'chat:done',
  chatError: 'chat:error',
  chatState: 'chat:state',
  mascotOpenChat: 'mascot:open-chat',
  mascotScale: 'mascot:scale',
} as const

export type ChatState = 'idle' | 'thinking' | 'speaking'

export interface ChatDeltaEvent {
  messageId: string
  text: string
}

export interface ChatDoneEvent {
  messageId: string
  text: string
}

export interface ChatErrorEvent {
  messageId?: string
  message: string
}

export interface ChatStateEvent {
  state: ChatState
}

export interface MascotScaleEvent {
  scale: number
}

export interface ProviderConfig {
  id: string
  name: string
  type: 'openai-compatible'
  baseURL: string
  model: string
  hasKey: boolean
}

export interface Settings {
  provider: ProviderConfig | null
  mascotScale: number
}

export interface SendMessageRequest {
  conversationId?: string
  text: string
}

export interface SendMessageResponse {
  messageId: string
}
