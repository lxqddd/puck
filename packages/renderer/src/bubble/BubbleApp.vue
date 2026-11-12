<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import type { BubbleSide } from '@puck/contract'
import { bridge } from '../bridge'

const text = ref('')
const side = ref<BubbleSide>('right')
const visible = ref(false)

let unsubscribe: (() => void) | undefined

function apply(nextText: string, nextSide: BubbleSide) {
  if (!nextText) {
    visible.value = false
    return
  }
  text.value = nextText
  side.value = nextSide === 'left' ? 'left' : 'right'
  visible.value = true
}

onMounted(async () => {
  unsubscribe = bridge.onBubbleShow((event) => apply(event.text, event.side))

  try {
    const state = await bridge.getBubbleState()
    if (state?.text) apply(state.text, state.side)
  } catch {
    // 页面初始状态只是兜底，事件订阅已能接收后续气泡。
  }
})

onBeforeUnmount(() => unsubscribe?.())
</script>

<template>
  <div
    class="bubble-stage"
    :class="side === 'left' ? 'bubble-stage-left' : 'bubble-stage-right'"
  >
    <transition name="bubble-pop">
      <div
        v-if="visible"
        class="puck-bubble"
        :class="side === 'left' ? 'puck-bubble-left' : 'puck-bubble-right'"
      >
        <span class="puck-bubble-text">{{ text }}</span>
      </div>
    </transition>
  </div>
</template>
