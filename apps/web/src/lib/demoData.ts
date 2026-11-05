import type { DeviceModel, MailAccountModel, MailDetailModel, SmsDetailModel } from './model'

/*
 * 演示数据：仅存于本机 localStorage，用于独立验证入口（/dev/synthetic）、侧栏演示状态
 * 与演示详情布局。所有出现位置都必须标注「演示数据」；演示内容一律使用 example.invalid
 * 与明显占位的文本，不得冒充真实设备、账号或会话。
 */
const DEVICE_KEY = 'relay.demo.devices.v1'
const ACCOUNT_KEY = 'relay.demo.accounts.v1'

function readJson(key: string): unknown {
  try {
    const raw = localStorage.getItem(key)
    if (!raw) return null
    return JSON.parse(raw) as unknown
  } catch {
    return null
  }
}

function writeJson(key: string, value: unknown): void {
  try {
    localStorage.setItem(key, JSON.stringify(value))
  } catch {
    // 存储不可用时忽略：演示数据本就只是本机临时内容。
  }
}

function isDeviceModel(value: unknown): value is DeviceModel {
  if (value === null || typeof value !== 'object') return false
  const device = value as Record<string, unknown>
  return typeof device.id === 'string' && device.id.length > 0 &&
    typeof device.name === 'string' && Array.isArray(device.sims) &&
    typeof device.permissions === 'string' && typeof device.lastSeenAt === 'string' &&
    typeof device.revoked === 'boolean'
}

function isAccountModel(value: unknown): value is MailAccountModel {
  if (value === null || typeof value !== 'object') return false
  const account = value as Record<string, unknown>
  return typeof account.id === 'string' && account.id.length > 0 &&
    typeof account.address === 'string' && account.address.length > 0 &&
    typeof account.connected === 'boolean'
}

export function loadDemoDevices(): DeviceModel[] {
  const raw = readJson(DEVICE_KEY)
  return Array.isArray(raw) ? raw.filter(isDeviceModel).slice(0, 20) : []
}

export function saveDemoDevices(devices: DeviceModel[]): void {
  writeJson(DEVICE_KEY, devices)
}

export function loadDemoAccounts(): MailAccountModel[] {
  const raw = readJson(ACCOUNT_KEY)
  return Array.isArray(raw) ? raw.filter(isAccountModel).slice(0, 20) : []
}

export function saveDemoAccounts(accounts: MailAccountModel[]): void {
  writeJson(ACCOUNT_KEY, accounts)
}

export function clearDemoData(): void {
  try {
    localStorage.removeItem(DEVICE_KEY)
    localStorage.removeItem(ACCOUNT_KEY)
  } catch {
    // 忽略。
  }
}

const DEMO_TIME = '2026-11-05T09:30:00+08:00'
const DEMO_TIME_2 = '2026-11-05T09:31:40+08:00'

/** 演示短信记录：固定合成内容，仅供 /dev/synthetic 展示字段渲染。 */
export const demoSmsDetail: SmsDetailModel = {
  id: 'demo-sms-0001',
  deviceName: '演示设备 A',
  simLabel: 'SIM 1',
  simNumber: null,
  counterpart: '演示发件人',
  direction: 'received',
  snippet: '这是一条演示短信，用于展示卡片字段……',
  time: DEMO_TIME,
  status: '已接收',
  body: '这是一条演示短信，用于展示卡片与详情页的完整字段：来源设备、SIM 显示名、发送方、时间与状态轨迹。内容为固定合成文本，不代表任何真实会话。',
  timeline: [
    { state: '设备接收', at: DEMO_TIME, note: '演示设备 A · SIM 1' },
    { state: '同步至服务', at: DEMO_TIME_2, note: '演示数据，非真实同步' },
  ],
}

/** 演示邮件记录：包含待净化的 HTML 片段，用于验证 sanitizeHtml 行为。 */
export const demoMailDetail: MailDetailModel = {
  id: 'demo-mail-0001',
  account: 'demo.account@example.invalid',
  from: 'demo.sender@example.invalid',
  subject: '演示主题：卡片与详情字段展示',
  snippet: '这是一封演示邮件，包含待净化的 HTML……',
  time: DEMO_TIME,
  hasAttachment: true,
  unread: true,
  htmlBody:
    '<p>这是一封 <b>演示邮件</b>，正文经过保守白名单净化后渲染。</p>' +
    '<script>window.__demo_xss_probe = 1</script>' +
    '<p><a href="javascript:alert(1)">危险链接（应被移除）</a></p>' +
    '<p><a href="https://example.invalid/page">示例链接（应保留）</a></p>' +
    '<img src="https://example.invalid/remote.png" alt="远程图片默认不加载">',
  textBody: undefined,
  attachments: [{ name: '演示附件.pdf', sizeBytes: 204800 }],
  timeline: [
    { state: '服务商已接受', at: DEMO_TIME, note: '演示数据，非真实收信' },
    { state: '同步至收件箱', at: DEMO_TIME_2, note: '演示账号 demo.account@example.invalid' },
  ],
}
