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
import { theme as antdTheme } from 'ant-design-vue'
import { bridge } from '../bridge'
import { persona } from '../character'
import PuckAvatar from '../shared/PuckAvatar.vue'
import { useTheme } from '../shared/theme'

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

const { isDark } = useTheme()

let pendingMessageId = ''
let viewEpoch = 0
let toastTimer: number | undefined

const streamingMessageId = computed(() => {
  if (!pending.value) return ''
  const last = messages.value[messages.value.length - 1]
  return last && last.role === 'assistant' ? last.id : ''
})

const themeConfig = computed(() => ({
  algorithm: isDark.value ? antdTheme.darkAlgorithm : antdTheme.defaultAlgorithm,
  token: {
    colorPrimary: isDark.value ? '#5fd8b4' : '#35bd98',
    colorText: isDark.value ? '#f7f2e9' : '#17342d',
    colorTextSecondary: isDark.value ? '#b9ab9b' : '#6b7f79',
    colorTextDescription: isDark.value ? '#b9ab9b' : '#6b7f79',
    colorTextPlaceholder: isDark.value ? '#b9ab9b' : '#6b7f79',
    colorBgLayout: isDark.value ? '#1a1511' : '#fbf7ef',
    colorBgContainer: isDark.value ? '#2d261f' : '#ffffff',
    colorBgElevated: isDark.value ? '#362e25' : '#ffffff',
    colorBorder: isDark.value ? 'rgba(247, 242, 233, 0.12)' : 'rgba(23, 52, 45, 0.12)',
    colorBorderSecondary: isDark.value
      ? 'rgba(247, 242, 233, 0.08)'
      : 'rgba(23, 52, 45, 0.08)',
    colorFillSecondary: isDark.value
      ? 'rgba(247, 242, 233, 0.06)'
      : 'rgba(23, 52, 45, 0.06)',
    colorFillTertiary: isDark.value
      ? 'rgba(247, 242, 233, 0.04)'
      : 'rgba(23, 52, 45, 0.04)',
    borderRadius: 12,
    fontFamily:
      "'SF Pro Rounded', 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', system-ui, sans-serif",
  },
}))

function openSettings() {
  void bridge.openSettings()
}

const puckAvatar = h(PuckAvatar, { class: 'h-[30px] w-[30px] rounded-full' })

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
                          'rounded-md px-1.5 py-0.5 text-xs text-puck-ink transition-colors hover:bg-puck-mint/12 hover:text-puck-mint-deep',
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

const welcomeIcon = h(PuckAvatar, {
  class: 'h-16 w-16 rounded-full shadow-[0_12px_28px_rgba(53,189,152,0.35)]',
})

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

function onProviderSelect(event: Event) {
  selectedProviderId.value = (event.target as HTMLSelectElement).value
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
        class="chat-header flex items-center justify-between gap-4 border-b border-puck-line px-6 pb-2 pl-[88px] pt-2 [-webkit-app-region:drag]"
      >
        <div class="flex items-center gap-3">
          <PuckAvatar
            class="h-8 w-8 rounded-full shadow-[0_4px_10px_rgba(53,189,152,0.35)]"
          />
          <div class="flex flex-col">
            <h1 class="m-0 text-[15px] font-bold leading-5 tracking-[0.2px]">Puck</h1>
            <p class="mb-0 mt-0 text-[11px] leading-4 text-puck-muted">住在你桌角的小伙伴</p>
          </div>
        </div>
      </header>

      <div class="flex min-h-0 flex-1">
        <aside
          class="chat-sidebar flex min-h-0 w-[210px] flex-none flex-col overflow-hidden border-r border-puck-line bg-white/45 p-2.5"
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
          <div class="mt-2 border-t border-puck-line pt-2">
            <button
              type="button"
              class="flex w-full items-center gap-2.5 rounded-[10px] border-none bg-transparent px-2.5 py-2 text-left text-[13px] font-medium text-puck-muted transition-colors hover:bg-puck-mint/10 hover:text-puck-ink"
              title="设置"
              aria-label="设置"
              @click="openSettings"
            >
              <svg
                class="h-4 w-4 flex-none"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="1.8"
                stroke-linecap="round"
                stroke-linejoin="round"
              >
                <circle cx="12" cy="12" r="3" />
                <path
                  d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09a1.65 1.65 0 0 0-1.08-1.51 1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09a1.65 1.65 0 0 0 1.51-1.08 1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"
                />
              </svg>
              <span>设置</span>
            </button>
          </div>
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

          <p
            v-if="errorText"
            class="m-0 bg-puck-danger-soft px-6 py-2.5 text-xs text-puck-danger"
          >
            {{ errorText }}
          </p>

          <footer
            class="composer w-full border-t border-puck-line bg-white/42 px-6 pb-[18px] pt-3.5"
          >
            <Sender
              :value="input"
              :loading="pending"
              placeholder="跟 Puck 说点什么…"
              :auto-size="{ minRows: 3, maxRows: 8 }"
              :actions="false"
              :on-change="onSenderChange"
              :on-submit="send"
            >
              <template #footer>
                <div class="flex items-center justify-between gap-3">
                  <select
                    v-if="providers.length"
                    class="h-8 max-w-[220px] cursor-pointer rounded-lg bg-puck-mint/12 px-2 text-xs font-semibold text-puck-mint-deep outline-none hover:bg-puck-mint/20"
                    :value="selectedProviderId"
                    title="切换当前对话使用的模型"
                    @change="onProviderSelect"
                  >
                    <option
                      v-for="provider in providers"
                      :key="provider.id"
                      :value="provider.id"
                    >
                      {{ providerLabel(provider) }}
                    </option>
                  </select>
                  <span v-else class="text-xs text-puck-muted">未配置模型</span>
                  <button
                    type="button"
                    class="sender-send-btn"
                    :class="{ 'sender-send-btn-loading': pending }"
                    :disabled="pending || !input.trim()"
                    :title="pending ? '正在回复' : '发送'"
                    @click="send()"
                  >
                    <svg
                      v-if="pending"
                      class="h-5 w-5 animate-spin"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      stroke-width="2"
                    >
                      <circle class="opacity-25" cx="12" cy="12" r="9" />
                      <path class="opacity-90" d="M21 12a9 9 0 0 0-9-9" />
                    </svg>
                    <svg
                      v-else
                      class="h-5 w-5"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      stroke-width="2.2"
                      stroke-linecap="round"
                      stroke-linejoin="round"
                    >
                      <path d="M12 19V5" />
                      <path d="m5 12 7-7 7 7" />
                    </svg>
                  </button>
                </div>
              </template>
            </Sender>
          </footer>
        </section>
      </div>

      <transition name="toast">
        <div
          v-if="toast"
          class="pointer-events-none fixed bottom-[98px] left-1/2 z-20 -translate-x-1/2 rounded-full bg-puck-ink px-3.5 py-2 text-xs text-puck-cream"
        >
          {{ toast }}
        </div>
      </transition>
    </div>
  </XProvider>
</template>
