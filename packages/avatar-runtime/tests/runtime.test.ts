import { describe, expect, it } from 'vitest'
import { AvatarRuntime, buildCharacterPack } from '../src/index.js'
import { riveActionPlan } from '../src/rive-plan.js'
import type { ActionId, AvatarBackend, CharacterPack } from '../src/index.js'

const manifest = {
  id: 'test',
  displayName: 'Test',
  window: { width: 360, height: 420 },
  actions: ['idle', 'blink', 'wave', 'nod', 'hop', 'think', 'work', 'speak', 'happy', 'sad'],
  emotions: ['neutral', 'happy', 'sad'],
}

const mapping = {
  gestures: { wave: 'wave', nod: 'nod' },
  emotions: { happy: 'happy', neutral: 'neutral' },
  idlePool: ['wave', 'nod'],
}

function createPack(): CharacterPack {
  return buildCharacterPack(manifest, mapping)
}

function createBackend(): AvatarBackend & { played: ActionId[]; emotions: string[] } {
  const played: ActionId[] = []
  const emotions: string[] = []
  return {
    played,
    emotions,
    mount() {},
    play(action) {
      played.push(action)
    },
    setEmotion(emotion) {
      emotions.push(emotion)
    },
    destroy() {},
  }
}

describe('AvatarRuntime', () => {
  it('drops lower-priority actions while a higher-priority one is active', () => {
    let t = 0
    const backend = createBackend()
    const runtime = new AvatarRuntime({ pack: createPack(), backend, now: () => t })

    expect(runtime.play('hop')).toBe(true)
    expect(runtime.play('wave')).toBe(false)
    expect(runtime.current.action).toBe('hop')

    t = 1000
    runtime.tick(t)
    expect(runtime.play('wave')).toBe(true)
  })

  it('holds thinking until chat state returns to idle', () => {
    const backend = createBackend()
    const runtime = new AvatarRuntime({ pack: createPack(), backend, now: () => 0 })

    runtime.setChatState('thinking')
    expect(runtime.current.action).toBe('think')

    runtime.react('click')
    expect(runtime.current.action).toBe('think')

    runtime.setChatState('idle')
    expect(runtime.current.action).toBe('idle')
    expect(backend.played.at(-1)).toBe('idle')
  })

  it('enforces cooldown', () => {
    let t = 0
    const runtime = new AvatarRuntime({ pack: createPack(), backend: createBackend(), now: () => t })

    expect(runtime.play('blink')).toBe(true)
    t = 300
    runtime.tick(t)
    expect(runtime.play('blink')).toBe(false)
    t = 4000
    expect(runtime.play('blink')).toBe(true)
  })

  it('maps semantic gestures and emotions through the character pack', () => {
    const runtime = new AvatarRuntime({ pack: createPack(), backend: createBackend() })
    runtime.applyGesture('wave')
    expect(runtime.current.action).toBe('wave')
    runtime.applySemanticEmotion('happy')
    expect(runtime.current.emotion).toBe('happy')
    runtime.applySemanticEmotion('unknown')
    expect(runtime.current.emotion).toBe('happy')
  })

  it('forwards clamped lip levels to the backend', () => {
    const levels: number[] = []
    const backend: AvatarBackend = {
      mount() {},
      play() {},
      setEmotion() {},
      setLipLevel(level) {
        levels.push(level)
      },
      destroy() {},
    }
    const runtime = new AvatarRuntime({ pack: createPack(), backend })
    runtime.setLipLevel(1.5)
    runtime.setLipLevel(-1)
    runtime.setLipLevel(0.4)
    expect(levels).toEqual([1, 0, 0.4])
  })
})

describe('Rive integration helpers', () => {
  it('maps hold actions to booleans and one-shot actions to triggers', () => {
    expect(riveActionPlan('speak').bools).toEqual({
      speaking: true,
      thinking: false,
      working: false,
    })
    expect(riveActionPlan('think').bools.thinking).toBe(true)
    expect(riveActionPlan('work').bools.working).toBe(true)
    expect(riveActionPlan('wave').trigger).toBe('wave')
    expect(riveActionPlan('idle').trigger).toBeUndefined()
    expect(riveActionPlan('idle').bools).toEqual({
      speaking: false,
      thinking: false,
      working: false,
    })
  })

  it('only enables Rive mode when an asset url is provided', () => {
    const riveManifest = {
      ...manifest,
      assets: { mode: 'rive' as const, rive: 'assets/rive/character.riv' },
      rive: { stateMachine: 'PuckSM' },
    }
    expect(buildCharacterPack(riveManifest, mapping).renderMode).toBe('static')
    const pack = buildCharacterPack(riveManifest, mapping, { riveAssetUrl: '/character.riv' })
    expect(pack.renderMode).toBe('rive')
    expect(pack.rive?.stateMachine).toBe('PuckSM')
  })
})
