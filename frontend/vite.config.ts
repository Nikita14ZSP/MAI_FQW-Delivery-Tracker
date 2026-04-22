/// <reference types="vitest/config" />
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';
import path from 'node:path';

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    css: false,
    // Suite grew past ~230 tests; under parallel jsdom forks on slower machines
    // waitFor-based async tests can exceed the 5s default and false-timeout.
    // Raise the ceiling so contention never produces spurious failures.
    // Happy-path tests still resolve in well under a second.
    testTimeout: 15000,
    hookTimeout: 15000,
  },
});
