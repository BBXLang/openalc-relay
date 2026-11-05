/*
 * SearchPage — 当前频道搜索页（FORMAL-UI-MAIL-PLAN §2.1/§2.3）。
 * 正式搜索未接通，结果区诚实显示状态；搜索范围限定当前频道，不跨频道混合。
 * 返回（系统返回或返回钮）即恢复原列表（Navigation 主内容未销毁）。
 */
import { RelayNav } from '../common/RelayNav'
import { DetailTopBar } from '../components/DetailTopBar'
import { StateBlock } from '../components/StateBlock'
import { SearchGlyph } from '../components/RelayGlyphs'

@Component
export struct SearchPage {
  pathStack: NavPathStack = new NavPathStack()
  @State private channel: string = RelayNav.CHANNEL_SMS
  @State private query: string = ''

  build() {
    NavDestination() {
      Column() {
        Column() {
          DetailTopBar({
            pathStack: this.pathStack,
            title: this.channel === RelayNav.CHANNEL_MAIL
              ? $r('app.string.search_in_mail') : $r('app.string.search_in_sms')
          })

          Row() {
            Column() {
              SearchGlyph({ color: $r('app.color.relay_muted') })
            }
            .width(22)
            .height(22)

            TextInput({ placeholder: $r('app.string.search_field_hint'), text: this.query })
              .fontSize(14)
              .fontColor($r('app.color.relay_ink'))
              .placeholderColor($r('app.color.relay_muted'))
              .placeholderFont({ size: 14 })
              .backgroundColor(Color.Transparent)
              .layoutWeight(1)
              .height(44)
              .padding({ left: 8, right: 8 })
              .maxLength(200)
              .onChange((value: string) => {
                this.query = value
              })
              .accessibilityText(this.channel === RelayNav.CHANNEL_MAIL
                ? $r('app.string.search_in_mail') : $r('app.string.search_in_sms'))

            if (this.query !== '') {
              Text($r('app.string.search_clear'))
                .fontSize(12)
                .fontColor($r('app.color.relay_muted'))
                .padding({ left: 10, right: 6, top: 8, bottom: 8 })
                .onClick(() => {
                  this.query = ''
                })
                .accessibilityText($r('app.string.search_clear'))
            }
          }
          .width('100%')
          .borderRadius(50)
          .backgroundColor($r('app.color.relay_surface'))
          .padding({ left: 14, right: 14 })
          .margin({ left: 20, right: 20 })
          .alignItems(VerticalAlign.Center)

          Text(this.channel === RelayNav.CHANNEL_MAIL
            ? $r('app.string.search_scope_mail') : $r('app.string.search_scope_sms'))
            .fontSize(11)
            .fontColor($r('app.color.relay_muted'))
            .width('100%')
            .padding({ left: 28, right: 28, top: 8, bottom: 6 })

          Column() {
            StateBlock({
              message: $r('app.string.search_unavailable'),
              detail: this.channel === RelayNav.CHANNEL_MAIL
                ? $r('app.string.search_unavailable_detail_mail')
                : $r('app.string.search_unavailable_detail_sms')
            })
          }
          .width('100%')
          .layoutWeight(1)
          .justifyContent(FlexAlign.Center)
        }
        .width('100%')
        .constraintSize({ maxWidth: 780 })
        .height('100%')
      }
      .width('100%')
      .height('100%')
      .alignItems(HorizontalAlign.Center)
      .backgroundColor($r('app.color.relay_canvas'))
    }
    .hideTitleBar(true)
    .backgroundColor($r('app.color.relay_canvas'))
    .onReady((context: NavDestinationContext) => {
      this.pathStack = context.pathStack
      const param: string | null = context.pathInfo.param as string | null
      if (param === RelayNav.CHANNEL_MAIL) {
        this.channel = RelayNav.CHANNEL_MAIL
      } else {
        this.channel = RelayNav.currentChannel()
      }
    })
  }
}
