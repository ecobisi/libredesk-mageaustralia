import { fileURLToPath } from 'url'
import { createRequire } from 'module'
import path from 'path'
import autoprefixer from 'autoprefixer'
import tailwind from 'tailwindcss'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { overrideResolver } from './vite-plugins/override-resolver.js'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const require = createRequire(import.meta.url)

export default defineConfig(({ mode, command }) => {
  const isWidget = mode === 'widget'
  const appPath = isWidget ? 'apps/widget' : 'apps/main'

  // Load shared tailwind config but scope content to current app only,
  // so each app's CSS bundle doesn't include unused classes from the other.
  const tailwindConfig = require('./tailwind.config.cjs')
  const scopedContent = [
    `./apps/${isWidget ? 'widget' : 'main'}/src/**/*.{js,ts,vue}`,
    './shared-ui/**/*.{js,ts,vue}',
  ]

  return {
    base: isWidget && command === 'build' ? '/widget/' : '/',
    css: {
      preprocessorOptions: {
        scss: {
          api: 'modern',
        },
      },
      postcss: {
        plugins: [tailwind({ ...tailwindConfig, content: scopedContent }), autoprefixer()],
      },
    },
    root: path.resolve(__dirname, appPath),
    publicDir: path.resolve(__dirname, 'public'),
    // Separate cache per app to avoid stale/conflicting caches.
    cacheDir: path.resolve(__dirname, `node_modules/.vite-${isWidget ? 'widget' : 'main'}`),
    server: (() => {
      // LIBREDESK_API_PORT lets local devs point the proxy at a non-default
      // backend (e.g. a staging container on 9001) without editing this file.
      // Defaults to 9000 — the same port the docker-compose.yml app uses.
      const apiPort = process.env.LIBREDESK_API_PORT || '9000'
      const httpTarget = `http://127.0.0.1:${apiPort}`
      const wsTarget = `ws://127.0.0.1:${apiPort}`
      return {
        cors: { origin: "*" },
        // Allow access to parent dir so shared-ui imports work in dev.
        fs: {
          allow: [path.resolve(__dirname)],
        },
        port: isWidget ? 8001 : 8000,
        proxy: {
          '/api':        { target: httpTarget, changeOrigin: true },
          '/widget.js':  { target: httpTarget, changeOrigin: true },
          '/logout':     { target: httpTarget, changeOrigin: true },
          '/uploads':    { target: httpTarget, changeOrigin: true },
          '/ws':         { target: wsTarget,   ws: true, changeOrigin: true },
          '/widget/ws':  { target: wsTarget,   ws: true, changeOrigin: true },
        },
      }
    })(),
    build: {
      outDir: isWidget
        ? path.resolve(__dirname, 'dist/widget')
        : path.resolve(__dirname, 'dist/main'),
      emptyOutDir: true,
      chunkSizeWarningLimit: 600,
      rollupOptions: {
        output: {
          manualChunks: {
            'vue-vendor': ['vue', 'vue-router', 'pinia'],
            'radix': ['radix-vue', 'reka-ui'],
            'icons': ['lucide-vue-next', '@radix-icons/vue'],
            'utils': ['@vueuse/core', 'clsx', 'tailwind-merge', 'class-variance-authority'],
            'forms': ['vee-validate', '@vee-validate/zod', 'zod'],
            'misc': ['axios', 'date-fns', 'mitt', 'qs', 'vue-i18n'],
            // Main-app-only chunks - widget doesn't use these libraries.
            ...(!isWidget && {
              'charts': ['@unovis/ts', '@unovis/vue'],
              'editor': [
                '@tiptap/vue-3',
                '@tiptap/starter-kit',
                '@tiptap/extension-image',
                '@tiptap/extension-link',
                '@tiptap/extension-placeholder',
                '@tiptap/extension-table',
                '@tiptap/extension-table-cell',
                '@tiptap/extension-table-header',
                '@tiptap/extension-table-row',
              ],
              'codemirror': ['codemirror', '@codemirror/lang-html', '@codemirror/lang-javascript', '@codemirror/theme-one-dark'],
              'table': ['@tanstack/vue-table'],
            }),
          },
        },
      },
    },
    plugins: [
      // Magento-style overrides: any file under apps/<x>/src or shared-ui
      // is transparently replaced by a same-path file under .../overrides/
      // when one exists. Lets us customise upstream files without editing
      // them, so `git pull` from upstream stays conflict-free.
      overrideResolver({
        roots: [
          path.resolve(__dirname, 'apps/main/src'),
          path.resolve(__dirname, 'apps/widget/src'),
          path.resolve(__dirname, 'shared-ui'),
        ],
      }),
      vue(),
    ],
    resolve: {
      alias: {
        '@': path.resolve(__dirname, `${appPath}/src`),
        '@main': path.resolve(__dirname, 'apps/main/src'),
        '@widget': path.resolve(__dirname, 'apps/widget/src'),
        '@shared-ui': path.resolve(__dirname, 'shared-ui'),
      },
    },
  }
})
