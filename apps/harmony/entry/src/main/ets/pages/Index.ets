/*
 * Index — 应用入口（@Entry）。
 *
 * 架构：ArkUI Navigation + NavPathStack（选择理由见 common/RelayNav.ets）：
 * - 主内容是首页（短信/邮件双窗格列表 + 顶栏 + 撰写栏），不销毁；
 * - 其余屏（搜索、短信/邮件/设备详情、邮箱账号、配置、合成验证）以 NavDestination
 *   推入，系统返回逐级回退，详情返回恢复原列表与频道状态；
 * - 侧栏抽屉悬浮于 Navigation 之上：遮罩点击、关闭钮与系统返回均可关闭，开合不改变
 *   当前列表、筛选或草稿（FORMAL-UI-MAIL-PLAN §2.1）。
 */
import { RelayNav } from '../common/RelayNav'
import { ChannelListPage } from './ChannelListPage'
import { SearchPage } from './SearchPage'
import { DetailUnavailablePage } from './DetailUnavailablePage'
import { MailAccountsPage } from './MailAccountsPage'
import { SettingsPage } from './SettingsPage'
import { SyntheticPage } from './SyntheticPage'
import { RelayDrawer } from '../components/RelayDrawer'

@Entry
@Component
struct Index {
  pathStack: NavPathStack = new NavPathStack()
  @State private drawerOpen: boolean = false

  onBackPress(): boolean {
    if (this.drawerOpen) {
      this.drawerOpen = false
      return true
    }
    return false
  }

  @Builder
  PageMap(name: string) {
    if (name === RelayNav.PAGE_SEARCH) {
      SearchPage()
    } else if (name === RelayNav.PAGE_SMS_DETAIL) {
      DetailUnavailablePage({ titleRes: $r('app.string.detail_sms_title') })
    } else if (name === RelayNav.PAGE_MAIL_DETAIL) {
      DetailUnavailablePage({ titleRes: $r('app.string.detail_mail_title') })
    } else if (name === RelayNav.PAGE_DEVICE_DETAIL) {
      DetailUnavailablePage({ titleRes: $r('app.string.detail_device_title') })
    } else if (name === RelayNav.PAGE_MAIL_ACCOUNTS) {
      MailAccountsPage()
    } else if (name === RelayNav.PAGE_SETTINGS) {
      SettingsPage()
    } else if (name === RelayNav.PAGE_SYNTHETIC) {
      SyntheticPage()
    }
  }

  build() {
    Stack({ alignContent: Alignment.Start }) {
      Navigation(this.pathStack) {
        ChannelListPage({
          pathStack: this.pathStack,
          onOpenDrawer: () => {
            this.drawerOpen = true
          }
        })
      }
      .mode(NavigationMode.Stack)
      .hideTitleBar(true)
      .navDestination(this.PageMap)
      .width('100%')
      .height('100%')

      if (this.drawerOpen) {
        Column()
          .width('100%')
          .height('100%')
          .backgroundColor($r('app.color.relay_scrim'))
          .onClick(() => {
            this.drawerOpen = false
          })
          .transition(TransitionEffect.OPACITY.animation({ duration: 200, curve: Curve.EaseOut }))
          .accessibilityText($r('app.string.close_drawer'))

        Column() {
          RelayDrawer({
            pathStack: this.pathStack,
            onClose: () => {
              this.drawerOpen = false
            }
          })
        }
        .width('56%')
        .constraintSize({ minWidth: 220, maxWidth: 300 })
        .height('100%')
        .transition(TransitionEffect.translate({ x: -360 })
          .animation({ duration: 250, curve: Curve.EaseOut }))
      }
    }
    .width('100%')
    .height('100%')
    .backgroundColor($r('app.color.relay_canvas'))
  }
}
