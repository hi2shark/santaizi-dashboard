import { expect, test, type Page, type Route } from '@playwright/test'

const list = (data: unknown[] = []) => JSON.stringify({ data, meta: { page: 1, page_size: 20, total: data.length } })
const item = (data: unknown) => JSON.stringify({ data })
const probeMetadata = {
  required: ['heartbeat', 'identity'],
  optional: [{ id: 'cpu', disable_flag: '--disable-cpu' }, { id: 'memory', disable_flag: '--disable-memory' }, { id: 'nat', disable_flag: '--disable-nat' }],
  presets: {
    standard: { cpu: true, memory: true, disk: true, network: true, connections: true, processes: true, temperature: true, gpu: true, host_info: true, ip_report: true, http_probe: true, icmp_probe: true, tcp_probe: true, nat: false },
    light: { cpu: true, memory: true, disk: false, network: true, connections: false, processes: false, temperature: false, gpu: false, host_info: true, ip_report: true, http_probe: true, icmp_probe: true, tcp_probe: true, nat: false },
    alive: { cpu: false, memory: false, disk: false, network: false, connections: false, processes: false, temperature: false, gpu: false, host_info: false, ip_report: false, http_probe: false, icmp_probe: false, tcp_probe: false, nat: false },
  },
}

async function fulfillJSON(route: Route, body: string, status = 200) {
  await route.fulfill({ contentType: 'application/json', status, body })
}

test.beforeEach(async ({ page }) => {
  await page.route('**/static/logo.svg', route => route.fulfill({ contentType: 'image/svg+xml', body: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><path fill="#2563eb" d="M4 4h24v24H4z"/></svg>' }))
  await page.route('**/api/v2/auth/session', route => fulfillJSON(route, item({ authenticated: true, csrf_token: 'test-csrf', login_url: '/oauth2/login', capabilities: ['*'], user: { id: 1, login: 'admin', name: 'Admin', super_admin: true } })))
})

async function mockEditorOptions(page: Page) {
  await page.route('**/api/v2/admin/servers?**', route => fulfillJSON(route, list([{ id: 7, name: 'edge-a', tag: 'edge', online: true, public_note: {}, monitoring_options: {} }])))
  await page.route('**/api/v2/admin/notifications?**', route => fulfillJSON(route, list([{ id: 3, name: 'Ops', tag: 'ops', url: 'https://example.test/hook', method: 'post', request_type: 'json', headers: [], body: '', verify_tls: true }])))
}

test('creates a server with structured public notes and reusable installation credentials', async ({ page }) => {
  let submitted: Record<string, unknown> | undefined
  await page.route('**/api/v2/admin/ddns?**', route => fulfillJSON(route, list()))
  await page.route('**/api/v2/admin/notifications?**', route => fulfillJSON(route, list()))
  await page.route('**/api/v2/admin/servers**', route => {
    if (route.request().method() === 'POST') {
      submitted = route.request().postDataJSON() as Record<string, unknown>
      return fulfillJSON(route, item({ id: 2, ...submitted, secret: 'reusable-secret', online: false }), 201)
    }
    return fulfillJSON(route, list())
  })
  await page.route('**/api/v2/admin/probe-capabilities', route => fulfillJSON(route, item(probeMetadata)))
  await page.route('**/api/v2/admin/servers/2/install-preview', route => fulfillJSON(route, item({ platform: 'linux', command: 'install santaizi-agent --clean-install', clean_install: true, options: route.request().postDataJSON()?.options || {} })))

  await page.goto('/admin/servers')
  await page.getByRole('button', { name: '添加服务器' }).click()
  const dialog = page.getByRole('dialog', { name: '添加服务器' })
  await dialog.getByLabel('服务器名称').fill('edge-b')
  await dialog.getByLabel('分组').fill('edge')
  await dialog.getByRole('tab', { name: '公开备注' }).click()
  await dialog.getByLabel('金额').fill('12.00')
  await dialog.getByRole('tab', { name: '套餐' }).click()
  await dialog.getByLabel('带宽').fill('1 Gbps')
  await dialog.getByRole('button', { name: '保存' }).click()

  await expect.poll(() => submitted).toMatchObject({ name: 'edge-b', tag: 'edge', public_note: { billingDataMod: { amount: '12.00' }, planDataMod: { bandwidth: '1 Gbps' } } })
  const install = page.getByRole('dialog', { name: /安装探针/ })
  await expect(install.getByLabel('密钥')).toHaveValue('reusable-secret')
  await expect(install.getByText('标准', { exact: true })).toBeVisible()
  await expect(install.getByText('轻量', { exact: true })).toBeVisible()
  await expect(install.getByText('仅存活', { exact: true })).toBeVisible()
  await expect(install.getByText('CPU 与负载', { exact: true })).toBeVisible()
})

test('manages multiple traffic policies inside the server editor', async ({ page }) => {
  const policies: Record<string, unknown>[] = []
  await page.route('**/api/v2/admin/ddns?**', route => fulfillJSON(route, list()))
  await page.route('**/api/v2/admin/notifications?**', route => fulfillJSON(route, list()))
  await page.route('**/api/v2/admin/servers**', route => {
    if (route.request().method() === 'POST') return fulfillJSON(route, item({ id: 4, ...route.request().postDataJSON(), secret: 'server-secret' }), 201)
    return fulfillJSON(route, list())
  })
  await page.route('**/api/v2/admin/servers/4/traffic-policies', route => {
    policies.push(route.request().postDataJSON() as Record<string, unknown>)
    return fulfillJSON(route, item({ id: policies.length, server_id: 4, ...policies.at(-1) }), 201)
  })
  await page.route('**/api/v2/admin/probe-capabilities', route => fulfillJSON(route, item(probeMetadata)))
  await page.route('**/api/v2/admin/servers/4/install-preview', route => fulfillJSON(route, item({ platform: 'linux', command: 'install santaizi-agent', clean_install: true, options: route.request().postDataJSON()?.options || {} })))

  await page.goto('/admin/servers')
  await page.getByRole('button', { name: '添加服务器' }).click()
  const dialog = page.getByRole('dialog', { name: '添加服务器' })
  await dialog.getByLabel('服务器名称').fill('traffic-node')
  await dialog.getByRole('tab', { name: '流量策略' }).click()
  await dialog.getByRole('button', { name: '添加流量策略' }).click()
  await dialog.getByRole('button', { name: '添加流量策略' }).click()
  const cards = dialog.locator('.traffic-policy-card')
  await cards.nth(0).getByLabel('名称').fill('Monthly total')
  await cards.nth(1).getByLabel('名称').fill('Inbound cap')
  await dialog.getByRole('button', { name: '保存' }).click()

  await expect.poll(() => policies).toHaveLength(2)
  expect(policies.map(policy => policy.name)).toEqual(['Monthly total', 'Inbound cap'])
  expect(policies.every(policy => policy.mode === 'recurring' && Boolean(policy.cycle_start))).toBe(true)
})

test('service monitor uses typed target, notification group and searchable server transfer', async ({ page }) => {
  let payload: Record<string, unknown> | undefined
  await mockEditorOptions(page)
  await page.route('**/api/v2/admin/monitors**', route => {
    if (route.request().method() === 'POST') {
      payload = route.request().postDataJSON() as Record<string, unknown>
      return fulfillJSON(route, item({ id: 1, ...payload }), 201)
    }
    return fulfillJSON(route, list())
  })

  await page.goto('/admin/services')
  await page.getByRole('button', { name: '添加服务监控' }).click()
  const dialog = page.getByRole('dialog', { name: '添加服务监控' })
  await dialog.getByLabel('名称').fill('Website health')
  await dialog.getByLabel('目标').fill('https://example.test/health')
  await dialog.getByText('仅所选服务器', { exact: true }).click()
  await expect(dialog.locator('.el-transfer')).toBeVisible()
  await dialog.locator('.el-transfer-panel').first().getByText('edge-a', { exact: true }).click()
  await dialog.locator('.el-transfer__buttons .el-button:not(.is-disabled)').click()
  await dialog.getByRole('button', { name: '保存' }).click()
  await expect.poll(() => payload).toMatchObject({ type: 'http', target: 'https://example.test/health', scope: { mode: 'include', server_ids: [7] } })
})

test('notification channels and alert rules have separate typed editors', async ({ page }) => {
  let notification: Record<string, unknown> | undefined
  let alert: Record<string, unknown> | undefined
  await mockEditorOptions(page)
  await page.route('**/api/v2/admin/notifications**', route => {
    if (route.request().method() === 'POST') {
      notification = route.request().postDataJSON() as Record<string, unknown>
      return fulfillJSON(route, item({ id: 8, ...notification }), 201)
    }
    return fulfillJSON(route, list())
  })
  await page.route('**/api/v2/admin/alert-rules**', route => {
    if (route.request().method() === 'POST') {
      alert = route.request().postDataJSON() as Record<string, unknown>
      return fulfillJSON(route, item({ id: 9, ...alert }), 201)
    }
    return fulfillJSON(route, list())
  })

  await page.goto('/admin/notifications')
  await page.getByRole('button', { name: '添加通知渠道' }).click()
  let dialog = page.getByRole('dialog', { name: '添加通知渠道' })
  await dialog.getByLabel('名称').fill('Ops webhook')
  await dialog.getByLabel('通知组').fill('ops')
  await dialog.getByLabel('请求地址').fill('https://example.test/hook')
  await expect(dialog.getByLabel('通知组')).toHaveAttribute('type', 'text')
  await dialog.getByRole('button', { name: '保存' }).click()
  await expect.poll(() => notification).toMatchObject({ tag: 'ops', method: 'post', request_type: 'json' })

  await page.goto('/admin/alert-rules')
  await page.getByRole('button', { name: '添加告警规则' }).click()
  dialog = page.getByRole('dialog', { name: '添加告警规则' })
  await dialog.getByLabel('名称').fill('High CPU')
  await expect(dialog.getByText('CPU', { exact: true })).toBeVisible()
  await expect(dialog.getByLabel('信息类型')).toBeVisible()
  await dialog.getByRole('button', { name: '保存' }).click()
  await expect.poll(() => alert).toMatchObject({ name: 'High CPU', trigger_mode: 'always', conditions: [{ type: 'cpu', duration_seconds: 30 }] })
})

test('additional features use provider metadata and server selectors', async ({ page }) => {
  await page.route('**/api/v2/admin/ddns**', route => fulfillJSON(route, list()))
  // Playwright evaluates matching routes in reverse registration order, so the
  // specific metadata route must be registered after the DDNS collection route.
  await page.route('**/api/v2/admin/ddns/providers', route => fulfillJSON(route, list([{ id: 1, name: 'Webhook', access_id: true, access_secret: true, webhook_url: true, webhook_method: true, webhook_request_type: true, webhook_headers: true, webhook_request_body: true }])))
  await page.route('**/api/v2/admin/nat**', route => fulfillJSON(route, list()))
  await page.route('**/api/v2/admin/servers?**', route => fulfillJSON(route, list([{ id: 7, name: 'edge-a', tag: 'edge' }])))

  await page.goto('/admin/ddns')
  await page.getByRole('button', { name: '添加 DDNS 配置' }).click()
  let dialog = page.getByRole('dialog', { name: '添加 DDNS 配置' })
  await expect(dialog.getByLabel('服务商')).toBeVisible()
  await dialog.locator('.el-form-item').filter({ hasText: '服务商' }).locator('.el-select__wrapper').click()
  await page.getByRole('option', { name: 'Webhook' }).click()
  await expect(dialog.getByLabel('Access ID')).toHaveAttribute('type', 'text')
  await expect(dialog.getByLabel('Access Secret')).toHaveAttribute('type', 'password')
  await dialog.getByRole('button', { name: '取消' }).click()

  await page.goto('/admin/nat')
  await page.getByRole('button', { name: '添加内网穿透' }).click()
  dialog = page.getByRole('dialog', { name: '添加内网穿透' })
  await expect(dialog.getByLabel('服务器')).toHaveAttribute('role', 'combobox')
  await expect(dialog.getByLabel('本地服务')).toBeVisible()
  await expect(dialog.getByLabel('绑定域名')).toBeVisible()
})

test('dirty editor blocks escape and confirms cancellation', async ({ page }) => {
  await page.route('**/api/v2/admin/ddns?**', route => fulfillJSON(route, list()))
  await page.route('**/api/v2/admin/notifications?**', route => fulfillJSON(route, list()))
  await page.route('**/api/v2/admin/monitors**', route => fulfillJSON(route, list()))
  await page.route('**/api/v2/admin/servers**', route => fulfillJSON(route, list()))
  await page.goto('/admin/servers')
  await page.getByRole('button', { name: '添加服务器' }).click()
  const dialog = page.getByRole('dialog', { name: '添加服务器' })
  await dialog.getByLabel('服务器名称').fill('unsaved')
  await page.keyboard.press('Escape')
  await expect(dialog).toBeVisible()
  await dialog.getByRole('button', { name: '取消' }).click()
  await expect(page.getByRole('dialog', { name: '放弃修改' })).toBeVisible()
  await page.getByRole('button', { name: '继续编辑' }).click()
  await expect(dialog).toBeVisible()
  await dialog.getByRole('button', { name: '关闭此对话框' }).click()
  await expect(page.getByRole('dialog', { name: '放弃修改' }).last()).toBeVisible()
  await page.getByRole('dialog', { name: '放弃修改' }).last().getByRole('button', { name: '继续编辑' }).click()
  await page.locator('a[href="/admin/services"]').evaluate((element: HTMLElement) => element.click())
  const routeConfirm = page.getByRole('dialog', { name: '放弃修改' }).last()
  await expect(routeConfirm).toBeVisible()
  await routeConfirm.getByRole('button', { name: '放弃' }).click()
  await expect(page).toHaveURL(/\/admin\/services$/)
})

test('collector and API token credentials can be viewed again by stable identifier', async ({ page }) => {
  const collector = { id: 'collector-1', name: 'Shanghai edge', address: 'collector.example.com:5555', tls: true, insecure_tls: false, generation: 1, config_version: 1, status: 'healthy', revoked: false, scopes: [{ type: 'all', value: '' }] }
  await page.route('**/api/v2/admin/telemetry/collectors**', route => {
    const path = new URL(route.request().url()).pathname
    if (path.endsWith('/token')) return fulfillJSON(route, item({ collector_id: 'collector-1', registration_token: 'collector-token', revoked: false }))
    return fulfillJSON(route, list([collector]))
  })
  await page.goto('/admin/telemetry')
  await page.locator('.el-table__body .el-dropdown button').click()
  await page.getByText('查看 Token', { exact: true }).click()
  await expect(page.getByRole('dialog', { name: '注册 Token' }).locator('input')).toHaveValue('collector-token')

  await page.route('**/api/v2/admin/api-tokens**', route => {
    const path = new URL(route.request().url()).pathname
    if (path.endsWith('/17')) return fulfillJSON(route, item({ id: 17, note: 'automation', token: 'reusable-api-token', token_prefix: 'reus', created_at: '2026-08-11T12:00:00Z' }))
    return fulfillJSON(route, list([{ id: 17, note: 'automation', token_prefix: 'reus', created_at: '2026-08-11T12:00:00Z' }]))
  })
  await page.goto('/admin/api-tokens')
  await page.getByRole('button', { name: '查看 Token' }).click()
  await expect(page.getByRole('dialog', { name: 'Token' }).locator('input')).toHaveValue('reusable-api-token')
})

test('switches locale without a page navigation', async ({ page }) => {
  await page.route('**/api/v2/admin/summary', route => fulfillJSON(route, item({})))
  await page.goto('/admin/')
  const before = page.url()
  await page.locator('.locale-select').click()
  await page.getByText('English', { exact: true }).last().click()
  await expect(page.getByRole('heading', { name: 'Overview' })).toBeVisible()
  expect(page.url()).toBe(before)
})

test('keeps light and dark surfaces coherent at all responsive baselines', async ({ page }) => {
  await page.route('**/api/v2/admin/summary', route => fulfillJSON(route, item({})))
  await page.goto('/admin/')

  const widths = [375, 768, 1024, 1440]
  for (const theme of ['light', 'dark'] as const) {
    await page.evaluate(value => localStorage.setItem('santaizi-admin-theme', value), theme)
    for (const width of widths) {
      await page.setViewportSize({ width, height: 900 })
      await page.reload()
      await expect(page.locator('html')).toHaveClass(theme === 'dark' ? /dark/ : /^(?!.*dark)/)

      const measurements = await page.evaluate(() => {
        const rgb = (value: string) => value.match(/\d+(?:\.\d+)?/g)?.slice(0, 3).map(Number) ?? []
        const body = getComputedStyle(document.body)
        const sidebar = getComputedStyle(document.querySelector<HTMLElement>('.admin-sidebar')!)
        const surface = getComputedStyle(document.querySelector<HTMLElement>('.surface')!)
        const refresh = document.querySelector<HTMLElement>('.page-head .el-button')!
        const menu = getComputedStyle(document.querySelector<HTMLElement>('.mobile-menu')!)
        return {
          backgrounds: [rgb(body.backgroundColor), rgb(sidebar.backgroundColor), rgb(surface.backgroundColor)],
          refreshHeight: refresh.getBoundingClientRect().height,
          refreshFontSize: Number.parseFloat(getComputedStyle(refresh).fontSize),
          mobileMenuVisible: menu.display !== 'none',
        }
      })

      for (const color of measurements.backgrounds) {
        expect(color).toHaveLength(3)
        if (theme === 'light') expect(Math.min(...color)).toBeGreaterThanOrEqual(230)
        else expect(Math.max(...color)).toBeLessThanOrEqual(48)
      }
      expect(measurements.refreshHeight).toBeGreaterThanOrEqual(width <= 720 ? 44 : 36)
      expect(measurements.refreshFontSize).toBeGreaterThanOrEqual(12)
      expect(measurements.mobileMenuVisible).toBe(width <= 860)
    }
  }
})
