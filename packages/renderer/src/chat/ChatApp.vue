<script setup lang="ts">
import { computed, onActivated, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type {
  Conversation as ConversationRecord,
  ProviderConfig,
  StoredMessage,
} from '@puck/contract'
import {
  Conversation,
  ConversationContent,
  ConversationScrollButton,
} from '@/components/ai-elements/conversation'
import {
  Message,
  MessageAction,
  MessageActions,
  MessageContent,
  MessageResponse,
} from '@/components/ai-elements/message'
import { Shimmer } from '@/components/ai-elements/shimmer'
import { Suggestion } from '@/components/ai-elements/suggestion'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  CopyIcon,
  MoreHorizontalIcon,
  PlusIcon,
  Trash2Icon,
} from '@lucide/vue'
import { useRoute, useRouter } from 'vue-router'
import { bridge } from '../bridge'
import { persona } from '../character'
import PuckAvatar from '../shared/PuckAvatar.vue'
import ChatComposer from './ChatComposer.vue'
import ChatReasoning from './ChatReasoning.vue'

interface ChatMessage {
  id: string
  role: 'user' | 'assistant'
  text: string
  reasoning: string
}

const conversations = ref<ConversationRecord[]>([])
const activeConversationId = ref('')
const messages = ref<ChatMessage[]>([])
const pending = ref(false)
const errorText = ref('')
const toast = ref('')
const providers = ref<ProviderConfig[]>([])
const activeProviderId = ref('')
const selectedProviderId = ref('')
const deleteDialogOpen = ref(false)
const deleteTarget = ref<ConversationRecord | null>(null)
const composerRef = ref<InstanceType<typeof ChatComposer> | null>(null)
const unsubscribers: Array<() => void> = []

const suggestions = ['帮我想个周末计划', '给我讲个冷笑话', '解释一下什么是 RAG']

const route = useRoute()
const router = useRouter()
let activatedOnce = false

let pendingMessageId = ''
let viewEpoch = 0
let toastTimer: number | undefined

const streamingMessageId = computed(() => {
  if (!pending.value) return ''
  const last = messages.value[messages.value.length - 1]
  return last && last.role === 'assistant' ? last.id : ''
})

const conversationGroups = computed(() =>
  [
    {
      label: '今天',
      items: conversations.value.filter((conversation) => isToday(conversation.updatedAt)),
    },
    {
      label: '更早',
      items: conversations.value.filter((conversation) => !isToday(conversation.updatedAt)),
    },
  ].filter((group) => group.items.length > 0),
)

function isToday(timestamp: number): boolean {
  return new Date(timestamp).toDateString() === new Date().toDateString()
}

function isMessageStreaming(message: ChatMessage): boolean {
  return pending.value && message.id === streamingMessageId.value
}

function isReasoningStreaming(message: ChatMessage): boolean {
  return isMessageStreaming(message) && message.reasoning.length > 0 && !message.text
}

function showToast(text: string) {
  toast.value = text
  window.clearTimeout(toastTimer)
  toastTimer = window.setTimeout(() => {
    toast.value = ''
  }, 1600)
}

async function copyText(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    showToast('已复制')
    return
  } catch {
    const area = document.createElement('textarea')
    area.value = text
    area.style.position = 'fixed'
    area.style.opacity = '0'
    document.body.appendChild(area)
    area.select()
    const ok = document.execCommand('copy')
    area.remove()
    showToast(ok ? '已复制' : '复制失败')
  }
}

function setError(error: unknown) {
  const text = error instanceof Error ? error.message : String(error)
  errorText.value = text.length > 280 ? `${text.slice(0, 280)}…` : text
}

function fromStored(message: StoredMessage): ChatMessage {
  return {
    id: `h-${message.id}`,
    role: message.role === 'assistant' ? 'assistant' : 'user',
    text: message.content,
    reasoning: message.reasoning ?? '',
  }
}

function findMessage(id: string): ChatMessage | undefined {
  return messages.value.find((message) => message.id === id)
}

async function refreshConversations() {
  try {
    conversations.value = await bridge.listConversations()
  } catch (error) {
    setError(error)
  }
}

async function loadProviders() {
  try {
    const settings = await bridge.getSettings()
    providers.value = (settings.providers ?? []).filter(
      (provider) => provider.type !== 'assistant',
    )
    activeProviderId.value = settings.activeProviderId
    if (
      !selectedProviderId.value ||
      !providers.value.some((provider) => provider.id === selectedProviderId.value)
    ) {
      const active = providers.value.find(
        (provider) => provider.id === settings.activeProviderId,
      )
      selectedProviderId.value = active?.id ?? providers.value[0]?.id ?? ''
    }
  } catch {
    // 聊天仍可由后端使用当前供应商，不因模型列表加载失败而中断。
  }
}

function applyConversationProvider(conversationID: string) {
  const conversation = conversations.value.find((item) => item.id === conversationID)
  if (
    conversation?.providerId &&
    providers.value.some((provider) => provider.id === conversation.providerId)
  ) {
    selectedProviderId.value = conversation.providerId
    return
  }
  selectedProviderId.value = activeProviderId.value || providers.value[0]?.id || ''
}

function syncRouteToConversation(id: string) {
  if (route.name !== 'chat') return
  if (route.params.id === id) return
  void router.replace({ name: 'chat', params: { id } })
}

async function openConversation(id: string) {
  const epoch = ++viewEpoch
  activeConversationId.value = id
  errorText.value = ''
  try {
    const stored = await bridge.getMessages(id)
    if (epoch !== viewEpoch) return
    messages.value = stored.map(fromStored)
    applyConversationProvider(id)
    syncRouteToConversation(id)
  } catch (error) {
    if (epoch === viewEpoch) {
      setError(error)
    }
  }
}

function newConversation() {
  viewEpoch++
  activeConversationId.value = ''
  messages.value = []
  errorText.value = ''
  composerRef.value?.clear()
  selectedProviderId.value = activeProviderId.value || providers.value[0]?.id || ''
  if (route.name === 'chat' && route.params.id) {
    void router.replace({ name: 'chat' })
  }
}

async function removeConversation(id: string) {
  try {
    await bridge.deleteConversation(id)
    if (activeConversationId.value === id) newConversation()
    await refreshConversations()
  } catch (error) {
    setError(error)
  }
}

function requestDelete(conversation: ConversationRecord) {
  deleteTarget.value = conversation
  deleteDialogOpen.value = true
}

async function confirmDelete() {
  const target = deleteTarget.value
  deleteDialogOpen.value = false
  deleteTarget.value = null
  if (target) await removeConversation(target.id)
}

function usePrompt(text: string) {
  composerRef.value?.setText(text)
}

function updateSelectedProvider(id: string) {
  selectedProviderId.value = id
}

async function send(textInput?: string) {
  const text = (typeof textInput === 'string' ? textInput : '').trim()
  if (!text || pending.value) return

  errorText.value = ''
  pending.value = true
  const expectedActive = activeConversationId.value
  const epoch = viewEpoch
  messages.value.push({ id: `local-${Date.now()}`, role: 'user', text, reasoning: '' })

  try {
    const result = await bridge.sendChat(text, expectedActive, selectedProviderId.value)
    pendingMessageId = result.messageId
    if (epoch !== viewEpoch) {
      await refreshConversations()
      return
    }
    activeConversationId.value = result.conversationId
    if (route.name === 'chat' && route.params.id !== result.conversationId) {
      void router.replace({ name: 'chat', params: { id: result.conversationId } })
    }
    messages.value.push({ id: result.messageId, role: 'assistant', text: '', reasoning: '' })
    await refreshConversations()
  } catch (error) {
    pending.value = false
    pendingMessageId = ''
    if (epoch === viewEpoch) {
      setError(error)
    }
  }
}

onMounted(async () => {
  void bridge.setPersona(persona.systemPrompt).catch(() => {})
  unsubscribers.push(
    bridge.onChatDelta((event) => {
      if (event.conversationId !== activeConversationId.value) return
      const message = findMessage(event.messageId)
      if (!message) return
      message.text += event.text
    }),
    bridge.onChatReasoning((event) => {
      if (event.conversationId !== activeConversationId.value) return
      const message = findMessage(event.messageId)
      if (!message) return
      message.reasoning += event.text
    }),
    bridge.onChatDone((event) => {
      if (event.messageId === pendingMessageId) {
        pending.value = false
        pendingMessageId = ''
      }
      if (event.conversationId !== activeConversationId.value) {
        void refreshConversations()
        return
      }
      const message = findMessage(event.messageId)
      if (!message) {
        void openConversation(event.conversationId)
        void refreshConversations()
        return
      }
      if (event.text) message.text = event.text
      if (event.reasoning) message.reasoning = event.reasoning
      void refreshConversations()
    }),
    bridge.onChatError((event) => {
      if (event.messageId === pendingMessageId) {
        pending.value = false
        pendingMessageId = ''
      }
      if (event.conversationId && event.conversationId !== activeConversationId.value) return
      errorText.value = event.message
    }),
  )

  await Promise.all([loadProviders(), refreshConversations()])
  const routeId = typeof route.params.id === 'string' ? route.params.id : ''
  if (routeId) {
    await openConversation(routeId)
  } else {
    const mostRecent = conversations.value[0]
    if (mostRecent) await openConversation(mostRecent.id)
  }

  window.addEventListener('focus', onWindowFocus)
})

onActivated(async () => {
  if (!activatedOnce) {
    activatedOnce = true
    return
  }
  await Promise.all([loadProviders(), refreshConversations()])
  if (
    activeConversationId.value &&
    !conversations.value.some(
      (conversation) => conversation.id === activeConversationId.value,
    )
  ) {
    newConversation()
    return
  }
  const routeId = typeof route.params.id === 'string' ? route.params.id : ''
  if (routeId && routeId !== activeConversationId.value) {
    await openConversation(routeId)
  }
})

watch(
  () => route.params.id,
  (id) => {
    if (route.name !== 'chat') return
    const next = typeof id === 'string' ? id : ''
    if (!next || next === activeConversationId.value) return
    void openConversation(next)
  },
)

function onWindowFocus() {
  void loadProviders()
}

onBeforeUnmount(() => {
  window.clearTimeout(toastTimer)
  window.removeEventListener('focus', onWindowFocus)
  for (const unsubscribe of unsubscribers) unsubscribe()
})
</script>

<template>
  <div class="flex h-full w-full flex-col">
    <header
      class="chat-header flex items-center justify-between gap-4 border-b border-border bg-card px-5 py-2.5 [-webkit-app-region:drag]"
    >
      <div class="flex items-center gap-3">
        <PuckAvatar class="size-8 rounded-full shadow-sm" />
        <div class="flex flex-col">
          <h1 class="m-0 text-[15px] font-bold leading-5 tracking-[0.01em]">Puck</h1>
          <p class="mb-0 mt-0 text-[11px] leading-4 text-muted-foreground">
            住在你桌角的小伙伴
          </p>
        </div>
      </div>
    </header>

    <div class="flex min-h-0 flex-1">
      <aside
        class="chat-sidebar flex min-h-0 w-[218px] flex-none flex-col overflow-hidden border-r border-border bg-card p-2.5"
      >
        <Button
          type="button"
          variant="outline"
          size="sm"
          class="mb-2 w-full justify-start gap-2"
          @click="newConversation"
        >
          <PlusIcon class="size-3.5" />
          新对话
        </Button>

        <ScrollArea class="min-h-0 flex-1">
          <p
            v-if="!conversations.length"
            class="px-2 py-3 text-xs text-muted-foreground"
          >
            还没有对话
          </p>
          <div v-for="group in conversationGroups" :key="group.label">
            <p
              class="px-2 pb-1 pt-3 font-mono text-[10px] uppercase tracking-[0.14em] text-muted-foreground"
            >
              {{ group.label }}
            </p>
            <div
              v-for="conversation in group.items"
              :key="conversation.id"
              class="chat-conversation-row group flex items-center gap-0.5"
              :class="{
                'chat-conversation-row--active': conversation.id === activeConversationId,
              }"
            >
              <button
                type="button"
                class="chat-conversation-open min-w-0 flex-1 truncate rounded-lg px-2.5 py-2 text-left text-[12.5px] leading-5 transition-colors"
                :title="conversation.title || '新对话'"
                :aria-current="
                  conversation.id === activeConversationId ? 'page' : undefined
                "
                @click="openConversation(conversation.id)"
              >
                {{ conversation.title || '新对话' }}
              </button>
              <DropdownMenu>
                <DropdownMenuTrigger as-child>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-xs"
                    class="chat-conversation-menu"
                    aria-label="对话操作"
                  >
                    <MoreHorizontalIcon class="size-3.5" />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" class="w-36">
                  <DropdownMenuItem
                    class="text-destructive focus:bg-destructive/10 focus:text-destructive"
                    @select="requestDelete(conversation)"
                  >
                    <Trash2Icon class="size-3.5" />
                    删除对话
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
          </div>
        </ScrollArea>
      </aside>

      <section class="flex min-w-0 flex-1 flex-col">
        <main class="flex min-h-0 flex-1 flex-col">
          <div v-if="messages.length === 0" class="chat-welcome">
            <div class="welcome-hero">
              <div class="welcome-avatar">
                <PuckAvatar class="size-[74px] rounded-full" />
              </div>
              <h2 class="welcome-title">和 Puck 聊点什么吧</h2>
              <p class="welcome-desc">
                在设置里接入模型后就能真正对话；没有配置时使用本地回声。
              </p>
            </div>
            <div class="welcome-prompts">
              <span class="welcome-prompts-title">试试这些</span>
              <div class="welcome-prompts-grid">
                <Suggestion
                  v-for="text in suggestions"
                  :key="text"
                  :suggestion="text"
                  class="welcome-prompt"
                  @click="usePrompt"
                >
                  <span class="welcome-prompt-spark" aria-hidden="true">✦</span>
                  <span class="welcome-prompt-text">{{ text }}</span>
                  <svg
                    class="welcome-prompt-arrow"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="1.8"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    aria-hidden="true"
                  >
                    <path d="M5 12h14" />
                    <path d="m13 6 6 6-6 6" />
                  </svg>
                </Suggestion>
              </div>
            </div>
          </div>

          <Conversation
            v-else
            class="chat-conversation min-h-0 flex-1"
            aria-label="聊天消息"
            resize="instant"
          >
            <ConversationContent class="chat-conversation-content gap-5 px-5 py-5">
              <Message
                v-for="message in messages"
                :key="message.id"
                :from="message.role"
                class="chat-message"
              >
                <PuckAvatar
                  v-if="message.role === 'assistant'"
                  class="mt-0.5 size-8 flex-none rounded-full"
                />
                <MessageContent
                  :class="
                    message.role === 'user'
                      ? 'chat-message-content chat-message-content--user'
                      : 'chat-message-content chat-message-content--assistant'
                  "
                >
                  <ChatReasoning
                    v-if="message.role === 'assistant' && message.reasoning"
                    :content="message.reasoning"
                    :is-streaming="isReasoningStreaming(message)"
                    :default-open="isMessageStreaming(message)"
                  />
                  <Shimmer
                    v-if="
                      isMessageStreaming(message) &&
                      !message.text &&
                      !message.reasoning
                    "
                    :duration="1.5"
                    class="text-xs"
                  >
                    正在思考…
                  </Shimmer>
                  <div v-if="message.role === 'user'" class="whitespace-pre-wrap">
                    {{ message.text }}
                  </div>
                  <MessageResponse
                    v-else-if="message.text"
                    :content="message.text"
                    class="chat-message-markdown"
                  />
                  <MessageActions
                    v-if="
                      message.role === 'assistant' &&
                      message.text &&
                      !isMessageStreaming(message)
                    "
                    class="chat-message-actions"
                  >
                    <MessageAction
                      tooltip="复制"
                      label="复制"
                      @click="copyText(message.text)"
                    >
                      <CopyIcon class="size-3.5" />
                    </MessageAction>
                  </MessageActions>
                </MessageContent>
              </Message>
            </ConversationContent>
            <ConversationScrollButton aria-label="滚动到底部" />
          </Conversation>
        </main>

        <p
          v-if="errorText"
          class="m-0 break-words bg-puck-danger-soft px-5 py-2.5 text-xs text-puck-danger"
        >
          {{ errorText }}
        </p>

        <footer class="w-full px-5 pb-4 pt-1">
          <ChatComposer
            ref="composerRef"
            :providers="providers"
            :selected-provider-id="selectedProviderId"
            :pending="pending"
            @update:selectedProviderId="updateSelectedProvider"
            @submit="send"
            @error="setError"
          />
        </footer>
      </section>
    </div>

    <transition name="toast">
      <div
        v-if="toast"
        class="pointer-events-none fixed bottom-[98px] left-1/2 z-20 -translate-x-1/2 rounded-full bg-foreground px-3.5 py-2 text-xs text-background"
      >
        {{ toast }}
      </div>
    </transition>

    <AlertDialog v-model:open="deleteDialogOpen">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>删除这个对话？</AlertDialogTitle>
          <AlertDialogDescription>
            “{{ deleteTarget?.title || '新对话' }}”将永久删除，无法恢复。
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>取消</AlertDialogCancel>
          <AlertDialogAction
            class="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            @click="confirmDelete"
          >
            删除
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </div>
</template>
