import { DomAvatarBackend } from './runtime.js'
import { RiveBackend } from './rive.js'
import type { ActionId, AvatarBackend, CharacterPack } from './types.js'

export interface ResilientAvatarBackendOptions {
  wasmUrl?: string
  onFallback?: (error: Error) => void
}

/** 优先使用 Rive；加载失败时自动回退到静态 DOM 角色。 */
export class ResilientAvatarBackend implements AvatarBackend {
  private active: AvatarBackend | null = null
  private rive: RiveBackend | null = null
  private el: HTMLElement | null = null
  private pack: CharacterPack | null = null
  private lastAction: ActionId = 'idle'
  private lastEmotion = 'neutral'
  private lastLip = 0

  constructor(private readonly options: ResilientAvatarBackendOptions = {}) {}

  mount(el: HTMLElement, pack: CharacterPack): void {
    this.el = el
    this.pack = pack
    if (pack.renderMode === 'rive' && pack.riveAssetUrl) {
      const rive = new RiveBackend({
        wasmUrl: this.options.wasmUrl,
        onError: (error) => this.fallback(error),
      })
      this.rive = rive
      this.active = rive
      rive.mount(el, pack)
      return
    }
    this.useDom()
  }

  play(action: ActionId): void {
    this.lastAction = action
    this.active?.play(action)
  }

  setEmotion(emotion: string): void {
    this.lastEmotion = emotion
    this.active?.setEmotion(emotion)
  }

  setLipLevel(level: number): void {
    this.lastLip = level
    this.active?.setLipLevel?.(level)
  }

  destroy(): void {
    this.active?.destroy()
    this.active = null
    this.rive = null
    this.el = null
    this.pack = null
  }

  private fallback(error: Error): void {
    if (!this.el || !this.pack) return
    this.rive?.destroy()
    this.rive = null
    this.options.onFallback?.(error)
    this.useDom()
  }

  private useDom(): void {
    if (!this.el || !this.pack) return
    if (this.el.querySelector('.puck-rive-canvas')) {
      this.el.querySelector('.puck-rive-canvas')?.remove()
    }
    delete this.el.dataset.render
    const dom = new DomAvatarBackend()
    this.active = dom
    dom.mount(this.el, this.pack)
    dom.play(this.lastAction)
    dom.setEmotion(this.lastEmotion)
  }
}
