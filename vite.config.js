import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'path'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
      '@components': path.resolve(__dirname, './src/components'),
      '@assets': path.resolve(__dirname, './src/assets'),
      '@stores': path.resolve(__dirname, './src/stores'),
      '@views': path.resolve(__dirname, './src/views'),
      '@utils': path.resolve(__dirname, './src/utils')
    }
  },
  server: {
    port: 8080,
    host: '0.0.0.0',
    proxy: {
      '/api': {
        target: 'http://rpi5.lan:8000',
        changeOrigin: true,
        ws: true,
        secure: false,
        timeout: 30000
      },
      '/ws': {
        target: 'ws://rpi5.lan:8000',
        changeOrigin: true,
        ws: true,
        secure: false
      }
    }
  },
  build: {
    target: 'es2015',
    outDir: 'dist',
    assetsDir: 'assets',
    assetsInlineLimit: 4096,
    cssCodeSplit: true,
    sourcemap: false,
    minify: 'terser',
    terserOptions: {
      compress: {
        drop_debugger: true,
        // 保留 console.error 和 console.warn 用于生产环境排错
        pure_funcs: ['console.log', 'console.debug']
      },
      format: {
        comments: false
      }
    },
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes('node_modules')) {
            if (id.includes('leaflet')) return 'leaflet-vendor'
            if (id.includes('vue')) return 'vue-vendor'
            if (id.includes('pinia')) return 'pinia-vendor'
            return 'vendor'
          }
        },
        entryFileNames: 'assets/[name].[hash].js',
        chunkFileNames: 'assets/[name].[hash].js',
        assetFileNames: 'assets/[name].[hash].[ext]'
      }
    }
  },
  esbuild: {
    drop: process.env.NODE_ENV === 'production' ? ['debugger'] : []
  },
  optimizeDeps: {
    include: [
      'vue',
      'vue-router',
      'pinia',
      'axios',
      'leaflet'
    ]
  }
})
