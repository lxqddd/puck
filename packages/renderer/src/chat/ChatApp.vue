<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { bridge } from '../bridge'
import { persona } from '../character'

interface Message {
  id: string
  role: 'user' | 'assistant'
  text: string
}

const messages = ref<Message[]>([])
const input = ref('')
const pending = ref(false)
const errorText = ref('')
const listEl = ref<HTMLElement | null>(null)
const unsubscribers: Array<() => void> = []

const suggestions = ['帮我想个周末计划', '给我讲个冷笑话', '解释一下什么是 RAG']

const streamingMessageId = computed(() => {
  if (!pending.value) return ''
  const last = messages.value[messages.value.length - 1]
  return last && last.role === 'assistant' ? last.id : ''
})

function findMessage(id: string): Message | undefined {
  return messages.value.find((message) => message.id === id)
}

async function scrollToBottom() {
  await nextTick()
  const el = listEl.value
  if (el) el.scrollTop = el.scrollHeight
}

function useSuggestion(text: string) {
  input.value = text
}

async function send() {
  const text = input.value.trim()
  if (!text || pending.value) return

  input.value = ''
  errorText.value = ''
  pending.value = true
  messages.value.push({
    id: `local-${Date.now()}-${messages.value.length}`,
    role: 'user',
    text,
  })
  await scrollToBottom()

  try {
    const messageId = await bridge.sendChat(text)
    messages.value.push({ id: messageId, role: 'assistant', text: '' })
    await scrollToBottom()
  } catch (error) {
    pending.value = false
    errorText.value = error instanceof Error ? error.message : String(error)
  }
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Enter' && !event.shiftKey) {
    event.preventDefault()
    void send()
  }
}

onMounted(() => {
  void bridge.setPersona(persona.systemPrompt)
  unsubscribers.push(
    bridge.onChatDelta((event) => {
      const message = findMessage(event.messageId)
      if (!message) return
      message.text += event.text
      void scrollToBottom()
    }),
    bridge.onChatDone((event) => {
      const message = findMessage(event.messageId)
      if (message && event.text) message.text = event.text
      pending.value = false
      void scrollToBottom()
    }),
    bridge.onChatError((event) => {
      pending.value = false
      errorText.value = event.message
    }),
  )
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

    <main ref="listEl" class="message-list">
      <div v-if="messages.length === 0" class="empty-state">
        <div class="empty-orb">P</div>
        <h2>和 Puck 聊点什么吧</h2>
        <p>现在是本地回声模式；在设置里接入模型后就能真正对话。</p>
        <div class="suggestions">
          <button v-for="suggestion in suggestions" :key="suggestion" @click="useSuggestion(suggestion)">
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
        v-model="input"
        rows="1"
        placeholder="跟 Puck 说点什么…（Enter 发送，Shift+Enter 换行）"
        @keydown="onKeydown"
      ></textarea>
      <button class="send" :disabled="pending || !input.trim()" @click="send">发送</button>
    </footer>
  </div>
</template>
