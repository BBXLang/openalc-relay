/*
 * RelayNav — 导航目的地名称与频道状态常量（FORMAL-UI-MAIL-PLAN §2.1 导航表）。
 *
 * 采用 ArkUI Navigation + NavPathStack（而非 @ohos.router）：
 * - 系统返回手势/返回键由 Navigation 自动逐级回退（NavDestination pop），
 *   详情返回后列表（Navigation 主内容）不销毁，滚动位置与频道、草稿自然保留；
 * - 抽屉、搜索、详情、设置共享同一个 NavPathStack，无需手工维护返回栈。
 *
 * 短信列表 / 邮件列表是 Navigation 主内容里的双窗格（ChannelPager），频道切换不换页；
 * 其余屏以 NavDestination 推入。PoC 的合成验证仅由 配置 → 开发 进入。
 */
export class RelayNav {
  static readonly CHANNEL_SMS: string = 'sms'
  static readonly CHANNEL_MAIL: string = 'mail'

  static readonly PAGE_SEARCH: string = 'search'
  static readonly PAGE_SMS_DETAIL: string = 'sms-detail'
  static readonly PAGE_MAIL_DETAIL: string = 'mail-detail'
  static readonly PAGE_DEVICE_DETAIL: string = 'device-detail'
  static readonly PAGE_MAIL_ACCOUNTS: string = 'mail-accounts'
  static readonly PAGE_SETTINGS: string = 'settings'
  static readonly PAGE_SYNTHETIC: string = 'synthetic'

  static currentChannel(): string {
    const value = AppStorage.get<string>('relay_channel')
    return value === RelayNav.CHANNEL_MAIL ? RelayNav.CHANNEL_MAIL : RelayNav.CHANNEL_SMS
  }
}
