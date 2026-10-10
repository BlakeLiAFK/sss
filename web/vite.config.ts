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
        manualChunks(id) {
          if (!id.includes('/node_modules/')) return
          if (id.includes('/@element-plus/icons-vue/')) return 'el-icons'
          if (id.includes('/leaflet/')) return 'leaflet'
          if (/\/(vue|@vue|vue-router|pinia|vue-i18n|@intlify)\//.test(id)) return 'vue-vendor'
        }
      }
    },
    // 调高警告阈值
    chunkSizeWarningLimit: 500
  }
})
