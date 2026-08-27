import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vitest/config'

// Unit tests only. Playwright owns frontend/e2e/**; keeping `include` scoped to
// src/ stops the two runners from picking up each other's *.spec.ts files.
export default defineConfig({
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    }
  },
  test: {
    include: ['src/**/*.spec.ts'],
    environment: 'node'
  }
})
