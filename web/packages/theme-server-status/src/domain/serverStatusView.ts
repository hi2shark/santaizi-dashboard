import type { CycleTransfer, ResourceRecord, ServerHost, ServerRecord, ServerState } from '@santaizi/api'
import {
  buildPublicNoteView,
  decodeOrderLink,
  flagCode,
  publicLocation,
  type PublicNoteView,
} from './publicNoteView'

export interface MetricView {
  percent: number
  used: number
  total: number
  usedLabel: string
  totalLabel: string
}

export interface TempView {
  name: string
  value: number
}

export interface CycleTransferView {
  policyId: number
  name: string
  direction: string
  usedBytes: number
  quotaBytes: number
  usedLabel: string
  quotaLabel: string
  usagePercent: number
  status: string
}

export type CycleTransferMap = Map<number, CycleTransferView[]>

export interface ServerStatusView {
  id: number
  source: ServerRecord
  name: string
  group: string
  online: boolean
  flagCode: string
  location: string
  slogan: string
  publicNote: PublicNoteView
  orderLink: string
  cpu: MetricView
  memory: MetricView
  disk: MetricView
  gpu: MetricView | null
  gpuNames: string[]
  cpuModels: string[]
  speedIn: number
  speedOut: number
  speedInLabel: string
  speedOutLabel: string
  transferIn: number
  transferOut: number
  transferTotalLabel: string
  cycles: CycleTransferView[]
  uptimeSeconds: number
  load1: number
  load5: number
  load15: number
  hasLoad: boolean
  platform: string
  platformVersion: string
  arch: string
  virtualization: string
  agentVersion: string
  bootTime: number
  swap: MetricView | null
  tcp: number | null
  udp: number | null
  processes: number | null
  temperatures: TempView[]
  available: boolean | null
  hasSpecs: boolean
}

function finite(value: unknown, fallback = 0) {
  const n = typeof value === 'number' ? value : Number(value)
  return Number.isFinite(n) ? n : fallback
}

type FieldSource = ServerHost | ServerState | Record<string, unknown> | undefined

export function pick(object: FieldSource, ...keys: string[]) {
  if (!object) return undefined
  const source = object as Record<string, unknown>
  for (const key of keys) {
    const value = source[key]
    if (value !== undefined && value !== null) return value
  }
  return undefined
}

export function pickNumber(object: FieldSource, ...keys: string[]) {
  return finite(pick(object, ...keys))
}

export function pickOptionalNumber(object: FieldSource, ...keys: string[]) {
  const value = pick(object, ...keys)
  if (value === undefined || value === null || value === '') return null
  return finite(value)
}

export function pickText(object: FieldSource, ...keys: string[]) {
  const value = pick(object, ...keys)
  if (value === undefined || value === null) return ''
  return String(value).trim()
}

export function clampPercent(value: number) {
  return Math.max(0, Math.min(100, Number.isFinite(value) ? value : 0))
}

export function percentOf(used: number, total?: number) {
  return clampPercent(total && total > 0 ? (used / total) * 100 : used)
}

export function formatBytes(value: number, decimals = 1) {
  let n = Math.max(0, finite(value))
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  while (n >= 1024 && i < 4) {
    n /= 1024
    i += 1
  }
  return `${n.toFixed(i ? decimals : 0)} ${units[i]}`
}

function metric(used: number, total: number, usedIsPercent = false): MetricView {
  const percent = usedIsPercent ? clampPercent(used) : percentOf(used, total || undefined)
  return {
    percent,
    used,
    total,
    usedLabel: formatBytes(used),
    totalLabel: formatBytes(total),
  }
}

function textList(value: unknown) {
  if (Array.isArray(value)) return value.map((item) => String(item).trim()).filter(Boolean)
  if (typeof value === 'string' && value.trim()) return [value.trim()]
  return []
}

function temperatures(raw: unknown): TempView[] {
  if (!Array.isArray(raw)) return []
  const rows: TempView[] = []
  for (const item of raw) {
    if (!item || typeof item !== 'object') continue
    const row = item as Record<string, unknown>
    const value = finite(pick(row, 'Temperature', 'temperature'))
    const name = pickText(row, 'Name', 'name')
    if (!name && value === 0) continue
    rows.push({ name, value })
  }
  return rows
}

function readServerId(row: ResourceRecord | CycleTransfer) {
  const rec = row as ResourceRecord
  return Math.trunc(finite(row.server_id ?? rec.serverId ?? rec.ServerID))
}

export function mapCycleTransfers(rows: Array<ResourceRecord | CycleTransfer>): CycleTransferMap {
  const result = new Map<number, CycleTransferView[]>()
  for (const row of rows) {
    const rec = row as ResourceRecord
    const serverId = readServerId(row)
    if (!serverId) continue
    const usedBytes = Math.max(0, finite(row.used_bytes ?? rec.usedBytes))
    const quotaBytes = Math.max(0, finite(row.quota_bytes ?? rec.quotaBytes))
    const list = result.get(serverId) || []
    list.push({
      policyId: Math.trunc(finite(row.policy_id ?? rec.policyId ?? rec.id)),
      name: String(row.name || ''),
      direction: String(row.direction || rec.mode || ''),
      usedBytes,
      quotaBytes,
      usedLabel: formatBytes(usedBytes),
      quotaLabel: formatBytes(quotaBytes),
      usagePercent: quotaBytes > 0 ? percentOf(usedBytes, quotaBytes) : clampPercent(finite(row.usage_percent ?? rec.usagePercent)),
      status: String(row.status || 'normal'),
    })
    result.set(serverId, list)
  }
  return result
}

export function toServerStatusView(
  server: ServerRecord,
  cycles?: CycleTransferMap,
  nowMs = Date.now(),
  locale = 'zh-CN',
): ServerStatusView {
  const state = server.state || {}
  const host = server.host || {}
  const publicNote = buildPublicNoteView(server.public_note, nowMs)
  const memUsed = pickNumber(state, 'MemUsed', 'mem_used')
  const memTotal = pickNumber(host, 'MemTotal', 'mem_total') || pickNumber(state, 'MemTotal', 'mem_total')
  const diskUsed = pickNumber(state, 'DiskUsed', 'disk_used')
  const diskTotal = pickNumber(host, 'DiskTotal', 'disk_total') || pickNumber(state, 'DiskTotal', 'disk_total')
  const swapUsed = pickNumber(state, 'SwapUsed', 'swap_used')
  const swapTotal = pickNumber(host, 'SwapTotal', 'swap_total') || pickNumber(state, 'SwapTotal', 'swap_total')
  const gpuPercent = pickNumber(state, 'GPU', 'Gpu', 'gpu')
  const gpuNames = textList(pick(host, 'GPU', 'gpu'))
  const cpuModels = textList(pick(host, 'CPU', 'cpu')).filter((item) => Number.isNaN(Number(item)) || item.includes(' '))
  const load1 = pickNumber(state, 'Load1', 'load1')
  const load5 = pickNumber(state, 'Load5', 'load5')
  const load15 = pickNumber(state, 'Load15', 'load15')
  const tcp = pickOptionalNumber(state, 'TcpConnCount', 'tcp_conn_count')
  const udp = pickOptionalNumber(state, 'UdpConnCount', 'udp_conn_count')
  const processes = pickOptionalNumber(state, 'ProcessCount', 'process_count')
  const temps = temperatures(pick(state, 'Temperatures', 'temperatures'))
  const agentVersion = pickText(host, 'Version', 'version')
  const bootTime = pickNumber(host, 'BootTime', 'boot_time')
  const available = server.telemetry?.available ?? null
  const swap = swapTotal > 0 ? metric(swapUsed, swapTotal) : null
  const gpu = gpuPercent > 0 ? metric(gpuPercent, 0, true) : null
  const transferIn = pickNumber(state, 'NetInTransfer', 'net_in_transfer')
  const transferOut = pickNumber(state, 'NetOutTransfer', 'net_out_transfer')
  const speedIn = pickNumber(state, 'NetInSpeed', 'net_in_speed')
  const speedOut = pickNumber(state, 'NetOutSpeed', 'net_out_speed')
  const country = pickText(host, 'CountryCode', 'country_code')
  const hasSpecs = Boolean(
    cpuModels.length
    || gpuNames.length
    || swap
    || tcp !== null
    || udp !== null
    || processes !== null
    || temps.length
    || agentVersion
    || bootTime,
  )
  return {
    id: server.id,
    source: server,
    name: server.name,
    group: server.tag || 'default',
    online: server.online === true,
    flagCode: flagCode(server.public_note, country),
    location: publicLocation(server.public_note, country, locale),
    slogan: publicNote.presentation.slogan,
    publicNote,
    orderLink: decodeOrderLink(publicNote.presentation.orderLink),
    cpu: metric(pickNumber(state, 'CPU', 'Cpu', 'cpu'), 0, true),
    memory: metric(memUsed, memTotal),
    disk: metric(diskUsed, diskTotal),
    gpu,
    gpuNames,
    cpuModels,
    speedIn,
    speedOut,
    speedInLabel: `${formatBytes(speedIn)}/s`,
    speedOutLabel: `${formatBytes(speedOut)}/s`,
    transferIn,
    transferOut,
    transferTotalLabel: formatBytes(transferIn + transferOut),
    cycles: cycles?.get(server.id) || [],
    uptimeSeconds: pickNumber(state, 'Uptime', 'uptime'),
    load1,
    load5,
    load15,
    hasLoad: pick(state, 'Load1', 'load1', 'Load5', 'load5', 'Load15', 'load15') !== undefined,
    platform: pickText(host, 'Platform', 'platform'),
    platformVersion: pickText(host, 'PlatformVersion', 'platform_version'),
    arch: pickText(host, 'Arch', 'arch'),
    virtualization: pickText(host, 'Virtualization', 'virtualization'),
    agentVersion,
    bootTime,
    swap,
    tcp,
    udp,
    processes,
    temperatures: temps,
    available,
    hasSpecs,
  }
}

export function toServerStatusViews(
  servers: ServerRecord[],
  cycles?: CycleTransferMap,
  nowMs = Date.now(),
  locale = 'zh-CN',
) {
  return [...servers]
    .sort((a, b) => b.display_index - a.display_index)
    .map((server) => toServerStatusView(server, cycles, nowMs, locale))
}
