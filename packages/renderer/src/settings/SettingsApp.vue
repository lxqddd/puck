<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import type { ProviderConfig, ProviderTestResult, Settings, ThemeMode } from '@puck/contract'
import { bridge } from '../bridge'
import { useTheme } from '../shared/theme'

interface Draft {
  id: string
  name: string
  type: string
  baseURL: string
  model: string
  hasKey: boolean
  keyMask: string
  webSearch: boolean
}

function emptyDraft(): Draft {
  return {
    id: '',
    name: '',
    type: 'openai-compatible',
    baseURL: '',
    model: '',
    hasKey: false,
    keyMask: '',
    webSearch: true,
  }
}

const settings = ref<Settings>({
  providers: [],
  activeProviderId: '',
  mascotScale: 1,
  theme: 'system',
})
const draft = ref<Draft>(emptyDraft())
const apiKey = ref('')
const loading = ref(true)
const saving = ref(false)
const testing = ref(false)
const saved = ref(false)
const errorText = ref('')
const testResult = ref<ProviderTestResult | null>(null)
const models = ref<string[]>([])
const modelsLoading = ref(false)
const modelsError = ref('')

const providers = computed(() => settings.value.providers ?? [])
const isExisting = computed(() => providers.value.some((provider) => provider.id === draft.value.id))
const keyPending = computed(() => apiKey.value.trim().length > 0)

const DEFAULT_MODELS: Record<string, string> = {
  'openai-compatible': 'deepseek-chat',
  anthropic: 'claude-opus-5-5',
  gemini: 'gemini-3.8-flash',
}

const modelPlaceholder = computed(() => DEFAULT_MODELS[draft.value.type] ?? 'deepseek-chat')

const labelClass = 'flex flex-col gap-[7px] text-[13px] font-semibold text-puck-ink'
const fieldClass =
  'rounded-xl border border-puck-line bg-puck-card px-3.5 py-[11px] text-sm text-puck-ink outline-none placeholder:text-puck-muted focus:border-puck-mint/60 focus:ring-[3px] focus:ring-puck-mint/14 dark:bg-puck-surface'
const primaryClass =
  'rounded-xl border-none bg-gradient-to-br from-[#5bd8b4] to-[#2fa987] px-6 py-[11px] text-sm font-semibold text-[#10241d] hover:brightness-105 disabled:opacity-60'
const secondaryClass =
  'rounded-xl border border-puck-mint/50 bg-transparent px-[18px] py-[11px] text-[13px] font-semibold text-puck-mint-deep hover:bg-puck-mint/10 disabled:opacity-60'
const dangerClass =
  'rounded-xl border border-puck-danger/35 bg-transparent px-[18px] py-[11px] text-[13px] text-puck-danger hover:bg-puck-danger-soft'
const hintClass = 'm-0 text-xs leading-relaxed text-puck-muted'

const { mode: themeMode, setTheme } = useTheme()
const themeOptions: Array<{ value: ThemeMode; label: string }> = [
  { value: 'system', label: '跟随系统' },
  { value: 'light', label: '浅色' },
  { value: 'dark', label: '深色' },
]

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
    keyMask: provider.keyMask ?? '',
    webSearch: provider.webSearch !== false,
  }
  apiKey.value = ''
  errorText.value = ''
  testResult.value = null
  resetModels()
}

function addProvider() {
  draft.value = emptyDraft()
  apiKey.value = ''
  errorText.value = ''
  testResult.value = null
  resetModels()
}

let modelsRequestId = 0

function resetModels() {
  modelsRequestId += 1
  models.value = []
  modelsError.value = ''
  modelsLoading.value = false
}

async function fetchModels() {
  if (modelsLoading.value) return
  if (draft.value.type === 'openai-compatible' && !draft.value.baseURL.trim()) {
    resetModels()
    modelsError.value = '请先填写服务地址'
    return
  }
  const requestId = ++modelsRequestId
  modelsLoading.value = true
  modelsError.value = ''
  try {
    const list = await bridge.listModels(draftProvider(), apiKey.value)
    if (requestId !== modelsRequestId) return
    models.value = list
  } catch (error) {
    if (requestId !== modelsRequestId) return
    models.value = []
    modelsError.value = `未能获取模型列表：${error instanceof Error ? error.message : String(error)}`
  } finally {
    if (requestId === modelsRequestId) modelsLoading.value = false
  }
}

function onApiKeyBlur() {
  if (apiKey.value.trim()) void fetchModels()
}

function onModelSelect(event: Event) {
  draft.value.model = (event.target as HTMLSelectElement).value
}

function onTypeChange() {
  resetModels()
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

function draftProvider(): ProviderConfig {
  return {
    id: draft.value.id,
    name: draft.value.name.trim() || draft.value.model.trim() || '未命名供应商',
    type: draft.value.type,
    baseURL: draft.value.baseURL.trim(),
    model: draft.value.model.trim(),
    hasKey: draft.value.hasKey,
    keyMask: draft.value.keyMask,
    webSearch: draft.value.webSearch,
  }
}

async function testConnection() {
  testing.value = true
  testResult.value = null
  errorText.value = ''
  try {
    testResult.value = await bridge.testProvider(draftProvider(), apiKey.value)
  } catch (error) {
    testResult.value = {
      ok: false,
      message: error instanceof Error ? error.message : String(error),
      latencyMs: 0,
    }
  } finally {
    testing.value = false
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
    const savedProvider = await bridge.saveProvider(draftProvider(), apiKey.value)
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
  <div class="flex h-full flex-col">
    <header
      class="settings-header flex items-start justify-between gap-4 border-b border-puck-line px-6 pb-4 pl-[88px] pt-[18px] [-webkit-app-region:drag]"
    >
      <div>
        <h1 class="m-0 mb-1 text-xl font-bold">设置</h1>
        <p class="m-0 text-xs text-puck-muted">
          管理模型供应商；API Key 保存在系统钥匙串，当前对话使用选中的供应商。
        </p>
      </div>
      <div
        class="flex items-center gap-1 rounded-full border border-puck-line bg-white/60 p-1 text-xs [-webkit-app-region:no-drag] dark:bg-puck-surface-soft"
      >
        <button
          v-for="option in themeOptions"
          :key="option.value"
          type="button"
          class="rounded-full bg-transparent px-2.5 py-1 font-medium transition-colors hover:bg-puck-mint/10"
          :class="
            themeMode === option.value
              ? 'bg-puck-mint/16! text-puck-mint-deep'
              : 'text-puck-muted hover:text-puck-ink'
          "
          @click="setTheme(option.value)"
        >
          {{ option.label }}
        </button>
      </div>
    </header>

    <div v-if="!loading" class="flex min-h-0 flex-1">
      <aside
        class="settings-rail flex min-h-0 w-[210px] flex-none flex-col border-r border-puck-line bg-white/45"
      >
        <div
          class="flex items-center justify-between px-3 pb-2 pt-3.5 text-xs font-semibold text-puck-muted"
        >
          <span>供应商</span>
          <button
            class="rounded-lg border-none bg-transparent px-2 py-1 text-xs font-semibold text-puck-mint-deep hover:bg-puck-mint/14"
            @click="addProvider"
          >
            ＋ 添加
          </button>
        </div>
        <div class="flex flex-1 flex-col gap-0.5 overflow-y-auto px-2 pb-3">
          <button
            v-for="provider in providers"
            :key="provider.id"
            class="flex w-full items-center gap-2 rounded-[10px] border-none bg-transparent px-2.5 py-[9px] text-left text-[13px] text-puck-ink hover:bg-puck-ink/5 dark:hover:bg-puck-surface"
            :class="{ 'bg-puck-mint/16!': provider.id === draft.id }"
            @click="edit(provider)"
          >
            <span
              class="h-2.5 w-2.5 flex-none rounded-full border-[1.5px] border-puck-muted"
              :class="{
                'border-puck-mint bg-puck-mint shadow-[0_0_0_2px_rgba(63,201,165,0.2)]':
                  provider.id === settings.activeProviderId,
              }"
              :title="provider.id === settings.activeProviderId ? '当前使用中' : '设为当前供应商'"
              @click.stop="activate(provider.id)"
            ></span>
            <span class="flex-1 truncate">{{ provider.name }}</span>
            <span class="text-[11px] text-puck-muted">{{ typeLabel(provider.type) }}</span>
          </button>
          <p v-if="providers.length === 0" class="mx-2.5 my-2 text-xs text-puck-muted">
            还没有供应商
          </p>
        </div>
      </aside>

      <form
        class="flex min-w-0 flex-1 flex-col gap-4 overflow-y-auto px-7 pb-7 pt-5"
        @submit.prevent="save"
      >
        <label :class="labelClass">
          <span>名称</span>
          <input
            v-model="draft.name"
            :class="fieldClass"
            placeholder="例如 DeepSeek"
            autocomplete="off"
          />
        </label>

        <label :class="labelClass">
          <span>类型</span>
          <select v-model="draft.type" :class="fieldClass" @change="onTypeChange">
            <option value="openai-compatible">OpenAI 兼容</option>
            <option value="anthropic">Anthropic</option>
            <option value="gemini">Google Gemini</option>
          </select>
        </label>

        <label v-if="draft.type === 'openai-compatible'" :class="labelClass">
          <span>服务地址</span>
          <input
            v-model="draft.baseURL"
            :class="fieldClass"
            placeholder="https://api.deepseek.com/v1"
            autocomplete="off"
            @input="resetModels"
          />
        </label>

        <label v-else :class="labelClass">
          <span>自定义地址（可选）</span>
          <input
            v-model="draft.baseURL"
            :class="fieldClass"
            :placeholder="
              draft.type === 'gemini' ? '留空使用 Gemini 官方 API' : '留空使用 Anthropic 官方 API'
            "
            autocomplete="off"
            @input="resetModels"
          />
        </label>

        <label :class="labelClass">
          <span>模型</span>
          <div class="flex gap-2.5">
            <input
              v-model="draft.model"
              class="min-w-0 flex-1"
              :class="fieldClass"
              :placeholder="modelPlaceholder"
              autocomplete="off"
            />
            <button
              type="button"
              class="flex-none whitespace-nowrap"
              :class="secondaryClass"
              :disabled="modelsLoading"
              @click="fetchModels"
            >
              {{ modelsLoading ? '获取中…' : '获取模型' }}
            </button>
          </div>
        </label>

        <label v-if="models.length > 0" :class="labelClass">
          <span>
            可选模型
            <em
              class="ml-1.5 rounded-full bg-puck-mint/14 px-[7px] py-0.5 text-[11px] font-medium not-italic text-puck-mint-deep"
              >{{ models.length }} 个</em
            >
          </span>
          <select :class="fieldClass" :value="draft.model" @change="onModelSelect">
            <option value="" disabled>从列表选择</option>
            <option v-if="draft.model && !models.includes(draft.model)" :value="draft.model">
              {{ draft.model }}（当前）
            </option>
            <option v-for="model in models" :key="model" :value="model">{{ model }}</option>
          </select>
        </label>
        <p v-else-if="modelsError" :class="hintClass">{{ modelsError }}</p>

        <label
          v-if="draft.type === 'openai-compatible'"
          class="flex flex-row items-center gap-2 text-[13px] font-medium text-puck-ink"
        >
          <input
            v-model="draft.webSearch"
            type="checkbox"
            class="m-0 h-4 w-4 p-0 accent-puck-mint"
          />
          <span>允许联网搜索（模型自动调用）</span>
        </label>
        <p v-if="draft.type === 'openai-compatible' && draft.webSearch" :class="hintClass">
          DeepSeek 模型走 DeepSeek 搜索/抓取；其他兼容模型使用 Tavily keyless（额度较低，被限流时会自动降级为直接回答）。
        </p>

        <label :class="labelClass">
          <span>
            API Key
            <em
              v-if="draft.hasKey"
              class="ml-1.5 rounded-full bg-puck-mint/14 px-[7px] py-0.5 text-[11px] font-medium not-italic text-puck-mint-deep"
              >已保存</em
            >
          </span>
          <input
            v-model="apiKey"
            type="password"
            :class="fieldClass"
            :placeholder="draft.hasKey ? '留空保持不变' : 'sk-…'"
            autocomplete="off"
            @input="resetModels"
            @blur="onApiKeyBlur"
          />
        </label>

        <p v-if="draft.hasKey && draft.keyMask && !keyPending" class="-mt-1.5 text-xs text-puck-muted">
          已保存：<code
            class="rounded-md bg-puck-ink/6 px-[7px] py-0.5 font-mono text-xs text-puck-ink"
            >{{ draft.keyMask }}</code
          >（留空保持不变）
        </p>

        <p :class="hintClass">
          填入 API Key 后会自动获取该供应商的模型列表，也可点“获取模型”手动刷新。OpenAI
          兼容支持 OpenAI、DeepSeek、通义、Kimi、智谱、Ollama 等；Anthropic 与 Gemini 走官方
          SDK。点击左侧圆点切换对话使用的供应商。
        </p>

        <div class="flex items-center gap-3">
          <button type="submit" :class="primaryClass" :disabled="saving">
            {{ saving ? '保存中…' : isExisting ? '保存供应商' : '添加供应商' }}
          </button>
          <button type="button" :class="secondaryClass" :disabled="testing" @click="testConnection">
            {{ testing ? '测试中…' : '测试连接' }}
          </button>
          <button v-if="draft.id" type="button" :class="dangerClass" @click="removeProvider(draft.id)">
            删除
          </button>
          <span v-if="saved" class="text-[13px] text-puck-mint-deep">已保存</span>
          <span v-else-if="keyPending" class="text-[13px] font-semibold text-puck-warning">Key 未保存</span>
        </div>

        <p v-if="keyPending" :class="hintClass">
          API Key 填好后需要点“{{ isExisting ? '保存供应商' : '添加供应商' }}”才会写入钥匙串。
        </p>

        <p
          v-if="testResult"
          class="m-0 rounded-[10px] px-3.5 py-2.5 text-xs break-words"
          :class="
            testResult.ok
              ? 'bg-puck-mint/14 text-puck-mint-deep'
              : 'bg-puck-danger-soft text-puck-danger'
          "
        >
          {{
            testResult.ok
              ? `${testResult.message} · ${testResult.latencyMs} ms`
              : testResult.message
          }}
        </p>

        <p v-if="testResult?.ok && keyPending" :class="hintClass">
          连接成功，但新 Key 尚未保存，请点“保存供应商”。
        </p>

        <p
          v-if="errorText"
          class="m-0 rounded-[10px] bg-puck-danger-soft px-3.5 py-2.5 text-xs text-puck-danger"
        >
          {{ errorText }}
        </p>
      </form>
    </div>
  </div>
</template>
