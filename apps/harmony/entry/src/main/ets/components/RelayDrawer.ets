/*
 * RelayDrawer — 侧栏内容（FORMAL-UI-MAIL-PLAN §2.2）：
 * - 文字 logo：`Relay` 独占一行、字距约 0.14em（23fp 字号 ≈ 3.2fp），`by OpenALC`
 *   排在其右下（约 45% 字号），次要文字色；保持 Relay 独立标识；
 * - 「设备」「邮件」两个可独立展开的组；正式配对/账号未接通，列表诚实为空并给出
 *   配对 / 添加账号动作；「配置」为页面入口；
 * - 不含任何 PoC 操作（合成验证仅从 配置 → 开发 进入）。
 */
import { RelayNav } from '../common/RelayNav'
import { DrawerCaret } from './RelayGlyphs'

@Component
export struct RelayDrawer {
  pathStack: NavPathStack = new NavPathStack()
  onClose: () => void = () => {
  }
  @State private devicesOpen: boolean = false
  @State private mailOpen: boolean = false

  @Builder
  actionPill(label: Resource, onPress: () => void) {
    Row() {
      Text(label)
        .fontSize(12)
        .fontColor($r('app.color.relay_ink'))
    }
    .borderRadius(50)
    .backgroundColor($r('app.color.relay_segment'))
    .padding({ left: 14, right: 14, top: 8, bottom: 8 })
    .margin({ top: 6, bottom: 4 })
    .onClick(() => {
      onPress()
    })
    .accessibilityText(label)
  }

  @Builder
  deviceGroup() {
    Column() {
      Row() {
        Text($r('app.string.drawer_devices'))
          .fontSize(15)
          .fontWeight(FontWeight.Medium)
          .fontColor($r('app.color.relay_ink'))
          .layoutWeight(1)
        DrawerCaret({ expanded: this.devicesOpen })
      }
      .width('100%')
      .padding({ top: 8, bottom: 8 })
      .onClick(() => {
        this.devicesOpen = !this.devicesOpen
      })
      .accessibilityText($r('app.string.drawer_devices'))

      if (this.devicesOpen) {
        Column() {
          Text($r('app.string.drawer_no_devices'))
            .fontSize(12)
            .fontColor($r('app.color.relay_muted'))
            .margin({ left: 10, top: 4 })
          this.actionPill($r('app.string.drawer_pair_device'), () => {
            AlertDialog.show({
              title: $r('app.string.drawer_pair_device'),
              message: $r('app.string.drawer_pairing_unavailable'),
              confirm: {
                value: $r('app.string.dialog_ok'),
                action: () => {
                }
              }
            })
          })
        }
        .alignItems(HorizontalAlign.Start)
        .width('100%')
        .margin({ left: 10 })
      }
    }
    .alignItems(HorizontalAlign.Start)
    .width('100%')
  }

  @Builder
  mailGroup() {
    Column() {
      Row() {
        Text($r('app.string.drawer_mail'))
          .fontSize(15)
          .fontWeight(FontWeight.Medium)
          .fontColor($r('app.color.relay_ink'))
          .layoutWeight(1)
        DrawerCaret({ expanded: this.mailOpen })
      }
      .width('100%')
      .padding({ top: 8, bottom: 8 })
      .onClick(() => {
        this.mailOpen = !this.mailOpen
      })
      .accessibilityText($r('app.string.drawer_mail'))

      if (this.mailOpen) {
        Column() {
          Text($r('app.string.drawer_no_accounts'))
            .fontSize(12)
            .fontColor($r('app.color.relay_muted'))
            .margin({ left: 10, top: 4 })
          this.actionPill($r('app.string.drawer_add_account'), () => {
            this.onClose()
            this.pathStack.pushPathByName(RelayNav.PAGE_MAIL_ACCOUNTS, '')
          })
        }
        .alignItems(HorizontalAlign.Start)
        .width('100%')
        .margin({ left: 10 })
      }
    }
    .alignItems(HorizontalAlign.Start)
    .width('100%')
  }

  build() {
    Column() {
      Column() {
        Text('Relay')
          .fontSize(23)
          .fontWeight(FontWeight.Medium)
          .letterSpacing(3.2)
          .fontColor($r('app.color.relay_ink'))
        Text($r('app.string.drawer_by_openalc'))
          .fontSize(10)
          .fontColor($r('app.color.relay_muted'))
          .width('100%')
          .textAlign(TextAlign.End)
      }
      .alignItems(HorizontalAlign.Start)
      .width('100%')

      Blank().height(26)
      this.deviceGroup()
      Blank().height(14)
      this.mailGroup()
      Blank().height(14)

      this.actionPill($r('app.string.drawer_settings'), () => {
        this.onClose()
        this.pathStack.pushPathByName(RelayNav.PAGE_SETTINGS, '')
      })

      Blank().height(22)
      Text($r('app.string.drawer_footer_notice'))
        .fontSize(11)
        .lineHeight(16)
        .fontColor($r('app.color.relay_muted'))
    }
    .alignItems(HorizontalAlign.Start)
    .width('100%')
    .height('100%')
    .backgroundColor($r('app.color.relay_canvas'))
    .padding(22)
    .onClick(() => {
      // 点击面板本体不关闭（由遮罩/关闭钮/系统返回关闭）。
    })
  }
}
