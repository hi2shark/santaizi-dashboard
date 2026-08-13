export const LONG_ID_MIN = 16

export function shortId(value: string, keep = 8) {
  const text = value.trim()
  if (text.length < LONG_ID_MIN) return text
  return `${text.slice(0, keep)}…`
}

export function isLongId(value: unknown): value is string {
  return typeof value === 'string' && value.trim().length >= LONG_ID_MIN
}
