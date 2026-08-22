import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { VitePWA } from 'vite-plugin-pwa'

export default defineConfig({
  plugins: [
    vue(),
    VitePWA({
      registerType: 'autoUpdate',
      selfDestroying: true,
      manifest: {
        name: '随渡 Suidu',
        short_name: '随渡',
        description: '跨端内容中转与轻量记事',
        theme_color: '#1f6feb',
        background_color: '#f5f7fb',
        display: 'standalone',
        start_url: '/',
        icons: [],
      },
    }),
  ],
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
})
