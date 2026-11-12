export const EVENTS = {
  chatDelta: 'chat:delta',
  chatDone: 'chat:done',
  chatError: 'chat:error',
  chatState: 'chat:state',
  mascotOpenChat: 'mascot:open-chat',
  mascotScale: 'mascot:scale',
  themeChanged: 'theme:changed',
  bubbleShow: 'bubble:show',
} as const

export type ChatState = 'idle' | 'thinking' | 'speaking'
export type ThemeMode = 'light' | 'dark' | 'system'
export type BubbleSide = 'left' | 'right'

export interface ChatDeltaEvent {
  messageId: string
  conversationId: string
  text: string
}

export interface ChatDoneEvent {
  messageId: string
  conversationId: string
  text: string
}

export interface ChatErrorEvent {
  messageId?: string
  conversationId?: string
  message: string
}

export interface ChatStateEvent {
  state: ChatState
}

export interface MascotScaleEvent {
  scale: number
}

export interface ThemeChangedEvent {
  theme: ThemeMode
}

export interface BubbleState {
  text: string
  side: BubbleSide
}

export interface BubbleShowEvent {
  text: string
  side: BubbleSide
}

export type ProviderType = 'openai-compatible' | 'anthropic' | 'gemini'

export interface ProviderConfig {
  id: string
  name: string
  type: ProviderType | string
  baseURL: string
  model: string
  hasKey: boolean
  keyMask?: string
  webSearch?: boolean | null
}

export interface Settings {
  providers: ProviderConfig[]
  activeProviderId: string
  mascotScale: number
  theme: ThemeMode
}

export interface ProviderTestResult {
  ok: boolean
  message: string
  latencyMs: number
}

export interface Conversation {
  id: string
  title: string
  providerId: string
  createdAt: number
  updatedAt: number
}

export interface StoredMessage {
  id: number
  conversationId: string
  role: 'user' | 'assistant'
  content: string
  createdAt: number
}

export interface SendResult {
  messageId: string
  conversationId: string
}
