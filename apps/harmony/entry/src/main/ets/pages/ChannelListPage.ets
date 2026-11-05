/*
 * ChannelListPage — Navigation 主内容（首页）：正式顶栏 + 短信/邮件双窗格 + 底部撰写栏。
 * 内容宽度按平板视口限制（最大 780vp 居中），沿用既有布局密度。
 */
import { RelayTopBar } from '../components/RelayTopBar'
import { ChannelPager } from '../components/ChannelPager'
import { ComposeDock } from '../components/ComposeDock'

@Component
export struct ChannelListPage {
  pathStack: NavPathStack = new NavPathStack()
  onOpenDrawer: () => void = () => {
  }

  build() {
    Column() {
      Column() {
        RelayTopBar({
          pathStack: this.pathStack,
          onOpenDrawer: (): void => {
            this.onOpenDrawer()
          }
        })

        ChannelPager()

        ComposeDock({ pathStack: this.pathStack })
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
}
