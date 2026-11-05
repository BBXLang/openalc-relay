/*
 * 正式 UI 的展示模型。当前正式数据链路未接通：这些类型只被「演示数据」和
 * 详情页的占位布局使用，任何演示实例必须明确标注「演示数据」，不得冒充真实会话。
 */
export type Channel = 'sms' | 'mail'

export interface AttachmentModel {
  name: string
  sizeBytes: number
}

export interface SmsCardModel {
  id: string
  /** 来源设备名称 */
  deviceName: string
  /** SIM 显示名（如 SIM 1） */
  simLabel: string
  /** 已确认的本机号码；无法可靠读取时为 null，界面显示「号码未确认」 */
  simNumber: string | null
  /** 发送方（收到的短信）或收件方（发出的短信） */
  counterpart: string
  /** sent | received */
  direction: 'sent' | 'received'
  snippet: string
  /** ISO 8601 时间 */
  time: string
  /** 收发状态，如 已接收 / 已发送 / 发送失败 */
  status: string
}

export interface MailCardModel {
  id: string
  /** 收件账号 */
  account: string
  from: string
  subject: string
  snippet: string
  time: string
  hasAttachment: boolean
  unread: boolean
}

export interface SmsDetailModel extends SmsCardModel {
  body: string
  /** 状态轨迹：状态、发生时间（ISO）、说明 */
  timeline: Array<{ state: string; at: string; note?: string }>
}

export interface MailDetailModel extends MailCardModel {
  /** HTML 正文；渲染前必须经过 src/lib/sanitizeHtml.ts 净化 */
  htmlBody?: string
  textBody?: string
  attachments: AttachmentModel[]
  timeline: Array<{ state: string; at: string; note?: string }>
}

export interface DeviceModel {
  id: string
  name: string
  sims: Array<{ label: string; number: string | null }>
  /** 权限摘要，如 短信读取已授权 / 未授权 */
  permissions: string
  /** 最后观测时间（ISO） */
  lastSeenAt: string
  /** revoked 时设备已撤销 */
  revoked: boolean
}

export interface MailAccountModel {
  id: string
  address: string
  /** connected = 最近一次连接测试成功 */
  connected: boolean
}

export function formatTime(iso: string): string {
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return iso
  return date.toLocaleString('zh-CN', { hour12: false })
}

export function formatSize(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes < 0) return '未知大小'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KiB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MiB`
}
