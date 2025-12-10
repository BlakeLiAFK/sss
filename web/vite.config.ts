import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import AutoImport from 'unplugin-auto-import/vite'
import Components from 'unplugin-vue-components/vite'
import { ElementPlusResolver } from 'unplugin-vue-components/resolvers'

export default defineConfig({
  plugins: [
    vue(),
    // Element Plus 按需自动导入
    AutoImport({
      resolvers: [ElementPlusResolver()],
      dts: 'src/auto-imports.d.ts'
    }),
    Components({
      resolvers: [ElementPlusResolver()],
      dts: 'src/components.d.ts'
    })
  ],
  server: {
    port: 3000
  },
  build: {
    outDir: '../data/static',
    emptyOutDir: true,
    assetsDir: 'assets',
    // 分包策略，减少单个文件大小
    rollupOptions: {
      output: {
        manualChunks: {
          // Vue 核心
          'vue-vendor': ['vue', 'vue-router', 'pinia', 'vue-i18n'],
          // Element Plus 图标单独打包
          'el-icons': ['@element-plus/icons-vue'],
          // Leaflet 地图
          'leaflet': ['leaflet']
        }
      }
    },
    // 调高警告阈值
    chunkSizeWarningLimit: 500
  }
})
