import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  testDir: './web/e2e',
  fullyParallel: true,
  retries: process.env.CI ? 2 : 0,
  reporter: process.env.CI ? [['html', { open: 'never' }], ['github']] : 'list',
  use: { trace: 'retain-on-failure', screenshot: 'only-on-failure' },
  webServer: [
    {
      command: 'pnpm --filter @santaizi/admin dev --host 127.0.0.1 --port 4173',
      url: 'http://127.0.0.1:4173/admin/',
      reuseExistingServer: !process.env.CI,
    },
    {
      command: 'pnpm --filter @santaizi/status dev --host 127.0.0.1 --port 4174',
      url: 'http://127.0.0.1:4174/',
      reuseExistingServer: !process.env.CI,
    },
  ],
  projects: [
    { name: 'admin-desktop', testMatch: /admin\.spec\.ts/, use: { ...devices['Desktop Chrome'], baseURL: 'http://127.0.0.1:4173' } },
    { name: 'admin-mobile', testMatch: /admin\.spec\.ts/, use: { ...devices['iPhone 13'], browserName: 'chromium', baseURL: 'http://127.0.0.1:4173' } },
    { name: 'status-desktop', testMatch: /status\.spec\.ts/, use: { ...devices['Desktop Chrome'], baseURL: 'http://127.0.0.1:4174' } },
    { name: 'status-mobile', testMatch: /status\.spec\.ts/, use: { ...devices['iPhone 13'], browserName: 'chromium', baseURL: 'http://127.0.0.1:4174' } },
  ],
})
