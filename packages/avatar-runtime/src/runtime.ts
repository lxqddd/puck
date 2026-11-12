import type { ChatState } from '@puck/contract'
import {
  ACTIONS,
  type ActionDef,
  type ActionId,
  type AvatarBackend,
  type AvatarRuntimeOptions,
  type CharacterPack,
} from './types.js'

const DEFAULT_EMOTION = 'neutral'

/**
 * 动作调度器：优先级替换、保持类动作、冷却、空闲池。
 * 规则参考 nomi-ai 的桌面角色调度模型，保持与渲染后端解耦。
 */
export class AvatarRuntime {
  private readonly pack: CharacterPack
  private backend: AvatarBackend | null
  private readonly now: () => number
  private readonly onChange?: AvatarRuntimeOptions['onChange']

  private action: ActionId = 'idle'
  private emotion = DEFAULT_EMOTION
  /** 0 表示保持类动作，不自动结束 */
  private actionUntil = 0
  private readonly cooldownUntil = new Map<ActionId, number>()
  private chatState: ChatState = 'idle'
  private idleAt = 0

  constructor(options: AvatarRuntimeOptions) {
    this.pack = options.pack
    this.backend = options.backend ?? null
    this.now = options.now ?? (() => performance.now())
    this.onChange = options.onChange
  }

  mount(el: HTMLElement): void {
    if (!this.backend) throw new Error('AvatarRuntime: no backend mounted')
    this.backend.mount(el, this.pack)
    this.idleAt = this.now()
    this.emit()
  }

  setBackend(backend: AvatarBackend | null): void {
    this.backend = backend
  }

  play(id: ActionId, opts: { force?: boolean } = {}): boolean {
    const def = ACTIONS[id] as ActionDef | undefined
    if (!def) return false
    const t = this.now()

    if (!opts.force) {
      const cooledUntil = this.cooldownUntil.get(id) ?? 0
      if (t < cooledUntil) return false
      const current = ACTIONS[this.action] as ActionDef
      const currentActive = this.actionUntil === 0 || t < this.actionUntil
      if (currentActive && current.hold && current.id !== 'idle') return false
      if (currentActive && current.id !== 'idle' && def.priority < current.priority) return false
      if (currentActive && def.priority === 0 && current.id !== 'idle') return false
    }

    this.action = id
    this.actionUntil = def.hold ? 0 : t + def.duration
    if (def.cooldown) this.cooldownUntil.set(id, t + def.duration + def.cooldown)
    this.backend?.play(id)
    this.emit()
    return true
  }

  /** 结束保持类动作（思考/说话/忙碌），回到待机 */
  release(id?: ActionId): void {
    const current = ACTIONS[this.action] as ActionDef
    if (!current.hold) return
    if (id && id !== this.action) return
    this.action = 'idle'
    this.actionUntil = 0
    this.idleAt = this.now()
    this.backend?.play('idle')
    this.emit()
  }

  setChatState(state: ChatState): void {
    if (this.chatState === state) return
    this.chatState = state
    if (state === 'thinking') this.play('think', { force: true })
    else if (state === 'speaking') this.play('speak', { force: true })
    else this.release()
    this.emit()
  }

  react(kind: 'click' | 'double' | 'hover'): void {
    if (kind === 'click') this.play('hop')
    else if (kind === 'double') this.play('wave')
    else if (this.chatState === 'idle') this.play('blink')
  }

  setEmotion(name: string): void {
    if (this.pack.emotions.length > 0 && !this.pack.emotions.includes(name)) return
    this.emotion = name
    this.backend?.setEmotion(name)
    this.emit()
  }

  setLipLevel(level: number): void {
    this.backend?.setLipLevel?.(Math.min(1, Math.max(0, level)))
  }

  applySemanticEmotion(name: string): void {
    const mapped = this.pack.emotionMap[name]
    if (mapped) this.setEmotion(mapped)
  }

  applyGesture(name: string): void {
    const mapped = this.pack.gestures[name]
    if (mapped) this.play(mapped)
  }

  /** 由宿主按固定间隔调用；空闲时随机播放 idlePool 中的动作 */
  tick(now = this.now()): void {
    if (this.actionUntil !== 0 && now >= this.actionUntil) {
      this.action = 'idle'
      this.actionUntil = 0
      this.backend?.play('idle')
      this.emit()
    }
    if (this.chatState !== 'idle' || this.action !== 'idle') return
    if (now - this.idleAt < 18_000 || this.pack.idlePool.length === 0) return
    const next = this.pack.idlePool[Math.floor(now / 1000) % this.pack.idlePool.length]
    if (next && this.play(next)) this.idleAt = now
  }

  get current(): { action: ActionId; emotion: string; chatState: ChatState } {
    return { action: this.action, emotion: this.emotion, chatState: this.chatState }
  }

  destroy(): void {
    this.backend?.destroy()
    this.backend = null
    this.cooldownUntil.clear()
  }

  private emit(): void {
    this.onChange?.({ action: this.action, emotion: this.emotion, chatState: this.chatState })
  }
}

export class DomAvatarBackend implements AvatarBackend {
  private el: HTMLElement | null = null

  mount(el: HTMLElement, pack: CharacterPack): void {
    this.el = el
    el.dataset.avatar = pack.id
    el.dataset.action = 'idle'
    el.dataset.emotion = DEFAULT_EMOTION
  }

  play(action: ActionId): void {
    if (this.el) this.el.dataset.action = action
  }

  setEmotion(emotion: string): void {
    if (this.el) this.el.dataset.emotion = emotion
  }

  destroy(): void {
    this.el = null
  }
}
