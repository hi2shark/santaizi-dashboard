import { describe, expect, it } from 'vitest'
import type { CollectorRecord, ServerRecord } from '@santaizi/api'
import type { ConnectionPath } from '@santaizi/api'
import { DEFAULT_VIEW } from './geo'
import { buildTopology, primaryLatencyRows } from './topology'

function server(id: number, name: string, extra: { country?: string; note?: Record<string, unknown>; online?: boolean } = {}): ServerRecord {
  return {
    id, name, tag: 'edge', display_index: id, hide_for_guest: false, enable_ddns: false, online: extra.online ?? true,
    host: extra.country ? { CountryCode: extra.country } : undefined,
    public_note: extra.note,
  }
}

function collector(id: string, name: string, extra: Partial<CollectorRecord> = {}): CollectorRecord {
  return {
    id, name, address: `${id}.example:5555`, tls: true, insecure_tls: false,
    generation: 1, config_version: 1, revoked: false, status: 'online',
    scopes: [{ type: 'all', value: '' }],
    ...extra,
  }
}

function path(serverId: number, observerId: string, kind: 'primary' | 'collector', connected = true): ConnectionPath {
  return {
    server_id: serverId, server_name: `s${serverId}`, node_uuid: 'aa', observer_id: observerId,
    observer_kind: kind, observer_name: observerId === 'primary' ? '' : 'edge', assigned: true,
    sink: { connected, last_rtt_ms: connected ? 12 : undefined },
  }
}

describe('buildTopology', () => {
  it('places nodes by CountryCode and aggregates the same location', () => {
    const graph = buildTopology({
      servers: [server(1, 'a', { country: 'CN' }), server(2, 'b', { country: 'CN' }), server(3, 'c')],
      collectors: [],
      paths: [],
      siteTitle: '三太子监控',
    })
    expect(graph.nodes).toHaveLength(1)
    expect(graph.nodes[0]?.count).toBe(2)
    expect(graph.nodes[0]?.onlines).toEqual([true, true])
    expect(graph.unlocated).toEqual([{ id: '3', name: 'c' }])
    expect(graph.countries).toEqual(['CN'])
    expect(graph.primary.derived).toBe(true)
    expect(graph.primary.lon).not.toBeCloseTo(116.41)
    expect(Math.abs(graph.primary.lon - 116.41)).toBeLessThan(15)
  })

  it('derives collector position from covered nodes, then falls back to primary', () => {
    const derived = buildTopology({
      servers: [server(1, 'tokyo', { country: 'JP' })],
      collectors: [collector('c1', 'edge')],
      paths: [path(1, 'c1', 'collector')],
      primaryLocation: 'DE',
    })
    expect(derived.collectors[0]?.derived).toBe(true)
    expect(Math.abs((derived.collectors[0]?.lon ?? 0) - 139.65)).toBeLessThan(15)

    const empty = buildTopology({
      servers: [],
      collectors: [collector('c1', 'edge')],
      paths: [],
      primaryLocation: 'DE',
    })
    expect(Math.abs((empty.collectors[0]?.lon ?? 0) - 8.68)).toBeLessThan(15)
    expect(empty.primary.derived).toBe(false)
    expect(empty.primary.lon).toBeCloseTo(8.68)
  })

  it('spreads derived markers that would land on the same spot', () => {
    const graph = buildTopology({
      servers: [server(1, 'a', { country: 'CN' })],
      collectors: [collector('c1', 'one'), collector('c2', 'two')],
      paths: [],
    })
    const keys = [graph.primary, ...graph.collectors, ...graph.nodes].map(marker => `${marker.lon.toFixed(1)},${marker.lat.toFixed(1)}`)
    expect(new Set(keys).size).toBe(keys.length)
    expect(graph.nodes[0]?.lon).toBeCloseTo(116.41)
  })

  it('uses the default facing when nothing is located', () => {
    const graph = buildTopology({ servers: [server(1, 'ghost')], collectors: [], paths: [] })
    expect(graph.primary.lon).toBe(DEFAULT_VIEW.lon)
    expect(graph.primary.lat).toBe(DEFAULT_VIEW.lat)
    expect(graph.links).toEqual([])
  })

  it('builds path and replication links', () => {
    const graph = buildTopology({
      servers: [server(1, 'a', { country: 'SG' })],
      collectors: [collector('c1', 'sin')],
      paths: [path(1, 'primary', 'primary', true), path(1, 'c1', 'collector', false)],
      primaryLocation: 'CN',
    })
    expect(graph.links.some(link => link.kind === 'replication' && link.toId === 'primary')).toBe(true)
    expect(graph.links.some(link => link.kind === 'path' && link.toId === 'c1' && !link.connected)).toBe(true)
    expect(graph.pathsAssigned).toBe(2)
    expect(graph.pathsConnected).toBe(1)
    expect(graph.collectors[0]?.coverage).toBe('0/1')
  })

  it('keeps per-node online flags when the same location mixes status', () => {
    const graph = buildTopology({
      servers: [
        server(1, 'up', { country: 'CN', online: true }),
        server(2, 'down', { country: 'CN', online: false }),
      ],
      collectors: [],
      paths: [],
    })
    expect(graph.nodes[0]?.count).toBe(2)
    expect(graph.nodes[0]?.status).toBe('mixed')
    expect(graph.nodes[0]?.onlines).toEqual([true, false])
  })

  it('places nodes from public-note location before CountryCode', () => {
    const graph = buildTopology({
      servers: [server(1, 'salt', { country: 'US', note: { customData: { location: 'SLC' } } })],
      collectors: [],
      paths: [],
    })
    expect(graph.nodes[0]?.lon).toBeCloseTo(-111.891)
    expect(graph.unlocated).toEqual([])
    expect(graph.countries).toEqual(['US'])
  })

  it('lists primary latency and marks offline nodes', () => {
    const rows = primaryLatencyRows(
      [server(2, 'ghost', { online: false }), server(1, 'tokyo')],
      [path(1, 'primary', 'primary', true), path(1, 'c1', 'collector', true)],
    )
    expect(rows.map(row => row.name)).toEqual(['tokyo', 'ghost'])
    expect(rows[0]).toMatchObject({ online: true, rttMs: 12 })
    expect(rows[1]).toMatchObject({ online: false, rttMs: undefined })
  })

  it('treats a disconnected primary path as offline', () => {
    const rows = primaryLatencyRows(
      [server(1, 'tokyo')],
      [path(1, 'primary', 'primary', false)],
    )
    expect(rows[0]).toMatchObject({ online: false, rttMs: undefined })
  })
})
