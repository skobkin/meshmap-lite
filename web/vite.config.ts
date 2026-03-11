import { defineConfig } from 'vite'
import preact from '@preact/preset-vite'

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
    setupFiles: './src/test/setup.ts'
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true
  }
})
