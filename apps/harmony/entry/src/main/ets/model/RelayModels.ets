/*
 * RelayModels — 卡片与侧栏条目的数据模型（字段对齐 FORMAL-UI-MAIL-PLAN §2.2）。
 * 文案字段一律用 ResourceStr（string 或 $r 资源），组件内不硬编码中文。
 * 正式列表当前不渲染任何卡片（正式同步未接通）；卡片组件只在「合成验证」页以
 * 演示数据（demo = true，必须显示「演示数据」标记）展示字段渲染能力。
 */
export interface SmsCardData {
  deviceName: ResourceStr
  simLabel: ResourceStr
  /** 已确认本机号码；null 时按规格显示「号码未确认」，不推测号码。 */
  confirmedLocalNumber: ResourceStr | null
  sender: ResourceStr
  snippet: ResourceStr
  timeText: ResourceStr
  statusText: ResourceStr
  directionText: ResourceStr
}

export interface MailCardData {
  accountAddress: ResourceStr
  sender: ResourceStr
  subject: ResourceStr
  snippet: ResourceStr
  timeText: ResourceStr
  hasAttachment: boolean
  unread: boolean
}

export interface DrawerDeviceSummary {
  id: string
  name: ResourceStr
  simLabelLine: ResourceStr
  statusText: ResourceStr
}

export interface DrawerAccountSummary {
  id: string
  address: ResourceStr
  connectionStateText: ResourceStr
}
