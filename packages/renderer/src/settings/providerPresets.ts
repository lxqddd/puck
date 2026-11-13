import type { ProviderType } from '@puck/contract'

export type ProviderPresetGroup = 'official' | 'cn' | 'aggregate' | 'local'

export interface ProviderPreset {
  id: string
  name: string
  description: string
  /** 列表副标题展示的域名 */
  domain: string
  type: ProviderType
  baseURL: string
  model: string
  group: ProviderPresetGroup
  /** 图标徽章的品牌色 */
  color: string
}

export interface PresetDraft {
  name: string
  type: ProviderType
  baseURL: string
  model: string
  webSearch: boolean
}

export const providerPresetGroups: Array<{ id: ProviderPresetGroup; label: string }> = [
  { id: 'official', label: '官方 API' },
  { id: 'cn', label: '国内服务' },
  { id: 'aggregate', label: '聚合平台' },
  { id: 'local', label: '本地部署' },
]

export const providerPresets: ProviderPreset[] = [
  {
    id: 'anthropic',
    name: 'Anthropic',
    description: 'Claude 官方接口',
    domain: 'anthropic.com',
    type: 'anthropic',
    baseURL: '',
    model: 'claude-opus-5-5',
    group: 'official',
    color: '#d97757',
  },
  {
    id: 'gemini',
    name: 'Gemini',
    description: 'Google 官方接口',
    domain: 'ai.google.dev',
    type: 'gemini',
    baseURL: '',
    model: 'gemini-3.8-flash',
    group: 'official',
    color: '#4285f4',
  },
  {
    id: 'deepseek',
    name: 'DeepSeek',
    description: '深度求索 · 官方直连',
    domain: 'api.deepseek.com',
    type: 'openai-compatible',
    baseURL: 'https://api.deepseek.com/v1',
    model: 'deepseek-chat',
    group: 'cn',
    color: '#4d6bfe',
  },
  {
    id: 'kimi',
    name: 'Kimi',
    description: '月之暗面 Moonshot · 长上下文',
    domain: 'api.moonshot.cn',
    type: 'openai-compatible',
    baseURL: 'https://api.moonshot.cn/v1',
    model: 'kimi-latest',
    group: 'cn',
    color: '#1f2329',
  },
  {
    id: 'zhipu',
    name: '智谱 GLM',
    description: 'GLM 系列 · 有免费额度',
    domain: 'open.bigmodel.cn',
    type: 'openai-compatible',
    baseURL: 'https://open.bigmodel.cn/api/paas/v4',
    model: 'glm-4-flash',
    group: 'cn',
    color: '#3859ff',
  },
  {
    id: 'qwen',
    name: '通义千问',
    description: '阿里云百炼 · 兼容模式',
    domain: 'dashscope.aliyuncs.com',
    type: 'openai-compatible',
    baseURL: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
    model: 'qwen-plus',
    group: 'cn',
    color: '#615ced',
  },
  {
    id: 'siliconflow',
    name: '硅基流动',
    description: '多模型聚合 · 国内直连',
    domain: 'api.siliconflow.cn',
    type: 'openai-compatible',
    baseURL: 'https://api.siliconflow.cn/v1',
    model: 'deepseek-ai/DeepSeek-V3',
    group: 'aggregate',
    color: '#6e29f7',
  },
  {
    id: 'openrouter',
    name: 'OpenRouter',
    description: '全球模型聚合',
    domain: 'openrouter.ai',
    type: 'openai-compatible',
    baseURL: 'https://openrouter.ai/api/v1',
    model: 'anthropic/claude-sonnet-4.5',
    group: 'aggregate',
    color: '#6467f2',
  },
  {
    id: 'ollama',
    name: 'Ollama 本地',
    description: '本机模型 · 无需联网',
    domain: 'localhost:11434',
    type: 'openai-compatible',
    baseURL: 'http://localhost:11434/v1',
    model: 'llama3.2',
    group: 'local',
    color: '#333333',
  },
]

export function presetToDraft(preset: ProviderPreset): PresetDraft {
  return {
    name: preset.name,
    type: preset.type,
    baseURL: preset.baseURL,
    model: preset.model,
    webSearch: true,
  }
}

/** 名称、描述、域名和模型的模糊匹配，供预设搜索框使用 */
export function matchPreset(preset: ProviderPreset, query: string): boolean {
  const keyword = query.trim().toLowerCase()
  if (!keyword) return true
  return [preset.name, preset.description, preset.domain, preset.model, preset.id].some(
    (field) => field.toLowerCase().includes(keyword),
  )
}
