<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { TooltipProvider } from '@/components/ui/tooltip'
import { bridge } from '../bridge'
import AppNav from './AppNav.vue'

const route = useRoute()
const router = useRouter()
let unsubscribe: (() => void) | undefined

const shellClass = computed(() => {
  if (route.name === 'settings') return 'app-shell--settings'
  if (route.name === 'history') return 'app-shell--history'
  return 'app-shell--chat'
})

onMounted(() => {
  unsubscribe = bridge.onNavigateRoute((event) => {
    if (event.route === '/chat' && route.name === 'chat') return
    void router.push(event.route)
  })
})

onBeforeUnmount(() => unsubscribe?.())
</script>

<template>
  <TooltipProvider>
    <div class="app-shell flex h-full w-full" :class="shellClass">
      <AppNav />
      <div class="min-w-0 flex-1">
        <RouterView v-slot="{ Component }">
          <KeepAlive>
            <component :is="Component" />
          </KeepAlive>
        </RouterView>
      </div>
    </div>
  </TooltipProvider>
</template>
