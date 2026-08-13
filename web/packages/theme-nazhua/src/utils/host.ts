export function calcBinary(value: number) {
  const units = { value: 0, k: 0, m: 0, g: 0, t: 0 }
  let n = Math.max(0, value)
  units.value = n
  if (n >= 1024) {
    units.k = n / 1024
    n = units.k
  }
  if (n >= 1024) {
    units.m = n / 1024
    n = units.m
  }
  if (n >= 1024) {
    units.g = n / 1024
    n = units.g
  }
  if (n >= 1024) units.t = n / 1024
  return units
}

export function formatBinary(value: number, decimals = 1) {
  const units = calcBinary(value)
  if (units.t >= 1) return { value: units.t.toFixed(decimals), unit: 'T' }
  if (units.g >= 1) return { value: units.g.toFixed(decimals), unit: 'G' }
  if (units.m >= 1) return { value: units.m.toFixed(decimals), unit: 'M' }
  if (units.k >= 1) return { value: units.k.toFixed(decimals), unit: 'K' }
  return { value: String(Math.round(units.value)), unit: 'B' }
}

export function formatSpeed(value: number) {
  const formatted = formatBinary(value)
  return `${formatted.value} ${formatted.unit}/s`
}

export function formatPercent(current: number, total?: number) {
  const pct = total ? (100 * current) / total : current
  return `${Math.max(0, Math.min(100, pct)).toFixed(1)}%`
}

export function formatDateTime(value: unknown, locale = 'zh-CN') {
  if (value === null || value === undefined || value === '') return ''
  let date: Date | null = null
  if (value instanceof Date) date = value
  else if (typeof value === 'number' && Number.isFinite(value)) {
    date = new Date(value > 1e12 ? value : value * 1000)
  } else {
    const parsed = Date.parse(String(value))
    if (!Number.isNaN(parsed)) date = new Date(parsed)
  }
  if (!date || date.getUTCFullYear() <= 1) return ''
  return new Intl.DateTimeFormat(locale, { dateStyle: 'medium', timeStyle: 'short' }).format(date)
}
