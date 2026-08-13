import { describe, expect, it } from 'vitest'
import type { ResourceRecord, ServerRecord } from '@santaizi/api'
import {
  clampPercent,
  formatCompactBytes,
  formatUptime,
  mapCycleTransfers,
  percentOf,
  toNazhuaServerView,
} from '../../../../packages/theme-nazhua/src/domain/nazhuaServerView'

function server(overrides: Partial<ServerRecord> = {}): ServerRecord {
  return {
    id: 7,
    name: 'HKG-EDGE',
    tag: 'HKG',
    display_index: 10,
    hide_for_guest: false,
    enable_ddns: false,
    online: true,
    host: { CountryCode: 'HK', CPU: 2 },
    state: {
      CPU: 12.5,
      MemUsed: 1_073_741_824,
      MemTotal: 2_147_483_648,
      DiskUsed: 10_737_418_240,
      DiskTotal: 21_474_836_480,
      Uptime: 172_900,
      NetInSpeed: 4096,
      NetOutSpeed: 2048,
      NetInTransfer: 1024,
      NetOutTransfer: 2048,
    },
    public_note: {
      customData: { location: 'HKG', slogan: '香港边缘', flag: 'hk', orderLink: 'https%3A%2F%2Fexample.com' },
      planDataMod: { networkRoute: 'CN2,GIA', IPv4: '1', IPv6: '1' },
      billingDataMod: { amount: '9.99CNY', cycle: '月' },
    },
    ...overrides,
  }
}

describe('Nazhua server view adapter', () => {
  it('normalizes percentages, bytes, uptime and public note', () => {
    const view = toNazhuaServerView(server())
    expect(view.online).toBe(true)
    expect(view.cpuPercent).toBe(12.5)
    expect(view.memoryPercent).toBe(50)
    expect(view.diskPercent).toBe(50)
    expect(view.memoryValue).toBe('1G')
    expect(view.uptime).toBe('2')
    expect(view.slogan).toBe('香港边缘')
    expect(view.publicNote.planTags).toEqual(['CN2', 'GIA', '__dual_stack__'])
    expect(view.location?.code).toBeTruthy()
    expect(view.flagClass).toBe('fi fi-hk')
    expect(view.orderLink).toBe('https://example.com')
  })

  it('does not infer online from stale telemetry', () => {
    expect(toNazhuaServerView(server({ online: false })).online).toBe(false)
  })

  it('maps and aggregates one homepage cycle-transfer response', () => {
    const rows: ResourceRecord[] = [
      { server_id: 7, name: '月流量', direction: 'both', used_bytes: 30, quota_bytes: 100, status: 'normal' },
      { server_id: 7, name: '附加', direction: 'both', used_bytes: 20, quota_bytes: 100, status: 'warning' },
      { server_id: 8, used_bytes: 5, quota_bytes: 10 },
    ]
    const cycles = mapCycleTransfers(rows)
    expect(cycles.get(7)).toMatchObject({ usedBytes: 50, quotaBytes: 200, usagePercent: 25, status: 'warning' })
    expect(toNazhuaServerView(server(), cycles).trafficBytes).toBe(150)
  })

  it('keeps format helpers bounded and deterministic', () => {
    expect(clampPercent(140)).toBe(100)
    expect(percentOf(1, 4)).toBe(25)
    expect(formatCompactBytes(1_073_741_824)).toBe('1G')
    expect(formatUptime(3_601)).toBe('1h')
  })
})
