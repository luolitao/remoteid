import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'path'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [
    vue(),
  ],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
      '@components': path.resolve(__dirname, './src/components'),
      '@assets': path.resolve(__dirname, './src/assets'),
      '@stores': path.resolve(__dirname, './src/stores'),
      '@views': path.resolve(__dirname, './src/views'),
    }
  },
  server: {
    port: 8080,
    host: true,
    // 开发服务器代理配置（解决跨域）
    proxy: {
      '/api': {
        target: 'http://raspberrypi.local:8000', // 后端 API 地址
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api/, ''),
        ws: true, // 代理 WebSocket
        secure: false, // 不验证 SSL 证书
        timeout: 30000, // 30秒超时
      },
      '/ws': {
        target: 'ws://raspberrypi.local:8000',
        changeOrigin: true,
        ws: true,
        secure: false,
      }
    }
  },
  build: {
    // 优化生产构建
    target: 'es2015',
    outDir: 'dist',
    assetsDir: 'assets',
    assetsInlineLimit: 4096, // 4kb 以下的资源内联
    cssCodeSplit: true,
    sourcemap: false, // 生产环境不生成 sourcemap
    minify: 'terser',
    terserOptions: {
      compress: {
        drop_console: true, // 移除 console.log
        drop_debugger: true,
        pure_funcs: ['console.log', 'console.debug'] // 移除特定 console 调用
      },
      format: {
        comments: false // 移除注释
      }
    },
    rollupOptions: {
      output: {
        manualChunks(id) {
          // 拆分第三方库
          if (id.includes('node_modules')) {
            if (id.includes('leaflet')) return 'leaflet-vendor'
            if (id.includes('vue')) return 'vue-vendor'
            if (id.includes('pinia')) return 'pinia-vendor'
            return 'vendor'
          }
        },
        // 为 chunk 添加 hash
        entryFileNames: `assets/[name].[hash].js`,
        chunkFileNames: `assets/[name].[hash].js`,
        assetFileNames: `assets/[name].[hash].[ext]`
      }
    }
  },
  esbuild: {
    drop: process.env.NODE_ENV === 'production' ? ['console', 'debugger'] : []
  },
  optimizeDeps: {
    include: [
      'vue',
      'vue-router',
      'pinia',
      'axios',
      'leaflet',
      'vue3-leaflet',
      'lodash-es'
    ]
  }
})
