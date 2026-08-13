export type PublicThemeId = 'server-status' | 'nazhua'

export function normalizePublicTheme(value: unknown): PublicThemeId {
  return value === 'nazhua' ? 'nazhua' : 'server-status'
}

export function resolvePublicTheme(options: {
  siteTheme?: unknown
  allowSwitch?: boolean
  stored?: PublicThemeId | null
}): PublicThemeId {
  const site = normalizePublicTheme(options.siteTheme)
  if (options.allowSwitch === false) return site
  return options.stored || site
}
