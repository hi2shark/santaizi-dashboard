import { describe, expect, it } from 'vitest'
import {
  buildPublicNoteView,
  decodeOrderLink,
  flagCode,
  getBillAndPlan,
  getCycleMonths,
  getNextCycleTime,
  getPlanTags,
  publicLocation,
  publicSubtitle,
} from '@santaizi/theme-server-status'

const sample = {
  billingDataMod: {
    startDate: '2024-10-01',
    endDate: '2024-11-01',
    autoRenewal: '0',
    cycle: '月',
    amount: '$3.99',
  },
  planDataMod: {
    bandwidth: '30Mbps',
    trafficVol: '1TB/月',
    trafficType: '1',
    IPv4: '1',
    IPv6: '1',
    networkRoute: 'CN2,GIA',
    extra: '传家宝',
  },
  customData: {
    location: 'HKG',
    slogan: '香港节点',
    orderLink: 'https%3A%2F%2Fbuy.example.com',
    buyBtnText: '官网',
    buyBtnIcon: 'ri-gift-2-line',
    flag: 'cn',
    locationLabel: '香港',
  },
}

describe('publicNoteView', () => {
  it('reads customData for subtitle/location/flag', () => {
    expect(publicSubtitle(sample)).toBe('香港节点')
    expect(publicLocation(sample, 'us')).toBe('香港')
    expect(flagCode(sample, 'us')).toBe('cn')
  })

  it('falls back when presentation keys wrongly used', () => {
    expect(publicSubtitle({ presentation: { slogan: 'x' } })).toBe('x')
  })

  it('computes remaining days and infinity', () => {
    const now = Date.parse('2024-10-20T00:00:00+08:00')
    const bill = getBillAndPlan(sample, now)
    expect(bill.amountKind).toBe('priced')
    expect(bill.amountValue).toBe('$3.99')
    expect(bill.remainingKind).toBe('days')
    expect(bill.remainingDays).toBeGreaterThan(0)
    expect(bill.remainingTone).toMatch(/success|warning|danger/)

    const forever = getBillAndPlan({
      billingDataMod: { endDate: '0000-00-00', amount: '0' },
    }, now)
    expect(forever.amountKind).toBe('free')
    expect(forever.remainingKind).toBe('infinity')
    expect(forever.remainingTone).toBe('success')
  })

  it('marks expired without auto renewal', () => {
    const now = Date.parse('2025-01-01T00:00:00Z')
    const bill = getBillAndPlan({
      billingDataMod: { endDate: '2024-01-01', autoRenewal: '0', amount: '-1' },
    }, now)
    expect(bill.amountKind).toBe('metered')
    expect(bill.remainingKind).toBe('expired')
    expect(bill.remainingTone).toBe('danger')
  })

  it('builds plan tags with dual stack sentinel', () => {
    expect(getPlanTags(sample)).toEqual(['CN2', 'GIA', '传家宝', '__dual_stack__'])
  })

  it('decodes order link and cycle months', () => {
    expect(decodeOrderLink(sample.customData.orderLink)).toBe('https://buy.example.com')
    expect(getCycleMonths('年')).toBe(12)
    expect(getCycleMonths('季')).toBe(3)
  })

  it('advances auto-renewal past now', () => {
    const start = Date.parse('2024-01-01T00:00:00Z')
    const now = Date.parse('2024-06-15T00:00:00Z')
    const next = getNextCycleTime(start, 1, now)
    expect(next).toBeGreaterThan(now)
  })

  it('empty note has no detail shell', () => {
    const view = buildPublicNoteView({})
    expect(view.hasDetail).toBe(false)
    expect(view.hasBuy).toBe(false)
  })
})
