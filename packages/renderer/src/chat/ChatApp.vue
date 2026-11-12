<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import type { Conversation, StoredMessage } from '@puck/contract'
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
const listEl = ref<HTMLElement | null>(null)
const composerEl = ref<HTMLTextAreaElement | null>(null)
const unsubscribers: Array<() => void> = []

const suggestions = ['帮我想个周末计划', '给我讲个冷笑话', '解释一下什么是 RAG']

let pendingMessageId = ''
let viewEpoch = 0

const streamingMessageId = computed(() => {
  if (!pending.value) return ''
  const last = messages.value[messages.value.length - 1]
  return last && last.role === 'assistant' ? last.id : ''
})

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

async function scrollToBottom() {
  await nextTick()
  const el = listEl.value
  if (el) el.scrollTop = el.scrollHeight
}

async function refreshConversations() {
  try {
    conversations.value = await bridge.listConversations()
  } catch (error) {
    errorText.value = error instanceof Error ? error.message : String(error)
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
    await scrollToBottom()
  } catch (error) {
    if (epoch === viewEpoch) {
      errorText.value = error instanceof Error ? error.message : String(error)
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
    errorText.value = error instanceof Error ? error.message : String(error)
  }
}

function useSuggestion(text: string) {
  input.value = text
  autoResize()
}

function autoResize() {
  const el = composerEl.value
  if (!el) return
  el.style.height = 'auto'
  el.style.height = `${Math.min(el.scrollHeight, 140)}px`
}

function resetComposerHeight() {
  const el = composerEl.value
  if (!el) return
  el.style.height = '42px'
}

async function send() {
  const text = input.value.trim()
  if (!text || pending.value) return

  input.value = ''
  errorText.value = ''
  pending.value = true
  await nextTick()
  resetComposerHeight()
  const expectedActive = activeConversationId.value
  const epoch = viewEpoch
  messages.value.push({ id: `local-${Date.now()}`, role: 'user', text })
  await scrollToBottom()

  try {
    const result = await bridge.sendChat(text, expectedActive)
    pendingMessageId = result.messageId
    if (epoch !== viewEpoch) {
      await refreshConversations()
      return
    }
    activeConversationId.value = result.conversationId
    messages.value.push({ id: result.messageId, role: 'assistant', text: '' })
    await scrollToBottom()
    await refreshConversations()
  } catch (error) {
    pending.value = false
    pendingMessageId = ''
    if (epoch === viewEpoch) {
      errorText.value = error instanceof Error ? error.message : String(error)
    }
  }
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Enter' && !event.shiftKey) {
    event.preventDefault()
    void send()
  }
}

onMounted(async () => {
  void bridge.setPersona(persona.systemPrompt)
  unsubscribers.push(
    bridge.onChatDelta((event) => {
      if (event.conversationId !== activeConversationId.value) return
      const message = findMessage(event.messageId)
      if (!message) return
      message.text += event.text
      void scrollToBottom()
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
        // 流式完成时用户已切回该会话：从历史重新加载，拿到刚落库的回复。
        void openConversation(event.conversationId)
        void refreshConversations()
        return
      }
      if (event.text) message.text = event.text
      void refreshConversations()
      void scrollToBottom()
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
  for (const unsubscribe of unsubscribers) unsubscribe()
})
</script>

<template>
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
        <div class="history-head">
          <span>对话</span>
          <button class="new-chat" @click="newConversation">＋ 新对话</button>
        </div>
        <nav class="history-list">
          <button
            v-for="conversation in conversations"
            :key="conversation.id"
            class="history-item"
            :class="{ active: conversation.id === activeConversationId }"
            @click="openConversation(conversation.id)"
          >
            <span class="history-title">{{ conversation.title || '新对话' }}</span>
            <span
              class="history-delete"
              title="删除对话"
              @click.stop="removeConversation(conversation.id)"
              >×</span
            >
          </button>
          <p v-if="conversations.length === 0" class="history-empty">还没有历史对话</p>
        </nav>
      </aside>

      <section class="chat-main">
        <main ref="listEl" class="message-list">
          <div v-if="messages.length === 0" class="empty-state">
            <div class="empty-orb">P</div>
            <h2>和 Puck 聊点什么吧</h2>
            <p>在设置里接入模型后就能真正对话；没有配置时使用本地回声。</p>
            <div class="suggestions">
              <button
                v-for="suggestion in suggestions"
                :key="suggestion"
                @click="useSuggestion(suggestion)"
              >
                {{ suggestion }}
              </button>
            </div>
          </div>

          <div v-for="message in messages" :key="message.id" class="message" :class="message.role">
            <div v-if="message.role === 'assistant'" class="message-avatar">P</div>
            <div class="bubble">
              {{ message.text
              }}<span v-if="streamingMessageId === message.id && message.text" class="caret"></span>
            </div>
          </div>

          <div v-if="pending && !streamingMessageId" class="message assistant">
            <div class="message-avatar">P</div>
            <div class="bubble typing"><i></i><i></i><i></i></div>
          </div>
        </main>

        <p v-if="errorText" class="error-bar">{{ errorText }}</p>

        <footer class="composer">
          <textarea
            ref="composerEl"
            v-model="input"
            rows="1"
            title="Enter 发送，Shift+Enter 换行"
            placeholder="跟 Puck 说点什么…"
            @input="autoResize"
            @keydown="onKeydown"
          ></textarea>
          <button class="send" :disabled="pending || !input.trim()" @click="send">发送</button>
        </footer>
      </section>
    </div>
  </div>
</template>
