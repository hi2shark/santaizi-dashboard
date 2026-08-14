export const PRODUCT_REPO_URL = 'https://github.com/hi2shark/santaizi-dashboard'

export function formatProductVersion(raw?: string) {
  const value = (raw || '').trim()
  if (!value) return ''
  if (/^v/i.test(value)) return value
  if (/^\d/.test(value)) return `v${value}`
  return value
}
