import type { CollectorRecord, ServerRecord } from '@santaizi/api'
import type { ConnectionPath } from '@santaizi/api'
import { DEFAULT_VIEW, locationKey, parseLocation, resolveServerGeo, sphericalMean, type GeoPoint } from './geo'

export type MarkerKind = 'primary' | 'collector' | 'node'
export type MarkerStatus = 'online' | 'offline' | 'mixed'

export interface TopologyMarker {
  id: string
  kind: MarkerKind
  name: string
  lon: number
  lat: number
  derived: boolean
  status: MarkerStatus
  href?: string
  coverage?: string
  rttMs?: number
  count: number
  names: string[]
  /** 与 names 对齐，供同位置多节点分色。 */
  onlines: boolean[]
}

export interface TopologyLink {
  fromId: string
  toId: string
  connected: boolean
  kind: 'path' | 'replication'
  rttMs?: number
}

export interface UnlocatedNode {
  id: string
  name: string
}

export interface TopologyGraph {
  primary: TopologyMarker
  collectors: TopologyMarker[]
  nodes: TopologyMarker[]
  unlocated: UnlocatedNode[]
  /** 有节点落地的国家/地区 ISO2。地球不再点亮板块，仅保留给统计/测试。 */
  countries: string[]
  links: TopologyLink[]
  collectorsOnline: number
  collectorsTotal: number
  pathsConnected: number
  pathsAssigned: number
}

export interface TopologyInput {
  servers: ServerRecord[]
  collectors: CollectorRecord[]
  paths: ConnectionPath[]
  primaryLocation?: string
  siteTitle?: string
}

function collectorStatus(row: CollectorRecord): MarkerStatus {
  if (row.revoked) return 'offline'
  if (row.status === 'online') return 'online'
  if (row.status === 'offline') return 'offline'
  return 'mixed'
}

function serverLocated(server: ServerRecord): { point: GeoPoint; country: string } | null {
  return resolveServerGeo(server)
}

function collectorCovers(collector: CollectorRecord, server: ServerRecord, paths: ConnectionPath[]): boolean {
  if (paths.some(path => path.observer_id === collector.id && path.server_id === server.id)) return true
  const scopes = collector.scopes || []
  if (!scopes.length) return false
  if (scopes.some(scope => scope.type === 'all')) return true
  return scopes.some((scope) => {
    if (scope.type === 'server') return String(scope.value) === String(server.id)
    if (scope.type === 'group' || scope.type === 'tag') return scope.value === server.tag
    return false
  })
}

function mixStatus(online: number, offline: number): MarkerStatus {
  if (online && offline) return 'mixed'
  if (online) return 'online'
  return 'offline'
}

const RING_ANGLES = [0, 45, 90, 135, 180, 225, 270, 315]

/**
 * 推算位置常常互相重合（无节点定位时主面板与从端都会落到同一锚点），
 * 重合的点在地球上只剩一个可点区域，因此按环形错开。手填位置不动。
 */
function spread(point: GeoPoint, taken: Set<string>, derived: boolean): GeoPoint {
  const claim = (candidate: GeoPoint) => {
    taken.add(locationKey(candidate))
    return candidate
  }
  if (!derived || !taken.has(locationKey(point))) return claim(point)
  for (const radius of [7, 14, 21]) {
    for (const angle of RING_ANGLES) {
      const rad = angle * Math.PI / 180
      const lat = Math.max(-84, Math.min(84, point.lat + Math.sin(rad) * radius))
      const stretch = Math.max(0.2, Math.cos(lat * Math.PI / 180))
      let lon = point.lon + Math.cos(rad) * radius / stretch
      while (lon > 180) lon -= 360
      while (lon < -180) lon += 360
      const candidate = { lon, lat }
      if (!taken.has(locationKey(candidate))) return claim(candidate)
    }
  }
  return claim(point)
}

export function buildTopology(input: TopologyInput): TopologyGraph {
  const unlocated: UnlocatedNode[] = []
  const locatedServers: Array<{ server: ServerRecord; point: GeoPoint }> = []
  const countries = new Set<string>()
  for (const server of input.servers) {
    const located = serverLocated(server)
    if (!located) {
      unlocated.push({ id: String(server.id), name: server.name })
      continue
    }
    if (located.country) countries.add(located.country)
    locatedServers.push({ server, point: located.point })
  }

  const nodeGroups = new Map<string, { marker: TopologyMarker; online: number; offline: number }>()
  const serverMarkerId = new Map<number, string>()
  for (const { server, point } of locatedServers) {
    const key = locationKey(point)
    const id = `node:${key}`
    serverMarkerId.set(server.id, id)
    const online = server.online ? 1 : 0
    const offline = server.online ? 0 : 1
    const existing = nodeGroups.get(id)
    if (!existing) {
      nodeGroups.set(id, {
        online, offline,
        marker: {
          id, kind: 'node', name: server.name, lon: point.lon, lat: point.lat, derived: false,
          status: server.online ? 'online' : 'offline', href: `/servers?q=${encodeURIComponent(server.name)}`,
          count: 1, names: [server.name], onlines: [Boolean(server.online)],
        },
      })
      continue
    }
    existing.online += online
    existing.offline += offline
    existing.marker.count += 1
    existing.marker.names.push(server.name)
    existing.marker.onlines.push(Boolean(server.online))
    existing.marker.status = mixStatus(existing.online, existing.offline)
    existing.marker.name = existing.marker.count === 2
      ? existing.marker.names.join(' · ')
      : `${existing.marker.names[0]} +${existing.marker.count - 1}`
  }

  const taken = new Set<string>([...nodeGroups.keys()].map(id => id.slice('node:'.length)))
  const locatedPoints = locatedServers.map(item => item.point)
  const parsedPrimary = parseLocation(input.primaryLocation)
  const anchor = parsedPrimary || sphericalMean(locatedPoints) || DEFAULT_VIEW
  const primaryPoint = spread(anchor, taken, !parsedPrimary)
  const primary: TopologyMarker = {
    id: 'primary',
    kind: 'primary',
    name: input.siteTitle || 'Santaizi',
    lon: primaryPoint.lon,
    lat: primaryPoint.lat,
    derived: !parsedPrimary,
    status: 'online',
    count: 1,
    names: [input.siteTitle || 'Santaizi'],
    onlines: [true],
  }

  const collectors: TopologyMarker[] = input.collectors.map((collector) => {
    const parsed = parseLocation(collector.location)
    const covered = locatedServers.filter(item => collectorCovers(collector, item.server, input.paths)).map(item => item.point)
    const point = spread(parsed || sphericalMean(covered) || anchor, taken, !parsed)
    const assigned = input.paths.filter(path => path.observer_id === collector.id)
    const connected = assigned.filter(path => path.sink.connected)
    return {
      id: collector.id,
      kind: 'collector' as const,
      name: collector.name,
      lon: point.lon,
      lat: point.lat,
      derived: !parsed,
      status: collectorStatus(collector),
      href: `/connections?observer_id=${encodeURIComponent(collector.id)}`,
      coverage: `${connected.length}/${assigned.length}`,
      rttMs: collector.heartbeat_rtt_ms,
      count: 1,
      names: [collector.name],
      onlines: [collectorStatus(collector) === 'online'],
    }
  })

  const links: TopologyLink[] = []
  for (const collector of collectors) {
    const source = input.collectors.find(row => row.id === collector.id)
    links.push({
      fromId: collector.id,
      toId: 'primary',
      connected: collector.status === 'online',
      kind: 'replication',
      rttMs: source?.heartbeat_rtt_ms,
    })
  }
  for (const path of input.paths) {
    const fromId = serverMarkerId.get(path.server_id)
    if (!fromId) continue
    const toId = path.observer_kind === 'primary' ? 'primary' : path.observer_id
    if (toId !== 'primary' && !collectors.some(item => item.id === toId)) continue
    links.push({
      fromId,
      toId,
      connected: path.sink.connected,
      kind: 'path',
      rttMs: path.sink.last_rtt_ms,
    })
  }

  const assigned = input.paths.length
  const connected = input.paths.filter(path => path.sink.connected).length
  primary.coverage = `${connected}/${assigned}`

  return {
    primary,
    collectors,
    nodes: [...nodeGroups.values()].map(group => group.marker),
    unlocated,
    countries: [...countries],
    links,
    collectorsOnline: input.collectors.filter(row => !row.revoked && row.status === 'online').length,
    collectorsTotal: input.collectors.length,
    pathsConnected: connected,
    pathsAssigned: assigned,
  }
}

export function allMarkers(graph: TopologyGraph): TopologyMarker[] {
  return [graph.primary, ...graph.collectors, ...graph.nodes]
}

export interface NodeLatencyRow {
  id: string
  name: string
  online: boolean
  rttMs?: number
}

/** 节点相对主面板的链路延迟。主机离线或主端未连接都算离线。 */
export function primaryLatencyRows(servers: ServerRecord[], paths: ConnectionPath[]): NodeLatencyRow[] {
  const primaryByServer = new Map<number, ConnectionPath>()
  for (const path of paths) {
    if (path.observer_kind !== 'primary') continue
    primaryByServer.set(path.server_id, path)
  }
  return [...servers]
    .sort((a, b) => (a.display_index - b.display_index) || a.id - b.id)
    .map((server) => {
      const path = primaryByServer.get(server.id)
      const online = Boolean(server.online && path?.sink.connected)
      return {
        id: String(server.id),
        name: server.name,
        online,
        rttMs: online ? path?.sink.last_rtt_ms : undefined,
      }
    })
}
