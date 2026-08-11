import { describe, expect, it } from 'vitest'
import { normalizeServer } from './status'

describe('ServerStatus runtime normalization', () => {
  it('accepts canonical API v2 fields', () => {
    expect(normalizeServer({
      id: 7,
      name: 'edge-a',
      tag: 'edge',
      online: true,
      telemetry: { host: 'online', connectivity: 'partial', available: true, coverage: 'partial' },
    })).toMatchObject({ id: 7, name: 'edge-a', tag: 'edge', online: true })
  })

  it('normalizes a canonical realtime update', () => {
    expect(normalizeServer({ id: 8, name: 'edge-b', tag: 'edge', online: true })).toMatchObject({
      id: 8,
      name: 'edge-b',
      tag: 'edge',
      online: true,
    })
  })
})
