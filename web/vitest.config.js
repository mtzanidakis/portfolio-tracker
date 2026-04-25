import { defineConfig } from 'vitest/config';

// Vitest 4 transforms JSX with oxc (Vite 7's default). Pointing it at
// preact/jsx-runtime means <div/> compiles to the same shape the
// production bundle (esbuild via build.mjs) emits — no React shim needed.
export default defineConfig({
  oxc: {
    jsx: {
      runtime: 'automatic',
      importSource: 'preact',
    },
  },
  test: {
    environment: 'happy-dom',
    globals: true,
    setupFiles: ['./test/setup.js'],
    include: ['src/**/*.test.{js,jsx}'],
    coverage: {
      provider: 'v8',
      reporter: ['text', 'html', 'lcov'],
      include: ['src/**'],
      exclude: ['src/**/*.test.{js,jsx}', 'src/main.jsx'],
    },
  },
});
