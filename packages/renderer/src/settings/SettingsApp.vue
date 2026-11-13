<script setup lang="ts">
import { computed, h, onMounted, ref } from 'vue'
import type { ProviderConfig, ProviderTestResult, Settings, ThemeMode } from '@puck/contract'
import {
  Alert,
  Button,
  Empty,
  Form,
  Input,
  List,
  Popconfirm,
  Segmented,
  Select,
  Spin,
  Switch,
  Tag,
  Tooltip,
  Typography,
} from 'ant-design-vue'
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

const providers = computed(() => settings.value.providers ?? [])
const isExisting = computed(() => providers.value.some((provider) => provider.id === draft.value.id))
const keyPending = computed(() => apiKey.value.trim().length > 0)

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

const emptyImage = h(PuckAvatar, { class: 'mx-auto h-12 w-12 rounded-full opacity-70' })
const { mode: themeMode, setTheme } = useTheme()
const themeOptions: Array<{ value: ThemeMode; label: string }> = [
  { value: 'system', label: '跟随系统' },
  { value: 'light', label: '浅色' },
  { value: 'dark', label: '深色' },
]

function onThemeChange(value: string | number) {
  void setTheme(String(value) as ThemeMode)
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
  errorText.value = ''
  testResult.value = null
  pickingPreset.value = false
  activePresetId.value = ''
  resetModels()
}

function addProvider() {
  draft.value = emptyDraft()
  apiKey.value = ''
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
  if (target?.closest('.ant-switch')) return
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
      class="settings-header flex items-center justify-between gap-4 border-b border-puck-line px-6 pb-3 pt-3 [-webkit-app-region:drag]"
    >
        <div class="min-w-0">
          <Typography.Title :level="4" class="settings-title">设置</Typography.Title>
          <Typography.Text type="secondary" class="settings-subtitle">
            管理模型供应商；API Key 保存在系统钥匙串，当前对话使用选中的供应商。
          </Typography.Text>
        </div>
        <div class="flex-none [-webkit-app-region:no-drag]">
          <Segmented
            class="settings-theme-segmented"
            :options="themeOptions"
            :value="themeMode"
            @change="onThemeChange"
          />
        </div>
      </header>

      <div v-if="loading" class="grid min-h-0 flex-1 place-items-center">
        <Spin size="large" />
      </div>

      <div v-else class="flex min-h-0 flex-1">
        <aside
          class="settings-rail flex min-h-0 w-[218px] flex-none flex-col border-r border-puck-line"
        >
          <div class="settings-rail-head">
            <span class="settings-rail-title">供应商</span>
            <span class="settings-rail-count">{{ providers.length }}</span>
            <Button size="small" class="settings-add-button" @click="startAdd">
              ＋ 添加
            </Button>
          </div>
          <List
            v-if="providers.length > 0"
            :data-source="providers"
            :split="false"
            :bordered="false"
            row-key="id"
            class="provider-list min-h-0 flex-1 overflow-y-auto px-2 pb-3"
          >
            <template #renderItem="{ item }">
              <List.Item class="provider-list-item">
                <button
                  type="button"
                  class="provider-card"
                  :class="{ 'provider-card--active': item.id === draft.id }"
                  @click="edit(item)"
                >
                  <Tooltip
                    :title="
                      item.id === settings.activeProviderId ? '当前使用中' : '设为当前供应商'
                    "
                  >
                    <span
                      class="provider-dot"
                      :class="{ 'provider-dot--on': item.id === settings.activeProviderId }"
                      @click.stop="activate(item.id)"
                    ></span>
                  </Tooltip>
                  <span class="provider-meta">
                    <span class="provider-name">{{ item.name }}</span>
                    <span class="provider-sub">
                      {{ typeLabel(item.type)
                      }}<template v-if="item.id === settings.activeProviderId"> · 使用中</template>
                    </span>
                  </span>
                </button>
              </List.Item>
            </template>
          </List>
          <Empty
            v-else
            :image="emptyImage"
            class="my-6"
            description="还没有供应商"
          >
            <Typography.Text type="secondary" class="settings-rail-empty-hint">
              点上方“添加”创建第一个供应商
            </Typography.Text>
          </Empty>
        </aside>

        <div v-if="pickingPreset" class="settings-picker">
          <header class="settings-picker-head">
            <h2 class="settings-picker-title">添加供应商</h2>
            <p class="settings-picker-desc">从预设快速开始，服务地址和默认模型会自动填好</p>
          </header>

          <Input
            v-model:value="presetQuery"
            class="settings-picker-search"
            placeholder="搜索供应商，如 DeepSeek、Kimi、本地…"
            allow-clear
            autocomplete="off"
          >
            <template #prefix>
              <svg
                class="h-4 w-4"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="1.8"
                stroke-linecap="round"
                stroke-linejoin="round"
              >
                <circle cx="11" cy="11" r="7" />
                <path d="m20 20-3.5-3.5" />
              </svg>
            </template>
          </Input>

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

        <Form
          v-else
          :model="draft"
          layout="vertical"
          class="settings-form min-w-0 flex-1"
          @finish="save"
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
              <Button
                size="small"
                class="settings-secondary-button"
                @click="pickingPreset = true"
              >
                更换
              </Button>
            </div>

            <section class="settings-section">
              <header class="settings-section-head">
                <span class="settings-section-icon" aria-hidden="true">
                  <svg
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="1.7"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                  >
                    <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71" />
                    <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71" />
                  </svg>
                </span>
                <div class="settings-section-heading">
                  <h2 class="settings-section-title">连接方式</h2>
                  <p class="settings-section-desc">选择供应商类型并填写接口地址</p>
                </div>
              </header>

              <div class="settings-grid-2">
                <Form.Item label="名称" html-for="settings-name">
                  <Input
                    id="settings-name"
                    v-model:value="draft.name"
                    placeholder="例如 DeepSeek"
                    autocomplete="off"
                  />
                </Form.Item>
                <Form.Item label="类型" html-for="settings-type">
                  <Select
                    id="settings-type"
                    v-model:value="draft.type"
                    :options="typeOptions"
                    @change="onTypeChange"
                  />
                </Form.Item>
              </div>

              <Form.Item
                v-if="draft.type === 'openai-compatible'"
                label="服务地址"
                html-for="settings-base-url"
                required
              >
                <Input
                  id="settings-base-url"
                  v-model:value="draft.baseURL"
                  placeholder="https://api.deepseek.com/v1"
                  autocomplete="off"
                  @change="resetModels"
                />
              </Form.Item>
              <Form.Item v-else label="自定义地址（可选）" html-for="settings-base-url">
                <Input
                  id="settings-base-url"
                  v-model:value="draft.baseURL"
                  :placeholder="
                    draft.type === 'gemini'
                      ? '留空使用 Gemini 官方 API'
                      : '留空使用 Anthropic 官方 API'
                  "
                  autocomplete="off"
                  @change="resetModels"
                />
              </Form.Item>
            </section>

            <section class="settings-section">
              <header class="settings-section-head">
                <span class="settings-section-icon" aria-hidden="true">
                  <svg
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="1.7"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                  >
                    <path d="M12 3l1.9 5.2L19 10l-5.1 1.8L12 17l-1.9-5.2L5 10l5.1-1.8z" />
                    <path d="M19 15l.9 2.4L22 18l-2.1.7L19 21l-.9-2.3L16 18l2.1-.6z" />
                  </svg>
                </span>
                <div class="settings-section-heading">
                  <h2 class="settings-section-title">模型</h2>
                  <p class="settings-section-desc">对话使用的默认模型</p>
                </div>
              </header>

              <Form.Item label="模型" html-for="settings-model" required>
                <div class="flex items-center gap-2.5">
                  <Input
                    id="settings-model"
                    v-model:value="draft.model"
                    class="min-w-0 flex-1"
                    :placeholder="modelPlaceholder"
                    autocomplete="off"
                  />
                  <Button :loading="modelsLoading" class="settings-fetch-button" @click="fetchModels">
                    获取模型
                  </Button>
                </div>
              </Form.Item>

              <Form.Item v-if="models.length > 0" html-for="settings-model-list">
                <template #label>
                  <span>可选模型</span>
                  <Tag color="success" class="ml-1.5">{{ models.length }} 个</Tag>
                </template>
                <Select
                  id="settings-model-list"
                  v-model:value="draft.model"
                  :options="modelOptions"
                  show-search
                  placeholder="从列表选择"
                />
              </Form.Item>
              <Typography.Text v-else-if="modelsError" type="warning" class="settings-hint">
                {{ modelsError }}
              </Typography.Text>
            </section>

            <section class="settings-section">
              <header class="settings-section-head">
                <span class="settings-section-icon" aria-hidden="true">
                  <svg
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="1.7"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                  >
                    <circle cx="8" cy="8" r="4.2" />
                    <path d="M11 11l8.5 8.5" />
                    <path d="M15.5 15.5l2.2-2.2" />
                  </svg>
                </span>
                <div class="settings-section-heading">
                  <h2 class="settings-section-title">API Key</h2>
                  <p class="settings-section-desc">密钥仅保存在本机系统钥匙串</p>
                </div>
              </header>

              <Form.Item html-for="settings-api-key">
                <template #label>
                  <span>API Key</span>
                  <Tag v-if="draft.hasKey" color="success" class="ml-1.5">已保存</Tag>
                </template>
                <Input.Password
                  id="settings-api-key"
                  v-model:value="apiKey"
                  :placeholder="draft.hasKey ? '留空保持不变' : 'sk-…'"
                  autocomplete="off"
                  @change="resetModels"
                  @blur="onApiKeyBlur"
                />
              </Form.Item>

              <Typography.Text
                v-if="draft.hasKey && draft.keyMask && !keyPending"
                type="secondary"
                class="settings-hint"
              >
                已保存：<Typography.Text code>{{ draft.keyMask }}</Typography.Text>（留空保持不变）
              </Typography.Text>

              <Typography.Text type="secondary" class="settings-hint">
                填入 API Key 后会自动获取该供应商的模型列表，也可点“获取模型”手动刷新。OpenAI
                兼容支持 OpenAI、DeepSeek、通义、Kimi、智谱、Ollama 等；Anthropic 与 Gemini 走官方
                SDK。
              </Typography.Text>

              <Typography.Text v-if="keyPending" type="secondary" class="settings-hint">
                API Key 填好后需要点“{{ isExisting ? '保存供应商' : '添加供应商' }}”才会写入钥匙串。
              </Typography.Text>
            </section>

            <section v-if="draft.type === 'openai-compatible'" class="settings-section">
              <header class="settings-section-head">
                <span class="settings-section-icon" aria-hidden="true">
                  <svg
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="1.7"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                  >
                    <path d="M4 8h9a3 3 0 1 0-3-3" />
                    <path d="M20 16h-9a3 3 0 1 0 3 3" />
                    <circle cx="6" cy="16" r="2" />
                    <circle cx="18" cy="8" r="2" />
                  </svg>
                </span>
                <div class="settings-section-heading">
                  <h2 class="settings-section-title">选项</h2>
                  <p class="settings-section-desc">供应商能力开关</p>
                </div>
              </header>

              <div class="settings-switch-row" @click="toggleWebSearchFromLabel">
                <Switch v-model:checked="draft.webSearch" aria-label="允许联网搜索" />
                <div class="min-w-0">
                  <div class="settings-switch-title">允许联网搜索（模型自动调用）</div>
                  <Typography.Text
                    v-if="draft.webSearch"
                    type="secondary"
                    class="settings-hint"
                  >
                    DeepSeek 模型走 DeepSeek 搜索/抓取；其他兼容模型使用 Tavily
                    keyless（额度较低，被限流时会自动降级为直接回答）。
                  </Typography.Text>
                </div>
              </div>
            </section>

            <div class="settings-note">
              点击左侧圆点切换对话使用的供应商；API Key 只保存在本机系统钥匙串，不会随对话发送。
            </div>

            <Alert
              v-if="testResult"
              class="settings-alert"
              :type="testResult.ok ? 'success' : 'error'"
              show-icon
              :message="
                testResult.ok
                  ? `${testResult.message} · ${testResult.latencyMs} ms`
                  : testResult.message
              "
            />

            <Typography.Text
              v-if="testResult?.ok && keyPending"
              type="secondary"
              class="settings-hint"
            >
              连接成功，但新 Key 尚未保存，请点“保存供应商”。
            </Typography.Text>

            <Alert
              v-if="errorText"
              class="settings-alert"
              type="error"
              show-icon
              :message="errorText"
            />
          </div>

          <div class="settings-actionbar">
            <div class="settings-actionbar-status">
              <Popconfirm
                v-if="draft.id"
                title="确定删除这个供应商吗？"
                ok-text="删除"
                cancel-text="取消"
                @confirm="removeProvider(draft.id)"
              >
                <Button danger type="text" class="settings-delete-button">删除</Button>
              </Popconfirm>
              <Tag v-if="saved" color="success">已保存</Tag>
              <Tag v-else-if="keyPending" color="warning">Key 未保存</Tag>
            </div>
            <div class="settings-actionbar-buttons">
              <Button :loading="testing" class="settings-secondary-button" @click="testConnection">
                测试连接
              </Button>
              <Button
                type="primary"
                html-type="submit"
                :loading="saving"
                class="settings-primary-button"
              >
                {{ isExisting ? '保存供应商' : '添加供应商' }}
              </Button>
            </div>
          </div>
        </Form>
      </div>
    </div>
</template>
