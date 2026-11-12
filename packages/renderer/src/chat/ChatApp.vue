<script setup lang="ts">
import { computed, h, onBeforeUnmount, onMounted, ref } from 'vue'
import type { Conversation, ProviderConfig, StoredMessage } from '@puck/contract'
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
const providers = ref<ProviderConfig[]>([])
const activeProviderId = ref('')
const selectedProviderId = ref('')
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

const puckAvatar = h(
  'span',
  {
    class:
      'grid h-[30px] w-[30px] place-items-center rounded-full bg-gradient-to-br from-[#86eccb] to-[#35bd98] text-[13px] font-bold text-white',
  },
  'P',
)

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
                        class:
                          'rounded-md px-1.5 py-0.5 text-xs text-puck-muted transition-colors hover:bg-puck-mint/12 hover:text-puck-mint-deep',
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

const welcomeIcon = h(
  'span',
  {
    class:
      'grid h-16 w-16 place-items-center rounded-[22px] bg-gradient-to-br from-[#86eccb] to-[#35bd98] text-[28px] font-bold text-white shadow-[0_12px_28px_rgba(53,189,152,0.35)]',
  },
  'P',
)

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

function providerLabel(provider: ProviderConfig): string {
  const model = provider.model?.trim()
  const name = provider.name?.trim() || provider.type || '模型'
  return model ? `${name} · ${model}` : name
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

const modelPicker = computed(() => {
  if (providers.value.length === 0) return false
  return h(
    'select',
    {
      class:
        'h-7 max-w-[200px] cursor-pointer rounded-lg bg-puck-mint/12 px-1.5 text-xs font-semibold text-puck-mint-deep outline-none hover:bg-puck-mint/20',
      value: selectedProviderId.value,
      title: '切换当前对话使用的模型',
      onChange: (event: Event) => {
        selectedProviderId.value = (event.target as HTMLSelectElement).value
      },
    },
    providers.value.map((provider) =>
      h('option', { value: provider.id }, providerLabel(provider)),
    ),
  )
})

async function openConversation(id: string) {
  const epoch = ++viewEpoch
  activeConversationId.value = id
  errorText.value = ''
  try {
    const stored = await bridge.getMessages(id)
    if (epoch !== viewEpoch) return
    messages.value = stored.map(fromStored)
    applyConversationProvider(id)
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
  selectedProviderId.value = activeProviderId.value || providers.value[0]?.id || ''
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
    const result = await bridge.sendChat(text, expectedActive, selectedProviderId.value)
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

  await Promise.all([loadProviders(), refreshConversations()])
  const mostRecent = conversations.value[0]
  if (mostRecent) await openConversation(mostRecent.id)

  window.addEventListener('focus', onWindowFocus)
})

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
  <XProvider :theme="themeConfig">
    <div class="flex h-full w-full flex-col">
      <header
        class="flex items-center justify-between gap-4 border-b border-puck-line px-6 pb-4 pl-[88px] pt-[18px] [-webkit-app-region:drag]"
      >
        <div class="flex items-center gap-3">
          <span
            class="grid h-[38px] w-[38px] place-items-center rounded-full bg-gradient-to-br from-[#86eccb] to-[#35bd98] text-[17px] font-bold text-white shadow-[0_4px_10px_rgba(53,189,152,0.35)]"
            >P</span
          >
          <div>
            <h1 class="m-0 text-base font-bold tracking-[0.2px]">Puck</h1>
            <p class="mb-0 mt-0.5 text-xs text-puck-muted">住在你桌角的小伙伴</p>
          </div>
        </div>
        <span
          class="rounded-full px-2.5 py-[5px] text-xs font-semibold"
          :class="
            pending ? 'bg-[#ffd166]/25 text-[#8a6414]' : 'bg-puck-mint/12 text-puck-mint-deep'
          "
          >{{ pending ? '在想…' : '在线' }}</span
        >
      </header>

      <div class="flex min-h-0 flex-1">
        <aside
          class="flex min-h-0 w-[210px] flex-none flex-col overflow-hidden border-r border-puck-line bg-white/45 p-2.5"
        >
          <Conversations
            class="history-conversations min-h-0 flex-1"
            :items="conversationItems"
            :active-key="activeConversationId"
            :on-active-change="openConversation"
            :menu="conversationMenu"
            :creation="creation"
            :groupable="true"
          />
        </aside>

        <section class="flex min-w-0 flex-1 flex-col">
          <main class="flex min-h-0 flex-1 flex-col">
            <div
              v-if="messages.length === 0"
              class="flex flex-1 flex-col items-center justify-center gap-[18px] overflow-y-auto px-6 py-8 text-center [&_.ant-prompts]:w-full [&_.ant-prompts]:max-w-[420px] [&_.ant-prompts]:text-left [&_.ant-welcome]:w-full [&_.ant-welcome]:max-w-[420px]"
            >
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
              class="bubble-list min-h-0 flex-1"
              :items="bubbleItems"
              :roles="roles"
              :auto-scroll="true"
            />
          </main>

          <p v-if="errorText" class="m-0 bg-[#ff8f8f]/18 px-6 py-2.5 text-xs text-[#9a3535]">
            {{ errorText }}
          </p>

          <footer class="composer w-full border-t border-puck-line bg-white/42 px-6 pb-[18px] pt-3.5">
            <Sender
              :value="input"
              :loading="pending"
              placeholder="跟 Puck 说点什么…"
              :auto-size="{ minRows: 4, maxRows: 8 }"
              :prefix="modelPicker"
              :on-change="onSenderChange"
              :on-submit="send"
            />
          </footer>
        </section>
      </div>

      <transition name="toast">
        <div
          v-if="toast"
          class="pointer-events-none fixed bottom-[98px] left-1/2 z-20 -translate-x-1/2 rounded-full bg-[#17342d]/86 px-3.5 py-2 text-xs text-white"
        >
          {{ toast }}
        </div>
      </transition>
    </div>
  </XProvider>
</template>
