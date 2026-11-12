<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import type { ProviderConfig, Settings } from '@puck/contract'
import { bridge } from '../bridge'

interface Draft {
  id: string
  name: string
  type: string
  baseURL: string
  model: string
  hasKey: boolean
}

function emptyDraft(): Draft {
  return {
    id: '',
    name: '',
    type: 'openai-compatible',
    baseURL: '',
    model: '',
    hasKey: false,
  }
}

const settings = ref<Settings>({ providers: [], activeProviderId: '', mascotScale: 1 })
const draft = ref<Draft>(emptyDraft())
const apiKey = ref('')
const loading = ref(true)
const saving = ref(false)
const saved = ref(false)
const errorText = ref('')

const providers = computed(() => settings.value.providers ?? [])
const isExisting = computed(() => providers.value.some((provider) => provider.id === draft.value.id))

const DEFAULT_MODELS: Record<string, string> = {
  'openai-compatible': 'deepseek-chat',
  anthropic: 'claude-opus-5-5',
  gemini: 'gemini-3.8-flash',
}

const modelPlaceholder = computed(() => DEFAULT_MODELS[draft.value.type] ?? 'deepseek-chat')

function typeLabel(type: string): string {
  if (type === 'anthropic') return 'Anthropic'
  if (type === 'gemini') return 'Gemini'
  return 'OpenAI 兼容'
}

function edit(provider: ProviderConfig) {
  draft.value = {
    id: provider.id,
    name: provider.name,
    type: provider.type,
    baseURL: provider.baseURL,
    model: provider.model,
    hasKey: provider.hasKey,
  }
  apiKey.value = ''
  errorText.value = ''
}

function addProvider() {
  draft.value = emptyDraft()
  apiKey.value = ''
  errorText.value = ''
}

function onTypeChange() {
  const knownModels = Object.values(DEFAULT_MODELS)
  if (!draft.value.model.trim() || knownModels.includes(draft.value.model.trim())) {
    draft.value.model = modelPlaceholder.value
  }
  if (draft.value.type !== 'openai-compatible' && draft.value.baseURL === 'https://api.deepseek.com/v1') {
    draft.value.baseURL = ''
  }
  if (draft.value.type === 'openai-compatible' && !draft.value.baseURL.trim()) {
    draft.value.baseURL = 'https://api.deepseek.com/v1'
  }
}

async function load() {
  settings.value = await bridge.getSettings()
  const active =
    providers.value.find((provider) => provider.id === settings.value.activeProviderId) ??
    providers.value[0]
  if (active) {
    edit(active)
  } else {
    addProvider()
  }
}

async function save() {
  if (!draft.value.model.trim()) {
    errorText.value = '请填写模型'
    return
  }
  if (draft.value.type === 'openai-compatible' && !draft.value.baseURL.trim()) {
    errorText.value = '请填写服务地址'
    return
  }

  saving.value = true
  errorText.value = ''
  saved.value = false
  try {
    const provider: ProviderConfig = {
      id: draft.value.id,
      name: draft.value.name.trim() || draft.value.model.trim() || '未命名供应商',
      type: draft.value.type,
      baseURL: draft.value.baseURL.trim(),
      model: draft.value.model.trim(),
      hasKey: draft.value.hasKey,
    }
    const savedProvider = await bridge.saveProvider(provider, apiKey.value)
    apiKey.value = ''
    const fresh = await bridge.getSettings()
    settings.value = fresh
    const target = fresh.providers.find((item) => item.id === savedProvider.id)
    if (target) edit(target)
    saved.value = true
    window.setTimeout(() => {
      saved.value = false
    }, 2400)
  } catch (error) {
    errorText.value = error instanceof Error ? error.message : String(error)
  } finally {
    saving.value = false
  }
}

async function removeProvider(id: string) {
  if (!window.confirm('确定删除这个供应商吗？')) return
  try {
    await bridge.deleteProvider(id)
    await load()
  } catch (error) {
    errorText.value = error instanceof Error ? error.message : String(error)
  }
}

async function activate(id: string) {
  try {
    await bridge.setActiveProvider(id)
    settings.value = await bridge.getSettings()
  } catch (error) {
    errorText.value = error instanceof Error ? error.message : String(error)
  }
}

onMounted(async () => {
  try {
    await load()
  } catch (error) {
    errorText.value = error instanceof Error ? error.message : String(error)
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="settings-app">
    <header class="settings-header">
      <h1>设置</h1>
      <p>管理模型供应商；API Key 保存在系统钥匙串，当前对话使用选中的供应商。</p>
    </header>

    <div v-if="!loading" class="settings-body">
      <aside class="provider-list">
        <div class="provider-list-head">
          <span>供应商</span>
          <button @click="addProvider">＋ 添加</button>
        </div>
        <div class="provider-list-scroll">
          <button
            v-for="provider in providers"
            :key="provider.id"
            class="provider-item"
            :class="{ active: provider.id === draft.id }"
            @click="edit(provider)"
          >
            <span
              class="provider-dot"
              :class="{ on: provider.id === settings.activeProviderId }"
              :title="provider.id === settings.activeProviderId ? '当前使用中' : '设为当前供应商'"
              @click.stop="activate(provider.id)"
            ></span>
            <span class="provider-name">{{ provider.name }}</span>
            <span class="provider-type">{{ typeLabel(provider.type) }}</span>
          </button>
          <p v-if="providers.length === 0" class="provider-empty">还没有供应商</p>
        </div>
      </aside>

      <form class="settings-form" @submit.prevent="save">
        <label>
          <span>名称</span>
          <input v-model="draft.name" placeholder="例如 DeepSeek" autocomplete="off" />
        </label>

        <label>
          <span>类型</span>
          <select v-model="draft.type" @change="onTypeChange">
            <option value="openai-compatible">OpenAI 兼容</option>
            <option value="anthropic">Anthropic</option>
            <option value="gemini">Google Gemini</option>
          </select>
        </label>

        <label v-if="draft.type === 'openai-compatible'">
          <span>服务地址</span>
          <input
            v-model="draft.baseURL"
            placeholder="https://api.deepseek.com/v1"
            autocomplete="off"
          />
        </label>

        <label v-else-if="draft.type === 'anthropic'">
          <span>自定义地址（可选）</span>
          <input v-model="draft.baseURL" placeholder="留空使用 Anthropic 官方 API" autocomplete="off" />
        </label>

        <label>
          <span>模型</span>
          <input v-model="draft.model" :placeholder="modelPlaceholder" autocomplete="off" />
        </label>

        <label>
          <span>API Key <em v-if="draft.hasKey">已保存</em></span>
          <input
            v-model="apiKey"
            type="password"
            :placeholder="draft.hasKey ? '留空保持不变' : 'sk-…'"
            autocomplete="off"
          />
        </label>

        <p class="hint">
          OpenAI 兼容支持 OpenAI、DeepSeek、通义、Kimi、智谱、Ollama 等；Anthropic 与 Gemini 走官方
          SDK。点击左侧圆点切换对话使用的供应商。
        </p>

        <div class="actions">
          <button type="submit" class="primary" :disabled="saving">
            {{ saving ? '保存中…' : isExisting ? '保存供应商' : '添加供应商' }}
          </button>
          <button
            v-if="draft.id"
            type="button"
            class="danger"
            @click="removeProvider(draft.id)"
          >
            删除
          </button>
          <span v-if="saved" class="saved">已保存</span>
        </div>

        <p v-if="errorText" class="error">{{ errorText }}</p>
      </form>
    </div>
  </div>
</template>
