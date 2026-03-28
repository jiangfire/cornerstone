import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import vueJsx from '@vitejs/plugin-vue-jsx'
import vueDevTools from 'vite-plugin-vue-devtools'

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    vue(),
    vueJsx(),
    vueDevTools(),
  ],
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes('node_modules')) {
            return undefined
          }

          if (id.includes('/vue/') || id.includes('/vue-router/') || id.includes('/pinia/')) {
            return 'framework'
          }

          const isElementPlus =
            id.includes('/element-plus/') ||
            id.includes('/@element-plus/') ||
            id.includes('/async-validator/') ||
            id.includes('/dayjs/')

          if (isElementPlus) {
            if (
              id.includes('/async-validator/') ||
              id.includes('/dayjs/') ||
              id.includes('/date-picker') ||
              id.includes('/form') ||
              id.includes('/input') ||
              id.includes('/select') ||
              id.includes('/option') ||
              id.includes('/checkbox') ||
              id.includes('/switch') ||
              id.includes('/upload') ||
              id.includes('/tabs')
            ) {
              return 'element-plus-form'
            }

            if (
              id.includes('/table') ||
              id.includes('/pagination') ||
              id.includes('/timeline') ||
              id.includes('/tag') ||
              id.includes('/empty') ||
              id.includes('/progress') ||
              id.includes('/result') ||
              id.includes('/card') ||
              id.includes('/row') ||
              id.includes('/col') ||
              id.includes('/avatar')
            ) {
              return 'element-plus-data'
            }

            return 'element-plus-shell'
          }

          return 'vendor'
        },
      },
    },
  },
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
})
