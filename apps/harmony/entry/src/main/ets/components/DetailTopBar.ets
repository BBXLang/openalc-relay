/*
 * DetailTopBar — 子页顶栏：圆形返回钮 + 页面标题。系统返回与返回钮等效（pop）。
 */
import { CircleIconButton } from './CircleIconButton'
import { BackChevronGlyph } from './RelayGlyphs'

@Component
export struct DetailTopBar {
  pathStack: NavPathStack = new NavPathStack()
  title: ResourceStr = ''

  build() {
    Row() {
      CircleIconButton({
        label: $r('app.string.back'),
        onPress: () => {
          this.pathStack.pop()
        }
      }) {
        BackChevronGlyph()
      }
      Text(this.title)
        .fontSize(19)
        .fontWeight(FontWeight.Medium)
        .fontColor($r('app.color.relay_ink'))
        .margin({ left: 14 })
      Blank()
    }
    .width('100%')
    .padding({ left: 16, right: 16, top: 16, bottom: 16 })
    .alignItems(VerticalAlign.Center)
  }
}
