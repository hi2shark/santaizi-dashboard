import type { ProbePath } from '@santaizi/api'

export function probePathKey(path: ProbePath) {
  return `${path.server_id}:${path.collector_id}`
}

export function probeHasNoTarget(path: ProbePath) {
  return path.target?.source === 'none'
}

export function probeTargetText(path: ProbePath) {
  return path.target?.hostname || path.target?.ipv4 || path.target?.ipv6 || '—'
}

export function formatProbeLoss(value: number | null | undefined, locale: string) {
  if (value == null || !Number.isFinite(value)) return '—'
  const percent = value <= 1 ? value * 100 : value
  return `${new Intl.NumberFormat(locale, { maximumFractionDigits: 1 }).format(percent)}%`
}
