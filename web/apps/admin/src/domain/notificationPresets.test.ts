import { describe, expect, it } from 'vitest'
import {
  NOTIFICATION_PRESETS,
  applyNotificationPreset,
  getNotificationPreset,
} from './notificationPresets'

describe('notificationPresets', () => {
  it('has unique ids', () => {
    const ids = NOTIFICATION_PRESETS.map(item => item.id)
    expect(new Set(ids).size).toBe(ids.length)
  })

  it('keeps form bodies as JSON object maps', () => {
    const telegram = getNotificationPreset('telegram')
    expect(telegram).toBeDefined()
    expect(telegram!.request_type).toBe('form')
    const parsed = JSON.parse(telegram!.body) as Record<string, string>
    expect(parsed.chat_id).toBe('xxxxxx')
    expect(parsed.text).toBe('#SANTAIZI#')
  })

  it('clears body for GET presets', () => {
    const bark = getNotificationPreset('bark')!
    const patch = applyNotificationPreset(bark, '', key => key)
    expect(patch.method).toBe('get')
    expect(patch.body).toBe('')
    expect(bark.url).toContain('#SANTAIZI#')
  })

  it('fills name only when empty', () => {
    const wecom = getNotificationPreset('wecom')!
    const empty = applyNotificationPreset(wecom, '  ', key => `translated:${key}`)
    expect(empty.name).toBe('translated:presetWeComName')
    const kept = applyNotificationPreset(wecom, '已有名称', key => `translated:${key}`)
    expect(kept.name).toBeUndefined()
    expect(kept.url).toContain('qyapi.weixin.qq.com')
  })

  it('includes 三太子 keyword in DingTalk body', () => {
    const body = getNotificationPreset('dingtalk')!.body
    expect(body).toContain('三太子')
    expect(body).toContain('#SANTAIZI#')
  })
})
