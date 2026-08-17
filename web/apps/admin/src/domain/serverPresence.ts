export type HostListTone = 'online' | 'degraded' | 'offline' | ''

export type HostListServer = {
  online?: boolean
  telemetry?: {
    connectivity?: string | null
    available?: boolean | null
  } | null
}

export function hostListTone(server: HostListServer): HostListTone {
  const connectivity = server.telemetry?.connectivity
  if (connectivity === 'full') return 'online'
  if (connectivity === 'partial') return 'degraded'
  if (connectivity === 'unavailable') return 'offline'
  if (connectivity === 'unknown') return ''
  return server.online ? 'online' : 'offline'
}

export function hostCoverageTone(server: HostListServer): 'is-ok' | 'is-warn' | 'is-bad' | 'is-unknown' {
  const connectivity = server.telemetry?.connectivity
  if (connectivity === 'full') return 'is-ok'
  if (connectivity === 'partial') return 'is-warn'
  if (connectivity === 'unavailable') return 'is-bad'
  if (connectivity === 'unknown') return 'is-unknown'
  if (server.telemetry?.available === false) return 'is-bad'
  if (server.telemetry?.available === true) return 'is-ok'
  return 'is-unknown'
}

export function hostCoverageIcon(tone: ReturnType<typeof hostCoverageTone>): string {
  if (tone === 'is-ok') return 'ri-checkbox-circle-fill'
  if (tone === 'is-warn') return 'ri-error-warning-fill'
  if (tone === 'is-bad') return 'ri-close-circle-fill'
  return 'ri-question-fill'
}
