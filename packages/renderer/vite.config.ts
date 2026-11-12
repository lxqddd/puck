import { fileURLToPath } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import wails from '@wailsio/runtime/plugins/vite'

export default defineConfig({
  plugins: [vue(), wails('./src/bindings'), tailwindcss()],
  build: {
    outDir: fileURLToPath(new URL('../../apps/desktop-go/frontend/dist', import.meta.url)),
    emptyOutDir: true,
    rollupOptions: {
      input: {
        index: fileURLToPath(new URL('index.html', import.meta.url)),
        app: fileURLToPath(new URL('app.html', import.meta.url)),
        mascot: fileURLToPath(new URL('mascot.html', import.meta.url)),
        bubble: fileURLToPath(new URL('bubble.html', import.meta.url)),
      },
    },
  },
  server: {
    // Wails 开发模式的应用内资源代理强制用 IPv4 连接 localhost，
    // 而默认的 localhost 绑定在本机会只监听 [::1]，导致所有页面请求 502。
    host: '127.0.0.1',
    port: 34115,
    strictPort: true,
    fs: {
      allow: [fileURLToPath(new URL('../..', import.meta.url))],
    },
  },
})
