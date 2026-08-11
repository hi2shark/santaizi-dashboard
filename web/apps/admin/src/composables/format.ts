export type Translate = (key: string, ...args: unknown[]) => string

export function formatBytes(value: unknown, locale: string) {
  let bytes = Number(value || 0)
  if (!Number.isFinite(bytes)) return '—'
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB']
  let index = 0
  while (Math.abs(bytes) >= 1024 && index < units.length - 1) {
    bytes /= 1024
    index++
  }
  return `${new Intl.NumberFormat(locale, { maximumFractionDigits: index ? 1 : 0 }).format(bytes)} ${units[index]}`
}

export function formatDateTime(value: unknown, locale: string) {
  if (value === null || value === undefined || value === '') return '—'
  const date = new Date(String(value))
  if (Number.isNaN(date.valueOf())) return String(value)
  if (date.getUTCFullYear() <= 1) return '—'
  return new Intl.DateTimeFormat(locale, { dateStyle: 'medium', timeStyle: 'medium' }).format(date)
}

export function formatAdminValue(value: unknown, key: string, locale: string, t: Translate, te: (key: string) => boolean) {
  if (value === null || value === undefined || value === '') return '—'
  if (typeof value === 'boolean') return t(value ? 'yes' : 'no')
  if (Array.isArray(value)) return value.join(', ')
  if (typeof value === 'object') return JSON.stringify(value)
  if (/(?:bytes|spool_size)$/.test(key)) return formatBytes(value, locale)
  if (/(?:_at|_from|_to|last_seen|last_active)$/.test(key)) return formatDateTime(value, locale)
  if (typeof value === 'string' && te(value)) return t(value)
  return String(value)
}

export interface ExtractedAPIError {
  code: string
  status?: number
  traceId?: string
  fields?: Record<string, string[]>
  detail?: string
  message?: string
}

export function extractAPIError(error: unknown): ExtractedAPIError {
  if (typeof error !== 'object' || error === null) return { code: '' }
  const record = error as {
    code?: unknown
    status?: unknown
    traceId?: unknown
    fields?: unknown
    message?: unknown
  }
  const code = record.code != null ? String(record.code) : ''
  const status = typeof record.status === 'number' ? record.status : undefined
  const traceId = record.traceId != null ? String(record.traceId) : undefined
  const fields = record.fields && typeof record.fields === 'object'
    ? record.fields as Record<string, string[]>
    : undefined
  const message = record.message != null ? String(record.message) : undefined
  return { code, status, traceId, fields, detail: message, message }
}

export function formatAPIError(error: unknown, t: Translate, te: (key: string) => boolean) {
  const { code } = extractAPIError(error)
  const key = `errors.${code}`
  if (code && te(key)) return t(key)
  return code ? t('requestFailedWithCode', { code }) : t('loadFailed')
}
