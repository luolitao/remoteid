// vite.config.js
export default defineConfig({
  server: {
    proxy: {
      // 代理所有 /api 请求到后端
      '/api': {
        target: 'http://localhost:8000',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api/, ''),
        secure: false,
        ws: true,
        timeout: 30000
      },
      // 代理 WebSocket
      '/ws': {
        target: 'ws://localhost:8000',
        changeOrigin: true,
        ws: true,
        secure: false
      }
    }
  }
})
