import preact from '@preact/preset-vite'
import { defineConfig } from 'vite'

const outDir = process.env.MML_FRONTEND_DIST_DIR ?? '../internal/frontend/assets/dist'

export default defineConfig({
  plugins: [preact()],
  resolve: {
    alias: {
      react: 'preact/compat',
      'react-dom': 'preact/compat'
    }
  },
  test: {
    environment: 'node',
    setupFiles: './src/test/setup.ts',
    coverage: {
      provider: 'v8',
      reporter: ['text', 'html']
    }
  },
  build: {
    outDir,
    emptyOutDir: true
  }
})
