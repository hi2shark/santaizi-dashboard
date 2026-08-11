import { describe, expect, it } from 'vitest'
import { formatAdminValue, formatAPIError, formatBytes } from './format'

const values: Record<string, string> = {
  yes: '是', no: '否', healthy: '健康', loadFailed: '加载失败', requestFailedWithCode: '请求失败（错误码：x）',
  'errors.authentication_required': '登录已过期，请重新登录',
}
const t = (key: string) => values[key] || key
const te = (key: string) => key in values

describe('localized value formatting', () => {
  it('formats bytes and protocol states without exposing raw values', () => {
    expect(formatBytes(1536, 'zh-CN')).toBe('1.5 KiB')
    expect(formatAdminValue('healthy', 'status', 'zh-CN', t, te)).toBe('健康')
    expect(formatAdminValue(true, 'active', 'zh-CN', t, te)).toBe('是')
  })

  it('uses stable problem codes for localized API errors', () => {
    expect(formatAPIError({ code: 'authentication_required' }, t, te)).toBe('登录已过期，请重新登录')
  })
})
