import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './tests',
  outputDir: './build/playwright-results',
  reporter: [['list'], ['json', { outputFile: './build/playwright-report.json' }]],
  workers: 1,
  use: { browserName: 'chromium', baseURL: process.env.POC_BASE_URL, trace: 'retain-on-failure' },
  projects: [
    { name: 'desktop', use: { viewport: { width: 1280, height: 900 } } },
    { name: 'mobile', use: { viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true } },
  ],
})
