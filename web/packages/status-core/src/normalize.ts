import type { ServerHost, ServerRecord, ServerState } from '@santaizi/api'

function pick(object: Record<string, unknown>, ...keys: string[]) {
  for (const key of keys) {
    if (object[key] !== undefined && object[key] !== null) return object[key]
  }
  return undefined
}

function publicNote(value: unknown): Record<string, unknown> {
  if (value && typeof value === 'object') return value as Record<string, unknown>
  if (typeof value === 'string' && value.trim()) {
    try {
      return JSON.parse(value) as Record<string, unknown>
    } catch {
      return { text: value }
    }
  }
  return {}
}

function asRecord(value: unknown): Record<string, unknown> | undefined {
  return value && typeof value === 'object' ? value as Record<string, unknown> : undefined
}

function asNumber(raw: unknown, fallback = 0) {
  const n = typeof raw === 'number' ? raw : Number(raw)
  return Number.isFinite(n) ? n : fallback
}

function deriveOnline(raw: unknown, lastActive: string) {
  if (typeof raw === 'boolean') return raw
  if (!lastActive) return false
  const ts = Date.parse(lastActive)
  if (Number.isNaN(ts)) return false
  return Date.now() - ts < 30_000
}

export function normalizeServer(raw: Record<string, unknown>): ServerRecord {
  const value = (
    raw.data && typeof raw.data === 'object' && !Array.isArray(raw.data)
    && (pick(raw.data as Record<string, unknown>, 'id', 'ID', 'name', 'Name') !== undefined)
      ? raw.data as Record<string, unknown>
      : raw
  )
  const lastActive = String(pick(value, 'last_active', 'LastActive') ?? '')
  const telemetryRaw = asRecord(pick(value, 'telemetry', 'Telemetry'))
  return {
    id: asNumber(pick(value, 'id', 'ID')),
    name: String(pick(value, 'name', 'Name') ?? ''),
    tag: String(pick(value, 'tag', 'Tag') ?? ''),
    display_index: asNumber(pick(value, 'display_index', 'DisplayIndex')),
    hide_for_guest: Boolean(pick(value, 'hide_for_guest', 'HideForGuest')),
    enable_ddns: Boolean(pick(value, 'enable_ddns', 'EnableDDNS')),
    public_note: publicNote(pick(value, 'public_note', 'PublicNote')),
    host: asRecord(pick(value, 'host', 'Host')) as ServerHost | undefined,
    state: asRecord(pick(value, 'state', 'State')) as ServerState | undefined,
    last_active: lastActive,
    online: deriveOnline(pick(value, 'online', 'Online'), lastActive),
    telemetry: telemetryRaw
      ? {
          host: String(pick(telemetryRaw, 'host', 'Host') ?? ''),
          connectivity: String(pick(telemetryRaw, 'connectivity', 'Connectivity') ?? ''),
          available: (pick(telemetryRaw, 'available', 'Available') as boolean | null | undefined) ?? null,
          coverage: String(pick(telemetryRaw, 'coverage', 'Coverage') ?? ''),
        }
      : undefined,
  }
}

function isEmptyRecord(value: ServerHost | ServerState | Record<string, unknown> | undefined) {
  return !value || Object.keys(value).length === 0
}

function mergePublicNote(
  prev: Record<string, unknown> | undefined,
  next: Record<string, unknown> | undefined,
): Record<string, unknown> {
  if (isEmptyRecord(next)) return prev || {}
  if (isEmptyRecord(prev)) return next || {}
  const prevNote = prev || {}
  const nextNote = next || {}
  const mergeSection = (key: string) => {
    const prevSection = asRecord(prevNote[key])
    const nextSection = asRecord(nextNote[key])
    if (isEmptyRecord(nextSection)) return prevSection || nextNote[key]
    if (isEmptyRecord(prevSection)) return nextSection
    return { ...prevSection, ...nextSection }
  }
  return {
    ...prevNote,
    ...nextNote,
    billingDataMod: mergeSection('billingDataMod'),
    planDataMod: mergeSection('planDataMod'),
    customData: mergeSection('customData'),
  }
}

/** WS 帧可能缺 public_note/name；用已有 HTTP 快照补齐，避免整表覆盖冲掉公开备注 */
export function mergeServerSnapshot(prev: ServerRecord | undefined, next: ServerRecord): ServerRecord {
  if (!prev) return next
  return {
    ...prev,
    ...next,
    name: next.name || prev.name,
    tag: next.tag || prev.tag,
    public_note: mergePublicNote(prev.public_note, next.public_note),
    host: isEmptyRecord(next.host) ? prev.host : next.host,
    state: isEmptyRecord(next.state) ? prev.state : next.state,
    telemetry: next.telemetry || prev.telemetry,
  }
}
