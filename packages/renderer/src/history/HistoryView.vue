<script setup lang="ts">
import { computed, h, onActivated, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  Alert,
  Button,
  Empty,
  Input,
  List,
  Popconfirm,
  Spin,
  Typography,
} from 'ant-design-vue'
import type { Conversation } from '@puck/contract'
import { bridge } from '../bridge'
import PuckAvatar from '../shared/PuckAvatar.vue'

const router = useRouter()
const conversations = ref<Conversation[]>([])
const loading = ref(true)
const errorText = ref('')
const query = ref('')
const emptyImage = h(PuckAvatar, { class: 'mx-auto h-14 w-14 rounded-full opacity-70' })
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

async function removeConversation(id: string) {
  try {
    await bridge.deleteConversation(id)
    await load()
  } catch (error) {
    errorText.value = error instanceof Error ? error.message : String(error)
  }
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
      class="history-header flex items-center justify-between gap-4 border-b border-puck-line px-6 pb-3 pt-3 [-webkit-app-region:drag]"
    >
      <div class="min-w-0">
        <Typography.Title :level="4" class="history-title">历史</Typography.Title>
        <Typography.Text type="secondary" class="history-subtitle">
          所有对话都在这里
        </Typography.Text>
      </div>
      <div class="w-[260px] flex-none [-webkit-app-region:no-drag]">
        <Input v-model:value="query" allow-clear placeholder="搜索对话" />
      </div>
    </header>

    <div class="min-h-0 flex-1 overflow-y-auto px-6 py-5">
      <div v-if="loading" class="grid h-full place-items-center">
        <Spin size="large" />
      </div>
      <Alert
        v-else-if="errorText"
        class="mx-auto max-w-[760px]"
        type="error"
        show-icon
        :message="errorText"
      />
      <Empty
        v-else-if="groups.length === 0"
        class="mt-10"
        :image="emptyImage"
        :description="query ? '没有找到匹配的对话' : '还没有历史对话'"
      />
      <div v-else class="mx-auto flex max-w-[760px] flex-col gap-6">
        <section v-for="group in groups" :key="group.label">
          <Typography.Text type="secondary" class="mb-2 block text-xs font-semibold">
            {{ group.label }}
          </Typography.Text>
          <List
            :data-source="group.items"
            :split="false"
            :bordered="false"
            row-key="id"
            class="history-list"
          >
            <template #renderItem="{ item }">
              <List.Item class="history-item">
                <button
                  type="button"
                  class="history-open"
                  @click="openConversation(item.id)"
                >
                  <span class="block truncate text-sm font-medium text-puck-ink">
                    {{ item.title || '新对话' }}
                  </span>
                  <span class="mt-1 block text-xs text-puck-muted">
                    {{ timeLabel(item.updatedAt) }}
                  </span>
                </button>
                <Popconfirm
                  title="删除这个对话？"
                  ok-text="删除"
                  cancel-text="取消"
                  @confirm="removeConversation(item.id)"
                >
                  <Button danger type="text" size="small" @click.stop>删除</Button>
                </Popconfirm>
              </List.Item>
            </template>
          </List>
        </section>
      </div>
    </div>
  </div>
</template>
