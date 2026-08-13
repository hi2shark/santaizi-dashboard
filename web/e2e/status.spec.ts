import { expect, test, type Page, type Route } from '@playwright/test'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

const list = (data: unknown[] = []) => JSON.stringify({ data, meta: { total: data.length } })
const item = (data: unknown) => JSON.stringify({ data })
const worldGeoJSON = readFileSync(resolve(process.cwd(), 'resource/static/theme-nazhua/maps/world.geo.json'), 'utf8')

const servers = [
  {
    id: 1, name: 'HKG-EDGE', tag: 'HKG', display_index: 30, hide_for_guest: false, enable_ddns: false, online: true,
    host: { Platform: 'linux', CountryCode: 'HK', CPU: 2, Arch: 'amd64', Version: '6.8' },
    state: { CPU: 8.2, MemUsed: 767_557_632, MemTotal: 2_147_483_648, DiskUsed: 6_442_450_944, DiskTotal: 21_474_836_480, Uptime: 2_074_200, NetInSpeed: 5_800, NetOutSpeed: 5_100, NetInTransfer: 98_320_000_000, NetOutTransfer: 63_740_000_000 },
    public_note: { customData: { location: 'HKG', slogan: 'Hong Kong Premium', flag: 'hk' }, billingDataMod: { amount: '109.00CNY', cycle: '月' }, planDataMod: { networkRoute: 'IEPL,电信专线', IPv4: '1', IPv6: '1', trafficType: '2' } },
  },
  {
    id: 2, name: 'SGP-BAGE', tag: 'SGP', display_index: 20, hide_for_guest: false, enable_ddns: false, online: true,
    host: { Platform: 'freebsd', CountryCode: 'SG', CPU: 1, Arch: 'amd64', Version: '14' },
    state: { CPU: 1, MemUsed: 549_453_824, MemTotal: 1_073_741_824, DiskUsed: 5_368_709_120, DiskTotal: 10_737_418_240, Uptime: 11_491_200, NetInSpeed: 22_200, NetOutSpeed: 14_800, NetInTransfer: 512_000_000_000, NetOutTransfer: 251_180_000_000 },
    public_note: { customData: { location: 'SGP', slogan: 'Singapore Edge', flag: 'sg' }, billingDataMod: { amount: '2.59USD', cycle: '月' }, planDataMod: { networkRoute: 'CTCSCI,原生IP', IPv4: '1', IPv6: '1', trafficType: '2' } },
  },
  {
    id: 3, name: 'TYO-OFFLINE', tag: 'JPN', display_index: 10, hide_for_guest: false, enable_ddns: false, online: false,
    host: { Platform: 'linux', CountryCode: 'JP', CPU: 4, Arch: 'arm64', Version: '6.6' },
    state: { CPU: 0, MemUsed: 0, MemTotal: 4_294_967_296, DiskUsed: 0, DiskTotal: 42_949_672_960, Uptime: 0, NetInSpeed: 0, NetOutSpeed: 0, NetInTransfer: 0, NetOutTransfer: 0 },
    public_note: { customData: { location: 'JPN', slogan: 'Maintenance', flag: 'jp' }, billingDataMod: { amount: '4.00USD', cycle: '月' }, planDataMod: { networkRoute: 'BGP', IPv4: '1' } },
  },
]

async function fulfillJSON(route: Route, body: string, status = 200) {
  await route.fulfill({ contentType: 'application/json', body, status })
}

async function useNazhua(page: Page, mode: 'dark' | 'light' = 'dark') {
  await page.addInitScript(({ color }) => {
    localStorage.setItem('santaizi-public-theme', 'nazhua')
    localStorage.setItem('santaizi-status-theme', color)
    localStorage.setItem('santaizi-locale', 'zh-CN')
  }, { color: mode })
}

test.beforeEach(async ({ page }) => {
  await page.route('**/static/logo.svg', route => route.fulfill({ contentType: 'image/svg+xml', body: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><path fill="#2563eb" d="M4 4h24v24H4z"/></svg>' }))
  await page.route('**/static/theme-server-status/**', route => route.fulfill({ status: 204 }))
  await page.route('**/static/theme-nazhua/maps/world.geo.json', route => fulfillJSON(route, worldGeoJSON))
  await page.routeWebSocket('**/ws/v2/public/runtime', () => {})
  await page.route('**/api/v2/public/bootstrap', route => fulfillJSON(route, item({
    brand: '三太子监控', locale: 'zh-CN', version: 'test', logo_url: '/static/logo.svg', primary_color: '#f0cb23',
    requires_view_password: false, view_password_verified: true, show_availability: true, authenticated: true,
    theme: 'nazhua', allow_frontend_theme_switch: true,
  })))
  await page.route('**/api/v2/public/servers', route => fulfillJSON(route, list(servers)))
  await page.route('**/api/v2/public/cycle-transfer**', route => {
    const cycleRows = [
      { policy_id: 1, server_id: 1, name: 'Monthly', direction: 'both', used_bytes: 173_999_036_989, quota_bytes: 1_099_511_627_776, usage_percent: 15.8, status: 'normal' },
      { policy_id: 2, server_id: 2, name: 'Monthly', direction: 'both', used_bytes: 92_997_746_115, quota_bytes: 549_755_813_888, usage_percent: 16.9, status: 'normal' },
    ]
    const serverId = Number(new URL(route.request().url()).searchParams.get('server_id') || 0)
    return fulfillJSON(route, list(serverId ? cycleRows.filter(row => row.server_id === serverId) : cycleRows))
  })
  await page.route('**/api/v2/public/services', route => fulfillJSON(route, list([
    { id: 1, name: 'Public API', current_up: 99, current_down: 1, up: [99, 100, 98, 100], down: [1, 0, 2, 0], avg_delay: 42 },
  ])))
  await page.route('**/api/v2/public/network/*', route => fulfillJSON(route, list([
    { monitor_name: 'ICMP', created_at: ['2026-08-12T12:00:00Z', '2026-08-12T12:05:00Z'], avg_delay: [42, 38] },
  ])))
})

test('renders a complete Nazhua homepage with one shell, map points and cycle-aware cards', async ({ page }) => {
  await useNazhua(page)
  let cycleRequests = 0
  page.on('request', request => {
    if (request.url().includes('/api/v2/public/cycle-transfer')) cycleRequests += 1
  })
  await page.goto('/')

  await expect(page.locator('.nazhua-shell')).toBeVisible()
  await expect(page.locator('.nazhua-header')).toHaveCount(1)
  await expect(page.locator('.status-nav')).toHaveCount(0)
  await expect(page.locator('.nazhua-world-map')).toBeVisible()
  expect(await page.locator('.nazhua-world-map__point').count()).toBeGreaterThanOrEqual(2)
  await expect(page.locator('.nazhua-card')).toHaveCount(3)
  await expect(page.locator('.nazhua-card').first().locator('.traffic strong')).toContainText('861.95')
  await expect.poll(() => cycleRequests).toBe(1)

  const layout = await page.evaluate(() => {
    const map = document.querySelector<HTMLElement>('.nazhua-world-map')!
    const mapImage = map.querySelector<HTMLElement>('.nazhua-world-map__image')!
    const card = document.querySelector<HTMLElement>('.nazhua-card')!
    const fontSizes = [...document.querySelectorAll<HTMLElement>('.nazhua-shell *')]
      .filter(node => getComputedStyle(node).display !== 'none')
      .map(node => Number.parseFloat(getComputedStyle(node).fontSize))
      .filter(Number.isFinite)
    return {
      mapBackground: getComputedStyle(mapImage).backgroundImage,
      mapWidth: map.getBoundingClientRect().width,
      viewportWidth: window.innerWidth,
      cardTop: card.getBoundingClientRect().top,
      minimumFont: Math.min(...fontSizes),
      bodyHeight: document.body.scrollHeight,
    }
  })
  expect(layout.mapBackground).toContain('world-map')
  expect(layout.mapWidth).toBeGreaterThanOrEqual(layout.viewportWidth > 720 ? 940 : 340)
  expect(layout.cardTop).toBeLessThan(760)
  expect(layout.minimumFont).toBeGreaterThanOrEqual(12)
  expect(layout.bodyHeight).toBeGreaterThan(700)
})

test('search opens an AppDialog and details retain the Nazhua shell', async ({ page }) => {
  await useNazhua(page)
  await page.goto('/')
  await page.getByRole('button', { name: '搜索' }).click()
  const dialog = page.getByRole('dialog', { name: '搜索' })
  await expect(dialog).toBeVisible()
  await dialog.getByPlaceholder('搜索服务器名称、分组、系统或国家').fill('SGP')
  await dialog.getByRole('button', { name: /SGP-BAGE/ }).click()
  await expect(page).toHaveURL(/\/server\/2$/)
  await expect(page.locator('.nazhua-header')).toHaveCount(1)
  await expect(page.locator('.nazhua-detail')).toBeVisible()
  await expect(page.getByRole('heading', { name: 'SGP-BAGE' })).toBeVisible()
  await expect(page.getByRole('heading', { name: '周期流量' })).toBeVisible()
  await expect(page.locator('.nazhua-cycle-transfer__item')).toHaveCount(1)
  await expect(page.getByRole('heading', { name: '网络监控' })).toBeVisible()
})

test('function menu keeps service and network pages inside Nazhua and switches shell cleanly', async ({ page }) => {
  await useNazhua(page)
  await page.goto('/')
  await page.getByRole('button', { name: '操作' }).click()
  await page.getByRole('menuitem', { name: /服务状态/ }).click()
  await expect(page).toHaveURL(/\/service$/)
  await expect(page.locator('.nazhua-header')).toHaveCount(1)
  await expect(page.getByText('Public API')).toBeVisible()

  await page.getByRole('button', { name: '操作' }).click()
  await page.getByRole('menuitem', { name: /网络/ }).click()
  await expect(page).toHaveURL(/\/network$/)
  await expect(page.locator('.nazhua-shell .network-panel')).toBeVisible()

  await page.getByRole('button', { name: '操作' }).click()
  await page.getByRole('menuitem', { name: 'ServerStatus' }).click()
  await expect(page.locator('.server-status-shell')).toBeVisible()
  await expect(page.locator('.status-nav')).toHaveCount(1)
  await expect(page.locator('.nazhua-header')).toHaveCount(0)
})

test('hides the map without locations and keeps the first card near the header', async ({ page }) => {
  await useNazhua(page, 'light')
  await page.route('**/api/v2/public/servers', route => fulfillJSON(route, list([
    { ...servers[0], host: { Platform: 'linux', CountryCode: 'ZZZ' }, public_note: {}, name: 'NO-LOCATION' },
  ])))
  await page.goto('/')
  await expect(page.locator('.nazhua-world-map')).toHaveCount(0)
  await expect(page.getByText('NO-LOCATION')).toBeVisible()
  const top = await page.locator('.nazhua-card').evaluate(node => node.getBoundingClientRect().top)
  expect(top).toBeLessThan(230)
})

test('shows a retry action after a load failure and recovers', async ({ page }) => {
  await useNazhua(page)
  let attempts = 0
  await page.route('**/api/v2/public/servers', route => {
    attempts += 1
    return attempts === 1 ? fulfillJSON(route, item({}), 503) : fulfillJSON(route, list(servers))
  })
  await page.goto('/')
  await expect(page.getByText('请求失败，请稍后重试')).toBeVisible()
  await page.getByRole('button', { name: '刷新' }).click()
  await expect(page.locator('.nazhua-card')).toHaveCount(3)
  expect(attempts).toBe(2)
})

test('redirects protected sites to the themed password screen', async ({ page }) => {
  await useNazhua(page)
  let verified = false
  let unlocked = false
  await page.route('**/api/v2/public/bootstrap', route => fulfillJSON(route, item({
    brand: '三太子监控', locale: 'zh-CN', version: 'test', csrf_token: 'public-csrf', logo_url: '/static/logo.svg',
    requires_view_password: !unlocked, view_password_verified: unlocked, show_availability: true, authenticated: false,
    theme: 'nazhua', allow_frontend_theme_switch: true,
  })))
  await page.route('**/api/v2/public/view-password/session', route => {
    verified = route.request().headers()['x-csrf-token'] === 'public-csrf'
    unlocked = true
    return route.fulfill({ status: 204 })
  })
  await page.goto('/')
  await expect(page).toHaveURL(/\/view-password$/)
  await expect(page.locator('.nazhua-shell .password-card')).toBeVisible()
  await page.getByLabel('密码').fill('view-password')
  await page.getByRole('button', { name: '验证' }).click()
  await expect.poll(() => verified).toBe(true)
  await expect(page).toHaveURL(/\/$/)
})

test('mobile temporarily falls back to cards without discarding the saved desktop mode', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'status-mobile')
  await page.addInitScript(() => localStorage.setItem('santaizi-nazhua-list-mode', 'row'))
  await useNazhua(page)
  await page.goto('/')
  await expect(page.locator('.nazhua-home__list.mode-card')).toBeVisible()
  await expect(page.locator('.nazhua-card')).toHaveCount(3)
  expect(await page.evaluate(() => localStorage.getItem('santaizi-nazhua-list-mode'))).toBe('row')
  const header = await page.evaluate(() => {
    const inner = document.querySelector<HTMLElement>('.nazhua-header__inner')!
    const brand = document.querySelector<HTMLElement>('.nazhua-header__brand')!
    const stats = document.querySelector<HTMLElement>('.nazhua-header__stats')!
    return {
      viewport: window.innerWidth,
      flexWrap: getComputedStyle(inner).flexWrap,
      statsBasis: getComputedStyle(stats).flexBasis,
      statsOrder: getComputedStyle(stats).order,
      brandWidth: brand.getBoundingClientRect().width,
    }
  })
  expect(header).toMatchObject({ flexWrap: 'nowrap', statsBasis: 'auto', statsOrder: '0' })
  expect(header.brandWidth).toBeGreaterThan(160)
})

test('desktop exposes the card, row and ServerStatus list modes', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'status-desktop')
  await useNazhua(page)
  await page.setViewportSize({ width: 1440, height: 900 })
  await page.goto('/')

  const modes = page.locator('.nazhua-filter__modes')
  await expect(page.locator('.nazhua-home__list.mode-card')).toBeVisible()
  await modes.getByRole('button', { name: '列表' }).click()
  await expect(page.locator('.nazhua-home__list.mode-row')).toBeVisible()
  await expect(page.locator('.nazhua-row')).toHaveCount(3)
  await modes.getByRole('button', { name: 'ServerStatus' }).click()
  await expect(page.locator('.nazhua-home__list.mode-server-status')).toBeVisible()
  await expect(page.locator('.nazhua-status-table__head span')).toHaveCount(13)
  await modes.getByRole('button', { name: '卡片' }).click()
  await expect(page.locator('.nazhua-card')).toHaveCount(3)
})

test('matches the upstream desktop track and map geometry at 1440px', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'status-desktop')
  await useNazhua(page)
  await page.setViewportSize({ width: 1440, height: 900 })
  await page.goto('/')

  const geometry = await page.evaluate(() => {
    const map = document.querySelector<HTMLElement>('.nazhua-world-map')!.getBoundingClientRect()
    const filter = document.querySelector<HTMLElement>('.nazhua-filter')!.getBoundingClientRect()
    const card = document.querySelector<HTMLElement>('.nazhua-card')!.getBoundingClientRect()
    return {
      map: { x: map.x, y: map.y, width: map.width, height: map.height },
      filterY: filter.y,
      cardY: card.y,
      cardWidth: card.width,
    }
  })
  expect(geometry.map.x).toBeCloseTo(180, 0)
  expect(geometry.map.y).toBeCloseTo(80, 0)
  expect(geometry.map.width).toBeCloseTo(1080, 0)
  expect(geometry.map.height).toBeCloseTo(524, 0)
  expect(geometry.filterY).toBeCloseTo(614, 0)
  expect(geometry.cardY).toBeCloseTo(660, 0)
  expect(geometry.cardWidth).toBeGreaterThan(350)
  expect(geometry.cardWidth).toBeLessThan(355)
})

test('mobile controls keep 44px touch targets', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'status-mobile')
  await useNazhua(page)
  await page.goto('/')
  const targets = page.locator('.nazhua-header__menu, .nazhua-search__trigger, .nazhua-filter .el-button:visible')
  expect(await targets.count()).toBeGreaterThanOrEqual(5)
  for (let index = 0; index < await targets.count(); index += 1) {
    const box = await targets.nth(index).boundingBox()
    expect(box?.width).toBeGreaterThanOrEqual(44)
    expect(box?.height).toBeGreaterThanOrEqual(44)
  }
})

test('captures accepted Nazhua visual baselines', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'status-desktop')
  await page.addInitScript(() => {
    localStorage.setItem('santaizi-public-theme', 'nazhua')
    localStorage.setItem('santaizi-locale', 'zh-CN')
    localStorage.setItem('santaizi-status-theme', new URL(window.location.href).searchParams.get('visual-mode') || 'dark')
  })
  const cases = [
    { name: 'nazhua-dark-1920x947.png', width: 1920, height: 947, mode: 'dark' as const },
    { name: 'nazhua-dark-1440x900.png', width: 1440, height: 900, mode: 'dark' as const },
    { name: 'nazhua-dark-reference-1399x945.png', width: 1399, height: 945, mode: 'dark' as const },
    { name: 'nazhua-dark-mobile-390x844.png', width: 390, height: 844, mode: 'dark' as const },
    { name: 'nazhua-light-1920x947.png', width: 1920, height: 947, mode: 'light' as const },
    { name: 'nazhua-light-1440x900.png', width: 1440, height: 900, mode: 'light' as const },
    { name: 'nazhua-light-mobile-390x844.png', width: 390, height: 844, mode: 'light' as const },
  ]
  for (const visual of cases) {
    await page.setViewportSize({ width: visual.width, height: visual.height })
    await page.goto(`/?visual-mode=${visual.mode}`)
    await expect(page.locator('html')).toHaveAttribute('data-theme', visual.mode)
    await page.evaluate(() => window.scrollTo(0, 0))
    await expect(page.locator('.nazhua-card').first()).toBeVisible()
    await expect(page).toHaveScreenshot(visual.name, { animations: 'disabled', fullPage: false, maxDiffPixelRatio: .01 })
    if (visual.width === 390 && visual.mode === 'dark') {
      await page.goto('/server/1')
      await expect(page.locator('.nazhua-detail')).toBeVisible()
      await expect(page).toHaveScreenshot('nazhua-detail-dark-mobile-390x844.png', { animations: 'disabled', fullPage: false, maxDiffPixelRatio: .01 })
    }
  }
})
