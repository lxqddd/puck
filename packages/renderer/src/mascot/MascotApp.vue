<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { AvatarRuntime, DomAvatarBackend } from '@puck/avatar-runtime'
import type { AvatarBackend } from '@puck/avatar-runtime'
import type { ChatState } from '@puck/contract'
import { bridge } from '../bridge'
import { characterManifest, characterPack, persona } from '../character'

const stage = ref<HTMLElement | null>(null)
const bubble = ref('')
const bubbleVisible = ref(false)
const scale = ref(1)

let runtime: AvatarRuntime | null = null
let tickTimer: number | undefined
let bubbleTimer: number | undefined
let lipTimer: number | undefined
let streamed = ''
const unsubscribers: Array<() => void> = []

interface DragState {
  startScreenX: number
  startScreenY: number
  startWinX: number
  startWinY: number
}
let dragState: DragState | null = null
let dragged = false

function randomFrom(items: string[], fallback: string): string {
  if (items.length === 0) return fallback
  return items[Math.floor(Math.random() * items.length)] ?? fallback
}

function showBubble(text: string, ms: number) {
  bubble.value = text
  bubbleVisible.value = true
  window.clearTimeout(bubbleTimer)
  bubbleTimer = window.setTimeout(() => {
    bubbleVisible.value = false
  }, ms)
}

function reportHitRegion(attempt = 0) {
  const area = characterManifest.hitAreas?.[0]
  if (!area) return
  const current = scale.value
  try {
    void bridge
      .setHitRegion(
        {
          x: Math.round(area.x * current),
          y: Math.round(area.y * current),
          width: Math.round(area.width * current),
          height: Math.round(area.height * current),
        },
        current,
      )
      .catch(() => retryHitRegion(attempt))
  } catch {
    retryHitRegion(attempt)
  }
}

function retryHitRegion(attempt: number) {
  if (attempt >= 8) return
  window.setTimeout(() => reportHitRegion(attempt + 1), 300)
}

function startLipSync() {
  stopLipSync()
  let phase = 0
  lipTimer = window.setInterval(() => {
    phase += 0.35
    const wave = (Math.sin(phase) + 1) / 2
    runtime?.setLipLevel(0.15 + wave * 0.75 * (0.6 + Math.random() * 0.4))
  }, 70)
}

function stopLipSync() {
  if (lipTimer !== undefined) {
    window.clearInterval(lipTimer)
    lipTimer = undefined
  }
  runtime?.setLipLevel(0)
}

const onResize = () => reportHitRegion()

function applyScale(next: number) {
  scale.value = next
  reportHitRegion()
}

async function loadScale(attempt = 0) {
  try {
    applyScale(await bridge.getScale())
  } catch {
    if (attempt < 6) {
      window.setTimeout(() => void loadScale(attempt + 1), 200 * 2 ** attempt)
      return
    }
    applyScale(1)
  }
}

let wheelAccum = 0
let wheelLockedUntil = 0

const onWheel = (event: WheelEvent) => {
  if (!event.metaKey && !event.ctrlKey) return
  event.preventDefault()
  if (dragState || event.deltaY === 0) return

  if (event.deltaMode === 1) wheelAccum += event.deltaY * 16
  else if (event.deltaMode === 2) wheelAccum += event.deltaY * 100
  else wheelAccum += event.deltaY

  const now = performance.now()
  if (now < wheelLockedUntil || Math.abs(wheelAccum) < 1) return

  const delta = wheelAccum < 0 ? 0.1 : -0.1
  wheelAccum = 0
  wheelLockedUntil = now + 120
  void bridge.scaleBy(delta).then(applyScale).catch(() => {})
}

function onClick() {
  if (dragged) {
    dragged = false
    return
  }
  runtime?.react('click')
  showBubble(randomFrom(persona.clickLines, '在呢。'), 2600)
}

function onDoubleClick() {
  runtime?.react('double')
  void bridge.openChat()
}

async function onPointerDown(event: PointerEvent) {
  if (event.button !== 0) return
  dragged = false
  void bridge.setDragging(true)
  const position = await bridge.getPosition()
  dragState = {
    startScreenX: event.screenX,
    startScreenY: event.screenY,
    startWinX: position.x,
    startWinY: position.y,
  }
  stage.value?.setPointerCapture(event.pointerId)
}

function onPointerMove(event: PointerEvent) {
  if (!dragState) return
  const dx = event.screenX - dragState.startScreenX
  const dy = event.screenY - dragState.startScreenY
  if (Math.abs(dx) + Math.abs(dy) > 3) dragged = true
  if (dragged) {
    void bridge.setPosition(
      Math.round(dragState.startWinX + dx),
      Math.round(dragState.startWinY + dy),
    )
  }
}

function onPointerUp(event: PointerEvent) {
  dragState = null
  void bridge.setDragging(false)
  stage.value?.releasePointerCapture?.(event.pointerId)
}

async function createBackend(): Promise<AvatarBackend> {
  if (characterPack.renderMode !== 'rive') {
    return new DomAvatarBackend()
  }
  try {
    const { ResilientAvatarBackend, riveWasmUrl } = await import('@puck/avatar-runtime/rive')
    return new ResilientAvatarBackend({
      wasmUrl: riveWasmUrl,
      onFallback: () => showBubble('动画加载失败，已切回静态形象', 3200),
    })
  } catch {
    return new DomAvatarBackend()
  }
}

onMounted(async () => {
  const backend = await createBackend()
  runtime = new AvatarRuntime({ pack: characterPack, backend })
  if (stage.value) runtime.mount(stage.value)
  tickTimer = window.setInterval(() => runtime?.tick(), 250)
  window.addEventListener('resize', onResize)
  window.addEventListener('wheel', onWheel, { passive: false })
  unsubscribers.push(bridge.onMascotScale((event) => applyScale(event.scale)))
  void loadScale()

  unsubscribers.push(
    bridge.onChatState((event) => {
      runtime?.setChatState(event.state as ChatState)
      if (event.state === 'speaking') startLipSync()
      else stopLipSync()
      if (event.state === 'thinking') showBubble('让我想想…', 60_000)
      if (event.state === 'idle') {
        window.clearTimeout(bubbleTimer)
        bubbleTimer = window.setTimeout(() => {
          bubbleVisible.value = false
        }, 2600)
      }
    }),
    bridge.onChatDelta((event) => {
      streamed += event.text
      const preview = streamed.length > 72 ? `${streamed.slice(0, 72)}…` : streamed
      showBubble(preview, 60_000)
    }),
    bridge.onChatDone(() => {
      streamed = ''
      showBubble(bubble.value, 4200)
      runtime?.setEmotion('happy')
      window.setTimeout(() => runtime?.setEmotion('neutral'), 1800)
    }),
    bridge.onChatError(() => {
      streamed = ''
      runtime?.setEmotion('sad')
      showBubble('唔，出了点问题。', 3600)
      window.setTimeout(() => runtime?.setEmotion('neutral'), 2200)
    }),
  )

  runtime.play('wave')
  window.setTimeout(() => {
    showBubble(randomFrom(persona.greetings, '我在呢。'), 3600)
  }, 350)
})

onBeforeUnmount(() => {
  window.clearInterval(tickTimer)
  window.clearTimeout(bubbleTimer)
  stopLipSync()
  window.removeEventListener('resize', onResize)
  window.removeEventListener('wheel', onWheel)
  for (const unsubscribe of unsubscribers) unsubscribe()
  runtime?.destroy()
})
</script>

<template>
  <div class="mascot-window">
    <div
      ref="stage"
      class="avatar-stage"
      @pointerdown="onPointerDown"
      @pointermove="onPointerMove"
      @pointerup="onPointerUp"
      @pointercancel="onPointerUp"
      @click="onClick"
      @dblclick="onDoubleClick"
    >
      <svg viewBox="0 0 240 300" class="puck" aria-label="Puck，桌角的小精灵">
        <defs>
          <linearGradient id="puck-body" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stop-color="#86eccb" />
            <stop offset="100%" stop-color="#35bd98" />
          </linearGradient>
          <radialGradient id="puck-glow" cx="50%" cy="50%" r="50%">
            <stop offset="0%" stop-color="#ffe9a8" stop-opacity="0.95" />
            <stop offset="100%" stop-color="#ffe9a8" stop-opacity="0" />
          </radialGradient>
        </defs>

        <ellipse class="shadow" cx="120" cy="276" rx="58" ry="11" fill="rgba(29, 107, 87, 0.16)" />

        <g class="body-group">
          <line
            class="antenna"
            x1="120"
            y1="66"
            x2="120"
            y2="30"
            stroke="#2b8f75"
            stroke-width="6"
            stroke-linecap="round"
          />
          <circle class="antenna-tip" cx="120" cy="26" r="9" fill="#ffd166" />
          <circle class="glow" cx="120" cy="26" r="24" fill="url(#puck-glow)" />

          <path
            class="body"
            d="M120 54c43 0 74 29 74 74v56c0 42-33 73-74 73s-74-31-74-73v-56c0-45 31-74 74-74z"
            fill="url(#puck-body)"
          />
          <ellipse class="belly" cx="120" cy="200" rx="45" ry="36" fill="#e8fff7" opacity="0.6" />

          <g class="face">
            <g class="eye eye-left">
              <ellipse cx="93" cy="143" rx="10" ry="14" fill="#143f35" />
              <circle cx="96" cy="138" r="3.5" fill="#ffffff" />
            </g>
            <g class="eye eye-right">
              <ellipse cx="147" cy="143" rx="10" ry="14" fill="#143f35" />
              <circle cx="150" cy="138" r="3.5" fill="#ffffff" />
            </g>
            <g class="brows">
              <path d="M80 121c8-7 19-7 27 0" stroke="#1d6b57" stroke-width="5" fill="none" stroke-linecap="round" />
              <path d="M133 121c8-7 19-7 27 0" stroke="#1d6b57" stroke-width="5" fill="none" stroke-linecap="round" />
            </g>
            <ellipse class="blush blush-left" cx="74" cy="168" rx="11" ry="6" fill="#ff9db0" opacity="0.55" />
            <ellipse class="blush blush-right" cx="166" cy="168" rx="11" ry="6" fill="#ff9db0" opacity="0.55" />
            <path
              class="mouth"
              d="M108 171c8 7 16 7 24 0"
              stroke="#143f35"
              stroke-width="5"
              fill="none"
              stroke-linecap="round"
            />
          </g>

          <g class="arm arm-left">
            <path d="M50 176c-12 12-13 27-6 40" stroke="#35bd98" stroke-width="15" fill="none" stroke-linecap="round" />
          </g>
          <g class="arm arm-right">
            <path d="M190 176c12 12 13 27 6 40" stroke="#35bd98" stroke-width="15" fill="none" stroke-linecap="round" />
          </g>

          <g class="feet">
            <ellipse cx="94" cy="270" rx="18" ry="9" fill="#2b8f75" />
            <ellipse cx="146" cy="270" rx="18" ry="9" fill="#2b8f75" />
          </g>

          <g class="thought-dots">
            <circle cx="196" cy="92" r="4" fill="#9adfc8" />
            <circle cx="206" cy="78" r="5.5" fill="#9adfc8" />
            <circle cx="218" cy="60" r="7" fill="#9adfc8" />
          </g>
        </g>
      </svg>
    </div>

    <transition name="bubble">
      <div v-if="bubbleVisible" class="bubble">{{ bubble }}</div>
    </transition>
  </div>
</template>
