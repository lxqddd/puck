<script setup lang="ts">
import { computed, onActivated, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import type { Conversation } from '@puck/contract'
import { CircleAlertIcon, Loader2Icon, SearchIcon, Trash2Icon, XIcon } from '@lucide/vue'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
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
import { Input } from '@/components/ui/input'
import { ScrollArea } from '@/components/ui/scroll-area'
import { bridge } from '../bridge'
import PuckAvatar from '../shared/PuckAvatar.vue'

const router = useRouter()
const conversations = ref<Conversation[]>([])
const loading = ref(true)
const errorText = ref('')
const query = ref('')
const deleteDialogOpen = ref(false)
const deleteTarget = ref<Conversation | null>(null)
let activatedOnce = false

async function load() {
  loading.value = true
  errorText.value = ''
  try {
    conversations.value = await bridge.listHistory(500)
  } catch (error) {
    conversations.value = []
    errorText.value = error instanceof Error ? error.message : String(error)
  } finally {
    loading.value = false
  }
}

function timeLabel(timestamp: number): string {
  return new Intl.DateTimeFormat('zh-CN', {
    month: 'numeric',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(timestamp))
}

const groups = computed(() => {
  const keyword = query.value.trim().toLowerCase()
  const filtered = keyword
    ? conversations.value.filter((conversation) =>
        (conversation.title || '新对话').toLowerCase().includes(keyword),
      )
    : conversations.value

  const now = new Date()
  const startOfToday = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime()
  const day = 24 * 60 * 60 * 1000
  const todayBucket = { label: '今天', items: [] as Conversation[] }
  const yesterdayBucket = { label: '昨天', items: [] as Conversation[] }
  const weekBucket = { label: '最近 7 天', items: [] as Conversation[] }
  const olderBucket = { label: '更早', items: [] as Conversation[] }

  for (const conversation of filtered) {
    if (conversation.updatedAt >= startOfToday) todayBucket.items.push(conversation)
    else if (conversation.updatedAt >= startOfToday - day) yesterdayBucket.items.push(conversation)
    else if (conversation.updatedAt >= startOfToday - 7 * day) weekBucket.items.push(conversation)
    else olderBucket.items.push(conversation)
  }

  return [todayBucket, yesterdayBucket, weekBucket, olderBucket].filter(
    (bucket) => bucket.items.length > 0,
  )
})

function openConversation(id: string) {
  void router.push({ name: 'chat', params: { id } })
}

function goChat() {
  void router.push({ name: 'chat' })
}

function requestDelete(conversation: Conversation) {
  deleteTarget.value = conversation
  deleteDialogOpen.value = true
}

async function removeConversation(id: string) {
  try {
    await bridge.deleteConversation(id)
    await load()
  } catch (error) {
    errorText.value = error instanceof Error ? error.message : String(error)
  }
}

async function confirmDelete() {
  const target = deleteTarget.value
  deleteDialogOpen.value = false
  deleteTarget.value = null
  if (target) await removeConversation(target.id)
}

onMounted(load)
onActivated(() => {
  if (!activatedOnce) {
    activatedOnce = true
    return
  }
  void load()
})
</script>

<template>
  <div class="flex h-full w-full flex-col">
    <header
      class="history-header flex items-center justify-between gap-4 border-b border-border bg-card px-5 py-2.5 [-webkit-app-region:drag]"
    >
      <div class="min-w-0">
        <h1 class="history-title">历史</h1>
        <p class="history-subtitle">所有对话都在这里</p>
      </div>
      <div class="history-search w-[260px] flex-none [-webkit-app-region:no-drag]">
        <SearchIcon class="history-search-icon" />
        <Input
          v-model="query"
          type="search"
          placeholder="搜索对话"
          autocomplete="off"
          aria-label="搜索对话"
          class="pl-9 pr-9"
        />
        <Button
          v-if="query"
          type="button"
          variant="ghost"
          size="icon-xs"
          class="history-search-clear"
          aria-label="清空搜索"
          @click="query = ''"
        >
          <XIcon class="size-3.5" />
        </Button>
      </div>
    </header>

    <ScrollArea class="min-h-0 flex-1">
      <div class="history-scroll-content">
        <div v-if="loading" class="grid h-48 place-items-center">
          <Loader2Icon class="size-5 animate-spin text-muted-foreground" />
        </div>

        <div v-else-if="errorText" class="history-state">
          <Alert variant="destructive">
            <CircleAlertIcon class="size-4" />
            <AlertTitle>加载失败</AlertTitle>
            <AlertDescription>{{ errorText }}</AlertDescription>
          </Alert>
          <Button type="button" variant="outline" size="sm" @click="load">重试</Button>
        </div>

        <div v-else-if="groups.length === 0" class="history-empty">
          <PuckAvatar class="size-14 rounded-full opacity-80" />
          <p class="history-empty-title">
            {{ query ? '没有找到匹配的对话' : '还没有历史对话' }}
          </p>
          <p class="history-empty-desc">
            {{ query ? '换个关键词试试。' : '和 Puck 聊过之后，对话会出现在这里。' }}
          </p>
          <Button
            v-if="!query"
            type="button"
            variant="outline"
            size="sm"
            class="mt-2"
            @click="goChat"
          >
            去聊天
          </Button>
        </div>

        <div v-else class="history-groups">
          <section v-for="group in groups" :key="group.label" class="history-group">
            <p class="history-group-title">{{ group.label }}</p>
            <div class="history-list">
              <div v-for="conversation in group.items" :key="conversation.id" class="history-item">
                <button
                  type="button"
                  class="history-open"
                  @click="openConversation(conversation.id)"
                >
                  <span class="history-name">{{ conversation.title || '新对话' }}</span>
                  <span class="history-time">{{ timeLabel(conversation.updatedAt) }}</span>
                </button>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-sm"
                  class="history-delete"
                  aria-label="删除对话"
                  @click.stop="requestDelete(conversation)"
                >
                  <Trash2Icon class="size-3.5" />
                </Button>
              </div>
            </div>
          </section>
        </div>
      </div>
    </ScrollArea>

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
