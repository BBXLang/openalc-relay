/*
 * DetailUnavailablePage — 短信详情 / 邮件详情 / 设备详情共用模板。
 * 正式同步未接通：显示诚实状态与内容 ID（如果推入带参），不渲染伪造数据。
 * 系统返回 / 返回钮 pop 后恢复原列表与筛选（Navigation 主内容保留）。
 */
import { DetailTopBar } from '../components/DetailTopBar'
import { StateBlock } from '../components/StateBlock'

@Component
export struct DetailUnavailablePage {
  pathStack: NavPathStack = new NavPathStack()
  titleRes: Resource = $r('app.string.detail_sms_title')
  @State private idResource: ResourceStr | null = null

  build() {
    NavDestination() {
      Column() {
        Column() {
          DetailTopBar({
            pathStack: this.pathStack,
            title: this.titleRes
          })

          Column() {
            StateBlock({
              message: $r('app.string.detail_unavailable'),
              detail: this.idResource
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
      if (param !== null && param !== '') {
        this.idResource = $r('app.string.detail_id_label', param)
      }
    })
  }
}
