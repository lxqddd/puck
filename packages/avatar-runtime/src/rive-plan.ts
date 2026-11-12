import type { ActionId } from './types.js'

export interface RiveActionPlan {
  bools: { speaking: boolean; thinking: boolean; working: boolean }
  trigger?: ActionId
}

const HOLD_ACTION_TO_INPUT = {
  speak: 'speaking',
  think: 'thinking',
  work: 'working',
} as const

/** 把调度器的动作翻译成 Rive 状态机输入：保持类动作用 boolean，其余用 trigger。 */
export function riveActionPlan(action: ActionId): RiveActionPlan {
  const plan: RiveActionPlan = {
    bools: { speaking: false, thinking: false, working: false },
  }
  if (action === 'idle') {
    return plan
  }
  const hold = HOLD_ACTION_TO_INPUT[action as keyof typeof HOLD_ACTION_TO_INPUT]
  if (hold) {
    plan.bools[hold] = true
    return plan
  }
  plan.trigger = action
  return plan
}

export function emotionIndex(emotion: string, emotions: string[]): number {
  return emotions.indexOf(emotion)
}
