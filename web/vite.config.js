import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'path'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [vue()],
  base: './',
  resolve: {
    alias: {
      '@': path.resolve(__dirname, 'src')
    }
  },
  root: './',
  build: {
    emptyOutDir: true,
    chunkSizeWarningLimit: 1600,
    rollupOptions: {
      output: {
        manualChunks: {
          'vendor': ['vue', 'vue-router', 'pinia'],
          'ant-design': ['ant-design-vue', '@ant-design/icons-vue']
        }
      }
    }
  },
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:9100',
        changeOrigin: true
      }
    }
  }
}) 