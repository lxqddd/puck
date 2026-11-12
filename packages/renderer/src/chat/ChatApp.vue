<script setup lang="ts">
import { computed, h, onBeforeUnmount, onMounted, ref } from 'vue'
import type { Conversation, StoredMessage } from '@puck/contract'
import {
  Actions,
  Bubble,
  Conversations,
  Prompts,
  Sender,
  Welcome,
  XProvider,
} from 'ant-design-x-vue'
import type {
  BubbleItemType,
  ConversationItemType,
  PromptProps,
  RoleType,
} from 'ant-design-x-vue'
import type { MenuProps } from 'ant-design-vue'
import { bridge } from '../bridge'
import { persona } from '../character'

interface Message {
  id: string
  role: 'user' | 'assistant'
  text: string
}

const conversations = ref<Conversation[]>([])
const activeConversationId = ref('')
const messages = ref<Message[]>([])
const input = ref('')
const pending = ref(false)
const errorText = ref('')
const toast = ref('')
const unsubscribers: Array<() => void> = []

const suggestions = ['帮我想个周末计划', '给我讲个冷笑话', '解释一下什么是 RAG']

let pendingMessageId = ''
let viewEpoch = 0
let toastTimer: number | undefined

const streamingMessageId = computed(() => {
  if (!pending.value) return ''
  const last = messages.value[messages.value.length - 1]
  return last && last.role === 'assistant' ? last.id : ''
})

const themeConfig = {
  token: {
    colorPrimary: '#35bd98',
    colorText: '#17342d',
    colorTextSecondary: '#6b7f79',
    colorBgContainer: '#ffffff',
    colorBorder: 'rgba(23, 52, 45, 0.12)',
    borderRadius: 12,
    fontFamily:
      "'SF Pro Rounded', 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', system-ui, sans-serif",
  },
}

const puckAvatar = h('span', { class: 'bubble-avatar' }, 'P')

const roles: RoleType = {
  ai: {
    placement: 'start',
    variant: 'filled',
    shape: 'corner',
    avatar: puckAvatar,
  },
  user: {
    placement: 'end',
    variant: 'filled',
    shape: 'corner',
  },
}

const bubbleItems = computed<BubbleItemType[]>(() => {
  const streamingId = streamingMessageId.value
  return messages.value.map((message) => {
    const isStreaming = pending.value && message.id === streamingId
    return {
      key: message.id,
      role: message.role === 'assistant' ? 'ai' : 'user',
      content: message.text,
      streaming: isStreaming,
      loading: isStreaming && !message.text,
      footer:
        message.role === 'assistant' && message.text
          ? h(Actions, {
              items: [
                {
                  key: 'copy',
                  label: '复制',
                  actionRender: () =>
                    h(
                      'button',
                      {
                        class: 'bubble-copy',
                        type: 'button',
                        onClick: () => void copyText(message.text),
                      },
                      '复制',
                    ),
                },
              ],
            })
          : undefined,
    }
  })
})

const promptItems: PromptProps[] = suggestions.map((text, index) => ({
  key: `prompt-${index}`,
  label: text,
}))

const welcomeIcon = h('span', { class: 'welcome-orb' }, 'P')

const conversationItems = computed(() =>
  conversations.value.map((conversation) => ({
    key: conversation.id,
    label: conversation.title || '新对话',
    group: isToday(conversation.updatedAt) ? '今天' : '更早',
  })),
)

const creation = {
  label: '新对话',
  onClick: () => newConversation(),
}

function isToday(timestamp: number): boolean {
  return new Date(timestamp).toDateString() === new Date().toDateString()
}

function conversationMenu(conversation: ConversationItemType): MenuProps {
  return {
    items: [{ key: 'delete', label: '删除对话', danger: true }],
    onClick: (info) => {
      if (info.key === 'delete') void removeConversation(String(conversation.key))
    },
  }
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

function fromStored(message: StoredMessage): Message {
  return {
    id: `h-${message.id}`,
    role: message.role === 'assistant' ? 'assistant' : 'user',
    text: message.content,
  }
}

function findMessage(id: string): Message | undefined {
  return messages.value.find((message) => message.id === id)
}

async function refreshConversations() {
  try {
    conversations.value = await bridge.listConversations()
  } catch (error) {
    setError(error)
  }
}

async function openConversation(id: string) {
  const epoch = ++viewEpoch
  activeConversationId.value = id
  errorText.value = ''
  try {
    const stored = await bridge.getMessages(id)
    if (epoch !== viewEpoch) return
    messages.value = stored.map(fromStored)
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
  input.value = ''
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

function onPromptClick({ data }: { data: PromptProps }) {
  if (typeof data.label === 'string') input.value = data.label
}

function onSenderChange(value: string) {
  input.value = value
}

async function send(textInput?: string) {
  const text = (typeof textInput === 'string' ? textInput : input.value).trim()
  if (!text || pending.value) return

  input.value = ''
  errorText.value = ''
  pending.value = true
  const expectedActive = activeConversationId.value
  const epoch = viewEpoch
  messages.value.push({ id: `local-${Date.now()}`, role: 'user', text })

  try {
    const result = await bridge.sendChat(text, expectedActive)
    pendingMessageId = result.messageId
    if (epoch !== viewEpoch) {
      await refreshConversations()
      return
    }
    activeConversationId.value = result.conversationId
    messages.value.push({ id: result.messageId, role: 'assistant', text: '' })
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

  await refreshConversations()
  const mostRecent = conversations.value[0]
  if (mostRecent) await openConversation(mostRecent.id)
})

onBeforeUnmount(() => {
  window.clearTimeout(toastTimer)
  for (const unsubscribe of unsubscribers) unsubscribe()
})
</script>

<template>
  <XProvider :theme="themeConfig">
    <div class="chat-app">
      <header class="chat-header">
        <div class="chat-identity">
          <span class="chat-avatar">P</span>
          <div>
            <h1>Puck</h1>
            <p>住在你桌角的小伙伴</p>
          </div>
        </div>
        <span class="chat-status" :data-busy="pending">{{ pending ? '在想…' : '在线' }}</span>
      </header>

      <div class="chat-body">
        <aside class="history-sidebar">
          <Conversations
            class="history-conversations"
            :items="conversationItems"
            :active-key="activeConversationId"
            :on-active-change="openConversation"
            :menu="conversationMenu"
            :creation="creation"
            :groupable="true"
          />
        </aside>

        <section class="chat-main">
          <main class="message-list">
            <div v-if="messages.length === 0" class="empty-state">
              <Welcome
                :icon="welcomeIcon"
                title="和 Puck 聊点什么吧"
                description="在设置里接入模型后就能真正对话；没有配置时使用本地回声。"
                variant="borderless"
              />
              <Prompts
                title="试试这些"
                :items="promptItems"
                vertical
                :on-item-click="onPromptClick"
              />
            </div>
            <Bubble.List
              v-else
              class="bubble-list"
              :items="bubbleItems"
              :roles="roles"
              :auto-scroll="true"
            />
          </main>

          <p v-if="errorText" class="error-bar">{{ errorText }}</p>

          <footer class="composer">
            <Sender
              :value="input"
              :loading="pending"
              placeholder="跟 Puck 说点什么…"
              :auto-size="{ minRows: 1, maxRows: 6 }"
              :on-change="onSenderChange"
              :on-submit="send"
            />
          </footer>
        </section>
      </div>

      <transition name="toast">
        <div v-if="toast" class="copy-toast">{{ toast }}</div>
      </transition>
    </div>
  </XProvider>
</template>
