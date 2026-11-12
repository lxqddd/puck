import {
  Alignment,
  Fit,
  Layout,
  Rive,
  RuntimeLoader,
  StateMachineInputType,
} from '@rive-app/canvas-lite'
import type { StateMachineInput } from '@rive-app/canvas-lite'
import wasmUrl from '@rive-app/canvas-lite/rive.wasm?url'
import { emotionIndex, riveActionPlan } from './rive-plan.js'
import type { ActionId, AvatarBackend, CharacterPack } from './types.js'

export const riveWasmUrl: string = wasmUrl

export interface RiveBackendOptions {
  wasmUrl?: string
  onError?: (error: Error) => void
}

/**
 * Rive 渲染后端。角色包的 .riv 需提供状态机与以下约定输入（可通过 manifest.rive.inputs 改名）：
 * - trigger：动作 id（wave/nod/hop/blink/happy/sad）
 * - boolean：speaking / thinking / working
 * - number：emotion（pack.emotions 的下标）、lipLevel（0-1）
 */
export class RiveBackend implements AvatarBackend {
  private el: HTMLElement | null = null
  private canvas: HTMLCanvasElement | null = null
  private rive: Rive | null = null
  private inputs: StateMachineInput[] = []
  private pack: CharacterPack | null = null
  private resizeObserver: ResizeObserver | null = null
  private destroyed = false

  private lastAction: ActionId = 'idle'
  private lastEmotion = 'neutral'
  private lipLevel = 0

  constructor(private readonly options: RiveBackendOptions = {}) {
    RuntimeLoader.setWasmUrl(options.wasmUrl ?? riveWasmUrl)
  }

  mount(el: HTMLElement, pack: CharacterPack): void {
    this.el = el
    this.pack = pack
    if (!pack.riveAssetUrl) {
      this.fail(new Error('character pack has no rive asset'))
      return
    }

    el.dataset.render = 'rive'
    const canvas = document.createElement('canvas')
    canvas.className = 'puck-rive-canvas'
    canvas.style.width = '100%'
    canvas.style.height = '100%'
    canvas.style.display = 'block'
    el.appendChild(canvas)
    this.canvas = canvas

    this.resizeObserver = new ResizeObserver(() => this.rive?.resizeDrawingSurfaceToCanvas())
    this.resizeObserver.observe(el)

    this.rive = new Rive({
      src: pack.riveAssetUrl,
      canvas,
      artboard: pack.rive?.artboard || undefined,
      stateMachine: pack.rive?.stateMachine || undefined,
      autoplay: true,
      layout: new Layout({ fit: Fit.Contain, alignment: Alignment.Center }),
      onLoad: () => this.handleLoad(),
      onLoadError: (error) => this.fail(new Error(String(error))),
    })
  }

  play(action: ActionId): void {
    this.lastAction = action
    if (this.rive) this.applyAction(action)
  }

  setEmotion(emotion: string): void {
    this.lastEmotion = emotion
    if (this.rive) this.applyEmotion(emotion)
  }

  setLipLevel(level: number): void {
    this.lipLevel = level
    this.setNumberInput(this.inputName('lipLevel'), level)
  }

  destroy(): void {
    this.destroyed = true
    this.resizeObserver?.disconnect()
    this.resizeObserver = null
    this.rive?.cleanup()
    this.rive = null
    this.canvas?.remove()
    this.canvas = null
    if (this.el) delete this.el.dataset.render
    this.el = null
  }

  private handleLoad(): void {
    if (this.destroyed || !this.rive) return
    this.rive.resizeDrawingSurfaceToCanvas()
    const stateMachine = this.pack?.rive?.stateMachine
    this.inputs = (stateMachine ? this.rive.stateMachineInputs(stateMachine) : undefined) ?? []
    this.applyEmotion(this.lastEmotion)
    this.applyAction(this.lastAction)
    this.setNumberInput(this.inputName('lipLevel'), this.lipLevel)
  }

  private applyAction(action: ActionId): void {
    const plan = riveActionPlan(action)
    this.setBooleanInput(this.inputName('speaking'), plan.bools.speaking)
    this.setBooleanInput(this.inputName('thinking'), plan.bools.thinking)
    this.setBooleanInput(this.inputName('working'), plan.bools.working)
    if (plan.trigger) {
      this.fireTrigger(plan.trigger)
    }
  }

  private applyEmotion(emotion: string): void {
    if (!this.pack) return
    const index = emotionIndex(emotion, this.pack.emotions)
    if (index < 0) return
    this.setNumberInput(this.inputName('emotion'), index)
  }

  private inputName(key: 'emotion' | 'lipLevel' | 'speaking' | 'thinking' | 'working'): string {
    return this.pack?.rive?.inputs?.[key] ?? key
  }

  private input(name: string, type: StateMachineInputType): StateMachineInput | undefined {
    return this.inputs.find((entry) => entry.name === name && entry.type === type)
  }

  private setBooleanInput(name: string, value: boolean): void {
    const entry = this.input(name, StateMachineInputType.Boolean)
    if (entry) entry.value = value
  }

  private setNumberInput(name: string, value: number): void {
    const entry = this.input(name, StateMachineInputType.Number)
    if (entry) entry.value = value
  }

  private fireTrigger(name: string): void {
    this.input(name, StateMachineInputType.Trigger)?.fire()
  }

  private fail(error: Error): void {
    if (this.destroyed) return
    this.destroyed = true
    this.resizeObserver?.disconnect()
    this.resizeObserver = null
    this.rive?.cleanup()
    this.rive = null
    this.canvas?.remove()
    this.canvas = null
    if (this.el) delete this.el.dataset.render
    this.options.onError?.(error)
  }
}
