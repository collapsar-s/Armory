import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// Wails 会把 frontend/dist 内嵌进可执行文件（go:embed），
// 因此构建产物固定输出到 frontend/dist，且使用相对 base 以便从内嵌 FS 加载。
export default defineConfig({
  plugins: [vue()],
  base: './',
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    assetsDir: 'assets',
  },
  server: {
    host: '127.0.0.1',
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
  },
})
