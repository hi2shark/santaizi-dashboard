import type { ResourceRecord, ServerRecord } from '@santaizi/api'
import { buildPublicNoteView, decodeOrderLink, type PublicNoteView } from '@santaizi/theme-server-status'
import { formatBinary, stateValue } from '../utils/host'
import { resolveServerLocation, type ServerLocation } from '../utils/worldMap'

export interface NazhuaCycleTransferView {
  name: string
  direction: string
  usedBytes: number
  quotaBytes: number
  usagePercent: number
  status: string
  windowStart: string
  windowEnd: string
}

export type NazhuaCycleTransferMap = Map<number, NazhuaCycleTransferView>

export interface NazhuaServerView {
  id: number
  source: ServerRecord
  name: string
  group: string
  online: boolean
  flagCode: string
  flagClass: string
  location: ServerLocation | null
  publicNote: PublicNoteView
  slogan: string
  spec: string
  cpuPercent: number
  memoryPercent: number
  diskPercent: number
  memoryValue: string
  diskValue: string
  uptimeSeconds: number
  uptime: string
  speedIn: number
  speedOut: number
  transferIn: number
  transferOut: number
  trafficBytes: number
  cycle: NazhuaCycleTransferView | null
  billing: string
  orderLink: string
}

function finite(value: unknown) {
  const number = Number(value)
  return Number.isFinite(number) ? number : 0
}

function objectValue(source: Record<string, unknown> | undefined, ...keys: string[]) {
  for (const key of keys) {
    const value = source?.[key]
    if (value !== undefined && value !== null && String(value).trim() !== '') return value
  }
  return undefined
}

export function clampPercent(value: number) {
  return Math.max(0, Math.min(100, Number.isFinite(value) ? value : 0))
}

export function percentOf(used: number, total?: number) {
  return clampPercent(total && total > 0 ? (used / total) * 100 : used)
}

export function formatCompactBytes(value: number, decimals = 0) {
  const result = formatBinary(value, decimals)
  return `${result.value}${result.unit}`
}

export function formatUptime(seconds: number) {
  const safe = Math.max(0, Math.floor(seconds))
  const days = Math.floor(safe / 86_400)
  if (days > 0) return `${days}`
  const hours = Math.floor(safe / 3_600)
  if (hours > 0) return `${hours}h`
  const minutes = Math.floor(safe / 60)
  return `${minutes}m`
}

function readServerId(row: ResourceRecord) {
  return Math.trunc(finite(row.server_id ?? row.serverId))
}

export function mapCycleTransfers(rows: ResourceRecord[]): NazhuaCycleTransferMap {
  const result = new Map<number, NazhuaCycleTransferView>()
  for (const row of rows) {
    const serverId = readServerId(row)
    if (!serverId) continue
    const usedBytes = Math.max(0, finite(row.used_bytes ?? row.usedBytes))
    const quotaBytes = Math.max(0, finite(row.quota_bytes ?? row.quotaBytes))
    const current = result.get(serverId)
    if (current) {
      current.usedBytes += usedBytes
      current.quotaBytes += quotaBytes
      current.usagePercent = percentOf(current.usedBytes, current.quotaBytes)
      if (String(row.status || '') && row.status !== 'normal') current.status = String(row.status)
      continue
    }
    result.set(serverId, {
      name: String(row.name || ''),
      direction: String(row.direction || row.mode || ''),
      usedBytes,
      quotaBytes,
      usagePercent: quotaBytes > 0 ? percentOf(usedBytes, quotaBytes) : clampPercent(finite(row.usage_percent)),
      status: String(row.status || 'normal'),
      windowStart: String(row.window_start || ''),
      windowEnd: String(row.window_end || ''),
    })
  }
  return result
}

function billingLabel(note: PublicNoteView) {
  if (note.bill.amountKind === 'free') return '0'
  if (note.bill.amountKind === 'metered') return '∞'
  return note.bill.amountValue
}

export function toNazhuaServerView(
  server: ServerRecord,
  cycles?: NazhuaCycleTransferMap,
  nowMs = Date.now(),
): NazhuaServerView {
  const state = server.state || {}
  const host = server.host || {}
  const memoryUsed = stateValue(state, 'MemUsed', 'mem_used')
  const memoryTotal = stateValue(state, 'MemTotal', 'mem_total')
  const diskUsed = stateValue(state, 'DiskUsed', 'disk_used')
  const diskTotal = stateValue(state, 'DiskTotal', 'disk_total')
  const transferIn = stateValue(state, 'NetInTransfer', 'net_in_transfer')
  const transferOut = stateValue(state, 'NetOutTransfer', 'net_out_transfer')
  const location = resolveServerLocation(server)
  const publicNote = buildPublicNoteView(server.public_note, nowMs)
  const flagCode = (publicNote.presentation.flag || location?.countryCode || '').toLowerCase()
  const cpuCores = finite(objectValue(host, 'CPU', 'cpu', 'Core', 'core'))
  const spec = [
    cpuCores > 0 ? `${cpuCores}C` : '',
    memoryTotal > 0 ? formatCompactBytes(memoryTotal) : '',
    diskTotal > 0 ? formatCompactBytes(diskTotal) : '',
  ].filter(Boolean).join('')
  const cycle = cycles?.get(server.id) || null
  return {
    id: server.id,
    source: server,
    name: server.name,
    group: server.tag || 'default',
    online: server.online === true,
    flagCode,
    flagClass: flagCode ? `fi fi-${flagCode}` : '',
    location,
    publicNote,
    slogan: publicNote.presentation.slogan,
    spec,
    cpuPercent: percentOf(stateValue(state, 'CPU', 'Cpu', 'cpu')),
    memoryPercent: percentOf(memoryUsed, memoryTotal || undefined),
    diskPercent: percentOf(diskUsed, diskTotal || undefined),
    memoryValue: formatCompactBytes(memoryUsed),
    diskValue: formatCompactBytes(diskUsed),
    uptimeSeconds: stateValue(state, 'Uptime', 'uptime'),
    uptime: formatUptime(stateValue(state, 'Uptime', 'uptime')),
    speedIn: stateValue(state, 'NetInSpeed', 'net_in_speed'),
    speedOut: stateValue(state, 'NetOutSpeed', 'net_out_speed'),
    transferIn,
    transferOut,
    trafficBytes: cycle && cycle.quotaBytes > 0
      ? Math.max(cycle.quotaBytes - cycle.usedBytes, 0)
      : transferIn + transferOut,
    cycle,
    billing: billingLabel(publicNote),
    orderLink: decodeOrderLink(publicNote.presentation.orderLink),
  }
}

export function toNazhuaServerViews(servers: ServerRecord[], cycles?: NazhuaCycleTransferMap) {
  return servers.map(server => toNazhuaServerView(server, cycles))
}
