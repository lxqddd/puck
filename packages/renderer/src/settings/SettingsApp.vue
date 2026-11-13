<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import type { ProviderConfig, ProviderTestResult, Settings, ThemeMode } from '@puck/contract'
import {
  CircleAlertIcon,
  CircleCheckIcon,
  EyeIcon,
  EyeOffIcon,
  GlobeIcon,
  KeyRoundIcon,
  Link2Icon,
  Loader2Icon,
  PlusIcon,
  SearchIcon,
  SparklesIcon,
  Trash2Icon,
  XIcon,
} from '@lucide/vue'
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
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { bridge } from '../bridge'
import PuckAvatar from '../shared/PuckAvatar.vue'
import { useTheme } from '../shared/theme'
import {
  matchPreset,
  presetToDraft,
  providerPresetGroups,
  providerPresets,
} from './providerPresets'
import type { ProviderPreset } from './providerPresets'

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
const showKey = ref(false)
const deleteProviderOpen = ref(false)
const deleteProviderId = ref('')

const providers = computed(() => settings.value.providers ?? [])
const isExisting = computed(() => providers.value.some((provider) => provider.id === draft.value.id))
const keyPending = computed(() => apiKey.value.trim().length > 0)
const deleteProviderName = computed(
  () =>
    providers.value.find((provider) => provider.id === deleteProviderId.value)?.name ??
    '这个供应商',
)

const pickingPreset = ref(false)
const presetQuery = ref('')
const activePresetId = ref('')

const activePreset = computed(
  () => providerPresets.find((preset) => preset.id === activePresetId.value) ?? null,
)

const filteredPresetGroups = computed(() =>
  providerPresetGroups
    .map((group) => ({
      ...group,
      presets: providerPresets.filter(
        (preset) => preset.group === group.id && matchPreset(preset, presetQuery.value),
      ),
    }))
    .filter((group) => group.presets.length > 0),
)

const DEFAULT_MODELS: Record<string, string> = {
  'openai-compatible': 'deepseek-chat',
  anthropic: 'claude-opus-5-5',
  gemini: 'gemini-3.8-flash',
}

const modelPlaceholder = computed(() => DEFAULT_MODELS[draft.value.type] ?? 'deepseek-chat')

const typeOptions = [
  { value: 'openai-compatible', label: 'OpenAI 兼容' },
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'gemini', label: 'Google Gemini' },
]

const modelOptions = computed(() => {
  const options = models.value.map((model) => ({ value: model, label: model }))
  if (draft.value.model && !models.value.includes(draft.value.model)) {
    options.unshift({ value: draft.value.model, label: `${draft.value.model}（当前）` })
  }
  return options
})

const { mode: themeMode, setTheme } = useTheme()
const themeOptions: Array<{ value: ThemeMode; label: string }> = [
  { value: 'system', label: '跟随系统' },
  { value: 'light', label: '浅色' },
  { value: 'dark', label: '深色' },
]

function onThemeChange(value: unknown) {
  const next = Array.isArray(value) ? value[0] : value
  if (next) void setTheme(String(next) as ThemeMode)
}

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
  showKey.value = false
  errorText.value = ''
  testResult.value = null
  pickingPreset.value = false
  activePresetId.value = ''
  resetModels()
}

function addProvider() {
  draft.value = emptyDraft()
  apiKey.value = ''
  showKey.value = false
  errorText.value = ''
  testResult.value = null
  resetModels()
}

/** 「＋ 添加」：先进入预设选择，选中后再填表单（参考 cc-switch 的两步流程） */
function startAdd() {
  addProvider()
  pickingPreset.value = true
  presetQuery.value = ''
  activePresetId.value = ''
}

function pickPreset(preset: ProviderPreset) {
  draft.value = { ...emptyDraft(), ...presetToDraft(preset) }
  apiKey.value = ''
  showKey.value = false
  errorText.value = ''
  testResult.value = null
  resetModels()
  activePresetId.value = preset.id
  pickingPreset.value = false
}

function chooseCustom() {
  addProvider()
  activePresetId.value = 'custom'
  pickingPreset.value = false
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

function onTypeChange(value: unknown) {
  const type = String(value)
  draft.value.type = type
  resetModels()
  const knownModels = Object.values(DEFAULT_MODELS)
  if (!draft.value.model.trim() || knownModels.includes(draft.value.model.trim())) {
    draft.value.model = DEFAULT_MODELS[type] ?? 'deepseek-chat'
  }
  if (type !== 'openai-compatible' && draft.value.baseURL === 'https://api.deepseek.com/v1') {
    draft.value.baseURL = ''
  }
  if (type === 'openai-compatible' && !draft.value.baseURL.trim()) {
    draft.value.baseURL = 'https://api.deepseek.com/v1'
  }
}

function toggleWebSearchFromLabel(event: MouseEvent) {
  const target = event.target as HTMLElement | null
  if (target?.closest('[role="switch"]') || target?.closest('[data-slot="switch"]')) return
  draft.value.webSearch = !draft.value.webSearch
}

async function load() {
  settings.value = await bridge.getSettings()
  const active =
    providers.value.find((provider) => provider.id === settings.value.activeProviderId) ??
    providers.value[0]
  if (active) {
    edit(active)
  } else {
    startAdd()
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
  try {
    await bridge.deleteProvider(id)
    await load()
  } catch (error) {
    errorText.value = error instanceof Error ? error.message : String(error)
  }
}

function requestRemoveProvider(id: string) {
  deleteProviderId.value = id
  deleteProviderOpen.value = true
}

async function confirmRemoveProvider() {
  const id = deleteProviderId.value
  deleteProviderOpen.value = false
  deleteProviderId.value = ''
  if (id) await removeProvider(id)
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
      class="settings-header flex items-center justify-between gap-4 border-b border-border bg-card px-5 py-2.5 [-webkit-app-region:drag]"
    >
      <div class="min-w-0">
        <h1 class="settings-title">设置</h1>
        <p class="settings-subtitle">
          管理模型供应商；API Key 保存在系统钥匙串，当前对话使用选中的供应商。
        </p>
      </div>
      <div class="flex-none [-webkit-app-region:no-drag]">
        <ToggleGroup
          type="single"
          :model-value="themeMode"
          variant="outline"
          size="sm"
          class="settings-theme-toggle"
          @update:model-value="onThemeChange"
        >
          <ToggleGroupItem
            v-for="option in themeOptions"
            :key="option.value"
            :value="option.value"
          >
            {{ option.label }}
          </ToggleGroupItem>
        </ToggleGroup>
      </div>
    </header>

    <div v-if="loading" class="grid min-h-0 flex-1 place-items-center">
      <Loader2Icon class="size-5 animate-spin text-muted-foreground" />
    </div>

    <div v-else class="flex min-h-0 flex-1">
      <aside
        class="settings-rail flex min-h-0 w-[218px] flex-none flex-col border-r border-border bg-card"
      >
        <div class="settings-rail-head">
          <span class="settings-rail-title">供应商</span>
          <span class="settings-rail-count">{{ providers.length }}</span>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            class="settings-add-button ml-auto"
            @click="startAdd"
          >
            <PlusIcon class="size-3.5" />
            添加
          </Button>
        </div>

        <div v-if="providers.length > 0" class="provider-list min-h-0 flex-1 overflow-y-auto px-2 pb-3">
          <div
            v-for="provider in providers"
            :key="provider.id"
            class="provider-card"
            :class="{ 'provider-card--active': provider.id === draft.id }"
          >
            <Tooltip>
              <TooltipTrigger as-child>
                <button
                  type="button"
                  class="provider-dot"
                  :class="{ 'provider-dot--on': provider.id === settings.activeProviderId }"
                  :aria-label="
                    provider.id === settings.activeProviderId
                      ? '当前使用中的供应商'
                      : '设为当前供应商'
                  "
                  @click.stop="activate(provider.id)"
                ></button>
              </TooltipTrigger>
              <TooltipContent side="right">
                {{
                  provider.id === settings.activeProviderId
                    ? '当前使用中'
                    : '设为当前供应商'
                }}
              </TooltipContent>
            </Tooltip>
            <button
              type="button"
              class="provider-card-main"
              @click="edit(provider)"
            >
              <span class="provider-name">{{ provider.name }}</span>
              <span class="provider-sub">
                {{ typeLabel(provider.type)
                }}<template v-if="provider.id === settings.activeProviderId"> · 使用中</template>
              </span>
            </button>
          </div>
        </div>

        <div v-else class="settings-rail-empty">
          <PuckAvatar class="size-12 rounded-full opacity-70" />
          <p class="settings-rail-empty-title">还没有供应商</p>
          <p class="settings-rail-empty-hint">点上方“添加”创建第一个供应商</p>
        </div>
      </aside>

      <div v-if="pickingPreset" class="settings-picker">
        <header class="settings-picker-head">
          <h2 class="settings-picker-title">添加供应商</h2>
          <p class="settings-picker-desc">从预设快速开始，服务地址和默认模型会自动填好</p>
        </header>

        <div class="settings-picker-search">
          <SearchIcon class="settings-search-icon" />
          <Input
            v-model="presetQuery"
            type="search"
            placeholder="搜索供应商，如 DeepSeek、Kimi、本地…"
            autocomplete="off"
            aria-label="搜索供应商"
            class="pl-9 pr-9"
          />
          <Button
            v-if="presetQuery"
            type="button"
            variant="ghost"
            size="icon-xs"
            class="settings-search-clear"
            aria-label="清空搜索"
            @click="presetQuery = ''"
          >
            <XIcon class="size-3.5" />
          </Button>
        </div>

        <button type="button" class="preset-card preset-card--custom" @click="chooseCustom">
          <span class="preset-badge preset-badge--custom" aria-hidden="true">＋</span>
          <span class="preset-meta">
            <span class="preset-name">自定义配置</span>
            <span class="preset-domain">手动填写全部字段</span>
          </span>
        </button>

        <div v-if="filteredPresetGroups.length === 0" class="settings-picker-empty">
          <p class="settings-picker-empty-title">没有匹配的供应商</p>
          <p class="settings-picker-empty-desc">换个关键词试试，或直接使用自定义配置。</p>
        </div>

        <section v-for="group in filteredPresetGroups" :key="group.id" class="preset-group">
          <h3 class="preset-group-title">{{ group.label }}</h3>
          <div class="preset-grid">
            <button
              v-for="preset in group.presets"
              :key="preset.id"
              type="button"
              class="preset-card"
              @click="pickPreset(preset)"
            >
              <span
                class="preset-badge"
                :style="{ '--preset-color': preset.color }"
                aria-hidden="true"
              >
                {{ preset.name.slice(0, 1) }}
              </span>
              <span class="preset-meta">
                <span class="preset-name">{{ preset.name }}</span>
                <span class="preset-domain">{{ preset.description }}</span>
              </span>
            </button>
          </div>
        </section>
      </div>

      <form
        v-else
        class="settings-form min-w-0 flex-1"
        novalidate
        @submit.prevent="save"
      >
        <div class="settings-scroll">
          <div v-if="!draft.id && activePresetId" class="preset-bar">
            <template v-if="activePreset">
              <span
                class="preset-badge"
                :style="{ '--preset-color': activePreset.color }"
                aria-hidden="true"
              >
                {{ activePreset.name.slice(0, 1) }}
              </span>
              <span class="preset-bar-meta">
                <span class="preset-bar-name">{{ activePreset.name }}</span>
                <span class="preset-bar-domain">
                  {{ activePreset.baseURL || activePreset.domain }}
                </span>
              </span>
            </template>
            <template v-else>
              <span class="preset-badge preset-badge--custom" aria-hidden="true">＋</span>
              <span class="preset-bar-meta">
                <span class="preset-bar-name">自定义配置</span>
                <span class="preset-bar-domain">手动填写全部字段</span>
              </span>
            </template>
            <Button type="button" variant="outline" size="sm" @click="pickingPreset = true">
              更换
            </Button>
          </div>

          <section class="settings-section">
            <header class="settings-section-head">
              <span class="settings-section-icon" aria-hidden="true">
                <Link2Icon class="size-4" />
              </span>
              <div class="settings-section-heading">
                <h2 class="settings-section-title">连接方式</h2>
                <p class="settings-section-desc">选择供应商类型并填写接口地址</p>
              </div>
            </header>

            <div class="settings-grid-2">
              <div class="settings-field">
                <Label for="settings-name" class="settings-label">名称</Label>
                <Input
                  id="settings-name"
                  v-model="draft.name"
                  placeholder="例如 DeepSeek"
                  autocomplete="off"
                />
              </div>
              <div class="settings-field">
                <Label for="settings-type" class="settings-label">类型</Label>
                <Select v-model="draft.type" @update:model-value="onTypeChange">
                  <SelectTrigger id="settings-type" type="button" class="w-full">
                    <SelectValue placeholder="选择类型" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem
                      v-for="option in typeOptions"
                      :key="option.value"
                      :value="option.value"
                    >
                      {{ option.label }}
                    </SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>

            <div v-if="draft.type === 'openai-compatible'" class="settings-field">
              <Label for="settings-base-url" class="settings-label">
                服务地址
                <span class="settings-required" aria-hidden="true">*</span>
              </Label>
              <Input
                id="settings-base-url"
                v-model="draft.baseURL"
                placeholder="https://api.deepseek.com/v1"
                autocomplete="off"
                @change="resetModels"
              />
            </div>
            <div v-else class="settings-field">
              <Label for="settings-base-url" class="settings-label">自定义地址（可选）</Label>
              <Input
                id="settings-base-url"
                v-model="draft.baseURL"
                :placeholder="
                  draft.type === 'gemini'
                    ? '留空使用 Gemini 官方 API'
                    : '留空使用 Anthropic 官方 API'
                "
                autocomplete="off"
                @change="resetModels"
              />
            </div>
          </section>

          <section class="settings-section">
            <header class="settings-section-head">
              <span class="settings-section-icon" aria-hidden="true">
                <SparklesIcon class="size-4" />
              </span>
              <div class="settings-section-heading">
                <h2 class="settings-section-title">模型</h2>
                <p class="settings-section-desc">对话使用的默认模型</p>
              </div>
            </header>

            <div class="settings-field">
              <Label for="settings-model" class="settings-label">
                模型
                <span class="settings-required" aria-hidden="true">*</span>
              </Label>
              <div class="flex items-center gap-2.5">
                <Input
                  id="settings-model"
                  v-model="draft.model"
                  class="min-w-0 flex-1"
                  :placeholder="modelPlaceholder"
                  autocomplete="off"
                />
                <Button
                  type="button"
                  variant="outline"
                  class="settings-fetch-button"
                  :disabled="modelsLoading"
                  @click="fetchModels"
                >
                  <Loader2Icon v-if="modelsLoading" class="size-4 animate-spin" />
                  获取模型
                </Button>
              </div>
            </div>

            <div v-if="models.length > 0" class="settings-field">
              <div class="settings-label-row">
                <Label for="settings-model-list" class="settings-label">可选模型</Label>
                <Badge variant="secondary">{{ models.length }} 个</Badge>
              </div>
              <Select v-model="draft.model">
                <SelectTrigger id="settings-model-list" type="button" class="w-full">
                  <SelectValue placeholder="从列表选择" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem
                    v-for="option in modelOptions"
                    :key="option.value"
                    :value="option.value"
                  >
                    {{ option.label }}
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>
            <p v-else-if="modelsError" class="settings-hint settings-hint--warning">
              {{ modelsError }}
            </p>
          </section>

          <section class="settings-section">
            <header class="settings-section-head">
              <span class="settings-section-icon" aria-hidden="true">
                <KeyRoundIcon class="size-4" />
              </span>
              <div class="settings-section-heading">
                <h2 class="settings-section-title">API Key</h2>
                <p class="settings-section-desc">密钥仅保存在本机系统钥匙串</p>
              </div>
            </header>

            <div class="settings-field">
              <div class="settings-label-row">
                <Label for="settings-api-key" class="settings-label">API Key</Label>
                <Badge v-if="draft.hasKey" class="settings-badge--success">已保存</Badge>
              </div>
              <div class="settings-key-wrap">
                <Input
                  id="settings-api-key"
                  v-model="apiKey"
                  :type="showKey ? 'text' : 'password'"
                  class="pr-10"
                  :placeholder="draft.hasKey ? '留空保持不变' : 'sk-…'"
                  autocomplete="off"
                  @change="resetModels"
                  @blur="onApiKeyBlur"
                />
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-xs"
                  class="settings-key-toggle"
                  :aria-label="showKey ? '隐藏 API Key' : '显示 API Key'"
                  @click="showKey = !showKey"
                >
                  <EyeOffIcon v-if="showKey" class="size-3.5" />
                  <EyeIcon v-else class="size-3.5" />
                </Button>
              </div>
            </div>

            <p v-if="draft.hasKey && draft.keyMask && !keyPending" class="settings-hint">
              已保存：<code class="settings-code">{{ draft.keyMask }}</code>（留空保持不变）
            </p>

            <p class="settings-hint">
              填入 API Key 后会自动获取该供应商的模型列表，也可点“获取模型”手动刷新。OpenAI
              兼容支持 OpenAI、DeepSeek、通义、Kimi、智谱、Ollama 等；Anthropic 与 Gemini 走官方
              SDK。
            </p>

            <p v-if="keyPending" class="settings-hint">
              API Key 填好后需要点“{{ isExisting ? '保存供应商' : '添加供应商' }}”才会写入钥匙串。
            </p>
          </section>

          <section v-if="draft.type === 'openai-compatible'" class="settings-section">
            <header class="settings-section-head">
              <span class="settings-section-icon" aria-hidden="true">
                <GlobeIcon class="size-4" />
              </span>
              <div class="settings-section-heading">
                <h2 class="settings-section-title">选项</h2>
                <p class="settings-section-desc">供应商能力开关</p>
              </div>
            </header>

            <div class="settings-switch-row" @click="toggleWebSearchFromLabel">
              <Switch
                v-model="draft.webSearch"
                aria-label="允许联网搜索"
              />
              <div class="min-w-0">
                <div class="settings-switch-title">允许联网搜索（模型自动调用）</div>
                <p v-if="draft.webSearch" class="settings-hint">
                  DeepSeek 模型走 DeepSeek 搜索/抓取；其他兼容模型使用 Tavily
                  keyless（额度较低，被限流时会自动降级为直接回答）。
                </p>
              </div>
            </div>
          </section>

          <div class="settings-note">
            点击左侧圆点切换对话使用的供应商；API Key 只保存在本机系统钥匙串，不会随对话发送。
          </div>

          <Alert
            v-if="testResult"
            :variant="testResult.ok ? undefined : 'destructive'"
            class="settings-alert"
            :class="{ 'settings-alert--success': testResult.ok }"
          >
            <CircleCheckIcon v-if="testResult.ok" class="size-4" />
            <CircleAlertIcon v-else class="size-4" />
            <AlertTitle>{{ testResult.ok ? '连接成功' : '连接失败' }}</AlertTitle>
            <AlertDescription>
              {{
                testResult.ok
                  ? `${testResult.message} · ${testResult.latencyMs} ms`
                  : testResult.message
              }}
            </AlertDescription>
          </Alert>

          <p v-if="testResult?.ok && keyPending" class="settings-hint">
            连接成功，但新 Key 尚未保存，请点“保存供应商”。
          </p>

          <Alert v-if="errorText" variant="destructive" class="settings-alert">
            <CircleAlertIcon class="size-4" />
            <AlertTitle>出错了</AlertTitle>
            <AlertDescription>{{ errorText }}</AlertDescription>
          </Alert>
        </div>

        <div class="settings-actionbar">
          <div class="settings-actionbar-status">
            <Button
              v-if="draft.id"
              type="button"
              variant="ghost"
              size="sm"
              class="settings-delete-button"
              :disabled="saving"
              @click="requestRemoveProvider(draft.id)"
            >
              <Trash2Icon class="size-3.5" />
              删除
            </Button>
            <Badge v-if="saved" class="settings-badge--success">已保存</Badge>
            <Badge v-else-if="keyPending" class="settings-badge--warning">Key 未保存</Badge>
          </div>
          <div class="settings-actionbar-buttons">
            <Button
              type="button"
              variant="outline"
              :disabled="testing"
              @click="testConnection"
            >
              <Loader2Icon v-if="testing" class="size-4 animate-spin" />
              测试连接
            </Button>
            <Button type="submit" :disabled="saving" class="settings-primary-button">
              <Loader2Icon v-if="saving" class="size-4 animate-spin" />
              {{ isExisting ? '保存供应商' : '添加供应商' }}
            </Button>
          </div>
        </div>
      </form>
    </div>

    <AlertDialog v-model:open="deleteProviderOpen">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>删除这个供应商？</AlertDialogTitle>
          <AlertDialogDescription>
            “{{ deleteProviderName }}”将从本机移除，API Key 也会一并删除。
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>取消</AlertDialogCancel>
          <AlertDialogAction
            class="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            @click="confirmRemoveProvider"
          >
            删除
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </div>
</template>
