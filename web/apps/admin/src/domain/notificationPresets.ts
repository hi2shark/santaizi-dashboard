import type { KeyValueRow, NotificationChannelRecord } from '@/types/admin'

export type NotificationPresetId =
  | 'custom'
  | 'wecom'
  | 'dingtalk'
  | 'feishu'
  | 'telegram'
  | 'bark'
  | 'discord'
  | 'slack'

export interface NotificationPreset {
  id: NotificationPresetId
  /** i18n key for the chip label */
  labelKey: string
  /** i18n key for default channel name (applied only when name is empty) */
  nameKey: string
  url: string
  method: NotificationChannelRecord['method']
  request_type: NotificationChannelRecord['request_type']
  headers: KeyValueRow[]
  body: string
}

const pretty = (value: unknown) => `${JSON.stringify(value, null, 2)}\n`

export const NOTIFICATION_PRESETS: readonly NotificationPreset[] = [
  {
    id: 'custom',
    labelKey: 'presetCustom',
    nameKey: 'presetCustomName',
    url: 'https://example.com/webhook',
    method: 'post',
    request_type: 'json',
    headers: [],
    body: '',
  },
  {
    id: 'wecom',
    labelKey: 'presetWeCom',
    nameKey: 'presetWeComName',
    url: 'https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxxxxxxx',
    method: 'post',
    request_type: 'json',
    headers: [],
    body: pretty({
      msgtype: 'text',
      text: { content: '#SANTAIZI#' },
    }),
  },
  {
    id: 'dingtalk',
    labelKey: 'presetDingTalk',
    nameKey: 'presetDingTalkName',
    url: 'https://oapi.dingtalk.com/robot/send?access_token=xxxxxxxx',
    method: 'post',
    request_type: 'json',
    headers: [],
    body: pretty({
      msgtype: 'text',
      text: { content: '三太子：\n#SANTAIZI#' },
    }),
  },
  {
    id: 'feishu',
    labelKey: 'presetFeishu',
    nameKey: 'presetFeishuName',
    url: 'https://open.feishu.cn/open-apis/bot/v2/hook/xxxxxxxx',
    method: 'post',
    request_type: 'json',
    headers: [],
    body: pretty({
      msg_type: 'text',
      content: { text: '#SANTAIZI#\n#DATETIME#' },
    }),
  },
  {
    id: 'telegram',
    labelKey: 'presetTelegram',
    nameKey: 'presetTelegramName',
    url: 'https://api.telegram.org/bot<token>/sendMessage',
    method: 'post',
    request_type: 'form',
    headers: [],
    // Form bodies are stored as a JSON string map (backend GjsonParseStringMap).
    body: pretty({
      chat_id: 'xxxxxx',
      text: '#SANTAIZI#',
    }),
  },
  {
    id: 'bark',
    labelKey: 'presetBark',
    nameKey: 'presetBarkName',
    url: 'https://api.day.app/xxxxxxxx/#SANTAIZI#',
    method: 'get',
    request_type: 'json',
    headers: [],
    body: '',
  },
  {
    id: 'discord',
    labelKey: 'presetDiscord',
    nameKey: 'presetDiscordName',
    url: 'https://discord.com/api/webhooks/xxxxxxxx/xxxxxxxx',
    method: 'post',
    request_type: 'json',
    headers: [],
    body: pretty({ content: '#SANTAIZI#' }),
  },
  {
    id: 'slack',
    labelKey: 'presetSlack',
    nameKey: 'presetSlackName',
    url: 'https://hooks.slack.com/services/XXXXXXXXX/XXXXXXXXX/XXXXXXXXXXXXXXXXXXXXXXXX',
    method: 'post',
    request_type: 'json',
    headers: [],
    body: pretty({ text: '#SANTAIZI#' }),
  },
]

export function getNotificationPreset(id: NotificationPresetId): NotificationPreset | undefined {
  return NOTIFICATION_PRESETS.find(item => item.id === id)
}

export interface NotificationPresetFormPatch {
  url: string
  method: NotificationChannelRecord['method']
  request_type: NotificationChannelRecord['request_type']
  body: string
  headers: KeyValueRow[]
  /** Present only when the current name is empty and should be filled */
  name?: string
}

/** Build form fields for a preset. Name is filled only when `currentName` is blank. */
export function applyNotificationPreset(
  preset: NotificationPreset,
  currentName: string,
  resolveName: (nameKey: string) => string,
): NotificationPresetFormPatch {
  const patch: NotificationPresetFormPatch = {
    url: preset.url,
    method: preset.method,
    request_type: preset.request_type,
    body: preset.method === 'get' ? '' : preset.body,
    headers: preset.headers.map(item => ({ ...item })),
  }
  if (!currentName.trim()) patch.name = resolveName(preset.nameKey)
  return patch
}
