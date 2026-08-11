import { expect, test } from '@playwright/test'

test.beforeEach(async ({ page }) => {
  await page.route('**/static/logo.svg', route => route.fulfill({ contentType: 'image/svg+xml', body: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><path fill="#2563eb" d="M4 4h24v24H4z"/></svg>' }))
  await page.route('**/static/theme-server-status/**', route => route.fulfill({ status: 204 }))
  await page.routeWebSocket('**/ws/v2/public/runtime', () => {})
  await page.route('**/api/v2/public/bootstrap', route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ data: { brand: '三太子监控', locale: 'zh-CN', version: 'test', logo_url: '/static/logo.svg', primary_color: '#2563eb', requires_view_password: false, view_password_verified: true, show_availability: true, authenticated: false } }) }))
  await page.route('**/api/v2/public/servers', route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ data: [{ id: 1, name: 'primary-a', tag: 'core', display_index: 1, hide_for_guest: false, enable_ddns: false, online: true, host: { Platform: 'linux', CountryCode: 'CN' }, state: { CPU: 8, MemUsed: 1024, MemTotal: 2048 }, telemetry: { host: 'online', connectivity: 'full', available: true, coverage: 'full' } }], meta: { total: 1 } }) }))
})

test('renders grouped live status and changes theme in place', async ({ page }) => {
  await page.goto('/')
  await expect(page.locator('.status-brand')).toBeVisible()
  await expect(page.getByText('primary-a')).toBeVisible()
  await expect(page.getByText('core')).toBeVisible()
  const before = page.url()
  await page.getByRole('button', { name: '浅色' }).click()
  expect(page.url()).toBe(before)
  await expect(page.locator('html')).toHaveAttribute('data-theme', /light|dark/)
})

test('redirects protected sites to the Vue password screen', async ({ page }) => {
  let verified = false
  await page.route('**/api/v2/public/bootstrap', route => route.fulfill({ contentType: 'application/json', body: JSON.stringify({ data: { brand: '三太子监控', locale: 'zh-CN', version: 'test', csrf_token: 'public-csrf', logo_url: '/static/logo.svg', requires_view_password: true, view_password_verified: false, show_availability: true, authenticated: false } }) }))
  await page.route('**/api/v2/public/view-password/session', route => {
    verified = route.request().headers()['x-csrf-token'] === 'public-csrf'
    return route.fulfill({ status: 204 })
  })
  await page.goto('/')
  await expect(page).toHaveURL(/\/view-password$/)
  await expect(page.getByRole('heading', { name: '访问受保护' })).toBeVisible()
  await page.getByLabel('密码').fill('view-password')
  await page.getByRole('button', { name: '验证' }).click()
  await expect.poll(() => verified).toBe(true)
})
