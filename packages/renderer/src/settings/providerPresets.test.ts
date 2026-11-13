import { describe, expect, it } from 'vitest'
import { providerPresets, presetToDraft } from './providerPresets'

describe('provider presets', () => {
  it('have unique ids and required fields', () => {
    const ids = new Set<string>()
    for (const preset of providerPresets) {
      expect(preset.id).toBeTruthy()
      expect(ids.has(preset.id)).toBe(false)
      ids.add(preset.id)
      expect(preset.name).toBeTruthy()
      expect(preset.model).toBeTruthy()
      expect(preset.domain).toBeTruthy()
      expect(preset.description).toBeTruthy()
    }
  })

  it('use http(s) base urls for openai-compatible presets', () => {
    const compatible = providerPresets.filter((preset) => preset.type === 'openai-compatible')
    expect(compatible.length).toBeGreaterThan(0)
    for (const preset of compatible) {
      expect(preset.baseURL).toMatch(/^https?:\/\//)
    }
  })

  it('map a preset to draft fields', () => {
    const preset = providerPresets.find((item) => item.id === 'deepseek')
    expect(preset).toBeDefined()
    expect(presetToDraft(preset!)).toEqual({
      name: 'DeepSeek',
      type: 'openai-compatible',
      baseURL: 'https://api.deepseek.com/v1',
      model: 'deepseek-chat',
      webSearch: true,
    })
  })

  it('leaves the base url empty for official anthropic and gemini', () => {
    for (const preset of providerPresets.filter(
      (item) => item.type === 'anthropic' || item.type === 'gemini',
    )) {
      expect(preset.baseURL).toBe('')
    }
  })
})
