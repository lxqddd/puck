import type { ChatState } from '@puck/contract'

export type ActionPriority = 0 | 1 | 2 | 3

export interface ActionDef {
  id: string
  label: string
  /** 毫秒；0 表示保持类动作，直到被 replace/release */
  duration: number
  priority: ActionPriority
  hold?: boolean
  /** 播放结束后的冷却毫秒数 */
  cooldown?: number
}

export const ACTIONS = {
  idle: { id: 'idle', label: '待机', duration: 0, priority: 0, hold: true },
  blink: { id: 'blink', label: '眨眼', duration: 260, priority: 0, cooldown: 3500 },
  wave: { id: 'wave', label: '挥手', duration: 1600, priority: 1 },
  nod: { id: 'nod', label: '点头', duration: 1200, priority: 1 },
  hop: { id: 'hop', label: '跳一下', duration: 900, priority: 2 },
  think: { id: 'think', label: '思考', duration: 0, priority: 1, hold: true },
  work: { id: 'work', label: '忙碌', duration: 0, priority: 1, hold: true },
  speak: { id: 'speak', label: '说话', duration: 0, priority: 1, hold: true },
  happy: { id: 'happy', label: '开心', duration: 1400, priority: 1 },
  sad: { id: 'sad', label: '难过', duration: 1600, priority: 1 },
} as const satisfies Record<string, ActionDef>

export type ActionId = keyof typeof ACTIONS

export interface CharacterPack {
  id: string
  displayName: string
  window: { width: number; height: number }
  actions: ActionId[]
  emotions: string[]
  /** 语义手势 → 动画动作 */
  gestures: Record<string, ActionId>
  /** 语义情绪 → 表情 */
  emotionMap: Record<string, string>
  idlePool: ActionId[]
}

export interface Manifest {
  id: string
  displayName: string
  window: { width: number; height: number }
  actions: string[]
  emotions: string[]
}

export interface Mapping {
  gestures: Record<string, string>
  emotions: Record<string, string>
  idlePool: string[]
}

export function buildCharacterPack(manifest: Manifest, mapping: Mapping): CharacterPack {
  const actions = manifest.actions.filter((id): id is ActionId => id in ACTIONS)
  const known = new Set(actions)
  const gestureEntries = Object.entries(mapping.gestures).filter(
    (entry): entry is [string, ActionId] => known.has(entry[1] as ActionId),
  )
  return {
    id: manifest.id,
    displayName: manifest.displayName,
    window: manifest.window,
    actions,
    emotions: manifest.emotions,
    gestures: Object.fromEntries(gestureEntries),
    emotionMap: { ...mapping.emotions },
    idlePool: mapping.idlePool.filter((id): id is ActionId => known.has(id as ActionId)),
  }
}

export interface AvatarBackend {
  mount(el: HTMLElement, pack: CharacterPack): void
  play(action: ActionId): void
  setEmotion(emotion: string): void
  destroy(): void
}

export interface AvatarChange {
  action: ActionId
  emotion: string
  chatState: ChatState
}

export interface AvatarRuntimeOptions {
  pack: CharacterPack
  backend?: AvatarBackend
  now?: () => number
  onChange?: (change: AvatarChange) => void
}
