/*
 * RelayTopBar — 正式顶栏（FORMAL-UI-MAIL-PLAN §2.2）：
 * 左圆形菜单钮开侧栏；中央「短信／邮件」分段（白色滑块随频道平移，约 250ms，
 * 系统减少动态效果时取消位移）；右圆形搜索钮（带无障碍名称，进入当前频道搜索页）。
 */
import { common } from '@kit.AbilityKit'
import { RelayStore } from '../common/RelayStore'
import { RelayNav } from '../common/RelayNav'
import { CircleIconButton } from './CircleIconButton'
import { MenuGlyph, SearchGlyph } from './RelayGlyphs'

@Component
export struct RelayTopBar {
  pathStack: NavPathStack = new NavPathStack()
  onOpenDrawer: () => void = () => {
  }
  @StorageProp('relay_channel') channel: string = RelayNav.CHANNEL_SMS

  /** 统一入口：切换频道时在 250ms 事务内更新 AppStorage，分段滑块与列表窗格同步滑移。 */
  switchChannel(target: string) {
    if (target === RelayNav.currentChannel()) {
      return
    }
    const context = getContext(this) as common.UIAbilityContext
    const duration: number = RelayStore.isReducedAnimation(context) ? 0 : 250
    this.getUIContext().animateTo({ duration: duration, curve: Curve.EaseOut }, () => {
      AppStorage.setOrCreate<string>('relay_channel', target)
    })
  }

  build() {
    Row() {
      CircleIconButton({
        label: $r('app.string.menu_open'),
        onPress: () => {
          this.onOpenDrawer()
        }
      }) {
        MenuGlyph()
      }

      Row() {
        Stack({ alignContent: Alignment.Start }) {
          Row()
            .width('50%')
            .height('100%')
            .borderRadius(25)
            .backgroundColor($r('app.color.relay_surface'))
            .translate({ x: this.channel === RelayNav.CHANNEL_MAIL ? '100%' : '0%' })

          Row() {
            Text($r('app.string.channel_sms'))
              .fontSize(15)
              .fontColor($r('app.color.relay_ink'))
              .width('50%')
              .textAlign(TextAlign.Center)
              .accessibilityText($r('app.string.channel_sms'))
              .onClick(() => {
                this.switchChannel(RelayNav.CHANNEL_SMS)
              })
            Text($r('app.string.channel_mail'))
              .fontSize(15)
              .fontColor($r('app.color.relay_ink'))
              .width('50%')
              .textAlign(TextAlign.Center)
              .accessibilityText($r('app.string.channel_mail'))
              .onClick(() => {
                this.switchChannel(RelayNav.CHANNEL_MAIL)
              })
          }
          .width('100%')
          .height('100%')
          .alignItems(VerticalAlign.Center)
        }
        .width('100%')
        .height(50)
        .borderRadius(25)
        .backgroundColor($r('app.color.relay_segment'))
        .padding(4)
        .accessibilityText($r('app.string.segment_a11y'))
      }
      .layoutWeight(1)
      .margin({ left: 14, right: 14 })

      CircleIconButton({
        label: $r('app.string.search'),
        onPress: () => {
          this.pathStack.pushPathByName(RelayNav.PAGE_SEARCH, RelayNav.currentChannel())
        }
      }) {
        SearchGlyph()
      }
    }
    .width('100%')
    .padding({ left: 16, right: 16, top: 16, bottom: 16 })
  }
}
