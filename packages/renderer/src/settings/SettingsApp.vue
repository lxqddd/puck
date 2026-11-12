<script setup lang="ts">
import { computed, h, onMounted, ref } from 'vue'
import type { ProviderConfig, ProviderTestResult, Settings, ThemeMode } from '@puck/contract'
import {
  Alert,
  Button,
  ConfigProvider,
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
import { useAntTheme } from '../shared/antdTheme'
import PuckAvatar from '../shared/PuckAvatar.vue'
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

const antTheme = useAntTheme()
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
  <ConfigProvider :theme="antTheme">
    <div class="flex h-full flex-col">
      <header
        class="settings-header flex items-center justify-between gap-4 border-b border-puck-line px-6 pb-3 pl-[88px] pt-3 [-webkit-app-region:drag]"
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
          class="settings-rail flex min-h-0 w-[210px] flex-none flex-col border-r border-puck-line bg-white/45"
        >
          <div class="flex items-center justify-between px-3 pb-2 pt-3.5">
            <Typography.Text type="secondary" class="text-xs font-semibold">
              供应商
            </Typography.Text>
            <Button type="link" size="small" class="settings-add-button" @click="addProvider">
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
                  class="flex w-full items-center gap-2 rounded-[10px] border-none bg-transparent px-2.5 py-[9px] text-left text-[13px] text-puck-ink transition-colors hover:bg-puck-ink/5 dark:hover:bg-puck-surface"
                  :class="{ 'bg-puck-mint/16!': item.id === draft.id }"
                  @click="edit(item)"
                >
                  <Tooltip
                    :title="
                      item.id === settings.activeProviderId ? '当前使用中' : '设为当前供应商'
                    "
                  >
                    <span
                      class="h-2.5 w-2.5 flex-none rounded-full border-[1.5px] border-puck-muted"
                      :class="{
                        'border-puck-mint bg-puck-mint shadow-[0_0_0_2px_rgba(63,201,165,0.2)]':
                          item.id === settings.activeProviderId,
                      }"
                      @click.stop="activate(item.id)"
                    ></span>
                  </Tooltip>
                  <span class="flex-1 truncate">{{ item.name }}</span>
                  <span class="text-[11px] text-puck-muted">{{ typeLabel(item.type) }}</span>
                </button>
              </List.Item>
            </template>
          </List>
          <Empty
            v-else
            :image="emptyImage"
            class="my-6"
            description="还没有供应商"
          />
        </aside>

        <Form
          :model="draft"
          layout="vertical"
          class="settings-form min-w-0 flex-1 overflow-y-auto px-7 pb-7 pt-5"
          @finish="save"
        >
          <Form.Item label="名称">
            <Input v-model:value="draft.name" placeholder="例如 DeepSeek" autocomplete="off" />
          </Form.Item>

          <Form.Item label="类型">
            <Select
              v-model:value="draft.type"
              :options="typeOptions"
              @change="onTypeChange"
            />
          </Form.Item>

          <Form.Item v-if="draft.type === 'openai-compatible'" label="服务地址" required>
            <Input
              v-model:value="draft.baseURL"
              placeholder="https://api.deepseek.com/v1"
              autocomplete="off"
              @change="resetModels"
            />
          </Form.Item>

          <Form.Item v-else label="自定义地址（可选）">
            <Input
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

          <div class="flex items-start gap-2.5">
            <Form.Item label="模型" required class="min-w-0 flex-1">
              <Input
                v-model:value="draft.model"
                :placeholder="modelPlaceholder"
                autocomplete="off"
              />
            </Form.Item>
            <Form.Item label=" " class="flex-none">
              <Button :loading="modelsLoading" @click="fetchModels">获取模型</Button>
            </Form.Item>
          </div>

          <Form.Item v-if="models.length > 0">
            <template #label>
              <span>可选模型</span>
              <Tag color="success" class="ml-1.5">{{ models.length }} 个</Tag>
            </template>
            <Select
              v-model:value="draft.model"
              :options="modelOptions"
              show-search
              placeholder="从列表选择"
            />
          </Form.Item>
          <Typography.Text
            v-else-if="modelsError"
            type="warning"
            class="block text-xs leading-relaxed"
          >
            {{ modelsError }}
          </Typography.Text>

          <template v-if="draft.type === 'openai-compatible'">
            <div class="flex items-center gap-2.5 text-[13px] text-puck-ink">
              <Switch v-model:checked="draft.webSearch" />
              <span>允许联网搜索（模型自动调用）</span>
            </div>
            <Typography.Text
              v-if="draft.webSearch"
              type="secondary"
              class="mt-2 block text-xs leading-relaxed"
            >
              DeepSeek 模型走 DeepSeek 搜索/抓取；其他兼容模型使用 Tavily keyless（额度较低，被限流时会自动降级为直接回答）。
            </Typography.Text>
          </template>

          <Form.Item>
            <template #label>
              <span>API Key</span>
              <Tag v-if="draft.hasKey" color="success" class="ml-1.5">已保存</Tag>
            </template>
            <Input.Password
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
            class="block text-xs leading-relaxed"
          >
            已保存：<Typography.Text code>{{ draft.keyMask }}</Typography.Text>（留空保持不变）
          </Typography.Text>

          <Typography.Text type="secondary" class="mt-2 block text-xs leading-relaxed">
            填入 API Key 后会自动获取该供应商的模型列表，也可点“获取模型”手动刷新。OpenAI
            兼容支持 OpenAI、DeepSeek、通义、Kimi、智谱、Ollama 等；Anthropic 与 Gemini 走官方
            SDK。点击左侧圆点切换对话使用的供应商。
          </Typography.Text>

          <div class="mt-4 flex flex-wrap items-center gap-3">
            <Button
              type="primary"
              html-type="submit"
              :loading="saving"
              class="settings-primary-button"
            >
              {{ isExisting ? '保存供应商' : '添加供应商' }}
            </Button>
            <Button
              :loading="testing"
              class="settings-secondary-button"
              @click="testConnection"
            >
              测试连接
            </Button>
            <Popconfirm
              v-if="draft.id"
              title="确定删除这个供应商吗？"
              ok-text="删除"
              cancel-text="取消"
              @confirm="removeProvider(draft.id)"
            >
              <Button danger>删除</Button>
            </Popconfirm>
            <Tag v-if="saved" color="success">已保存</Tag>
            <Tag v-else-if="keyPending" color="warning">Key 未保存</Tag>
          </div>

          <Typography.Text
            v-if="keyPending"
            type="secondary"
            class="mt-2 block text-xs leading-relaxed"
          >
            API Key 填好后需要点“{{ isExisting ? '保存供应商' : '添加供应商' }}”才会写入钥匙串。
          </Typography.Text>

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
            class="mt-2 block text-xs leading-relaxed"
          >
            连接成功，但新 Key 尚未保存，请点“保存供应商”。
          </Typography.Text>

          <Alert v-if="errorText" class="settings-alert" type="error" show-icon :message="errorText" />
        </Form>
      </div>
    </div>
  </ConfigProvider>
</template>
