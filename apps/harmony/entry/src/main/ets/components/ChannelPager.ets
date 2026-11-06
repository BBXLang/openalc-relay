/*
 * ChannelPager — 短信/邮件双窗格容器（FORMAL-UI-MAIL-PLAN §2.2）：
 * - 两个窗格常驻，频道切换不重建（滚动位置与状态保留）；
 * - 选中内容随频道切换滑移约 250ms；点击分段或列表内容区左右滑（阈值 60vp）均可切换；
 * - 系统减少动态效果时：取消位移动画并禁用手势拖拽（仍可点击分段切换）；
 * - 手势只绑定在本容器（列表内容区），顶栏与撰写栏不在其内，不会误触。
 */
import { common } from '@kit.AbilityKit'
import { RelayStore } from '../common/RelayStore'
import { RelayNav } from '../common/RelayNav'
import { StateBlock } from './StateBlock'

const SWITCH_THRESHOLD_VP: number = 60
const SWITCH_DURATION_MS: number = 250

@Component
export struct ChannelPager {
  @StorageProp('relay_channel') channel: string = RelayNav.CHANNEL_SMS
  @State private dragging: boolean = false
  @State private dragX: number = 0
  @State private viewportWidth: number = 0
  @State private reduced: boolean = false

  aboutToAppear(): void {
    const context = getContext(this) as common.UIAbilityContext
    this.reduced = RelayStore.isReducedAnimation(context)
  }

  // Number in vp (not '50%') so the drag→settle transition never mixes units, which would break
  // animation continuity and make the released swipe jump back first (user feedback 2026-11-06).
  private settledX(): number {
    return this.channel === RelayNav.CHANNEL_MAIL ? -this.viewportWidth : 0
  }

  private baseX(): number {
    return this.channel === RelayNav.CHANNEL_MAIL ? -this.viewportWidth : 0
  }

  private switchChannel(target: string) {
    if (target === RelayNav.currentChannel()) {
      return
    }
    const duration: number = this.reduced ? 0 : SWITCH_DURATION_MS
    this.getUIContext().animateTo({ duration: duration, curve: Curve.EaseOut }, () => {
      this.dragging = false
      this.dragX = 0
      AppStorage.setOrCreate<string>('relay_channel', target)
    })
  }

  private settleBack() {
    const duration: number = this.reduced ? 0 : SWITCH_DURATION_MS
    this.getUIContext().animateTo({ duration: duration, curve: Curve.EaseOut }, () => {
      this.dragging = false
      this.dragX = 0
    })
  }

  build() {
    Stack({ alignContent: Alignment.Start }) {
      Row() {
        // 短信窗格：正式同步未接通，诚实空态，不伪造会话。
        Column() {
          StateBlock({
            message: $r('app.string.formal_sync_not_connected'),
            detail: $r('app.string.sms_list_empty_detail')
          })
        }
        .width('50%')
        .height('100%')
        .justifyContent(FlexAlign.Center)
        .accessibilityText($r('app.string.channel_sms'))

        // 邮件窗格：同上。
        Column() {
          StateBlock({
            message: $r('app.string.formal_sync_not_connected'),
            detail: $r('app.string.mail_list_empty_detail')
          })
        }
        .width('50%')
        .height('100%')
        .justifyContent(FlexAlign.Center)
        .accessibilityText($r('app.string.channel_mail'))
      }
      .width('200%')
      .height('100%')
      .translate({ x: this.dragging ? this.dragX : this.settledX() })
    }
    .width('100%')
    .layoutWeight(1)
    .clip(true)
    .onAreaChange((_old: Area, newValue: Area) => {
      const width = newValue.width
      if (typeof width === 'number') {
        this.viewportWidth = width
      } else {
        this.viewportWidth = parseFloat(String(width))
      }
    })
    .gesture(PanGesture({ fingers: 1, direction: PanDirection.Horizontal, distance: 12 })
      .onActionStart((_event: GestureEvent) => {
        if (this.reduced || this.viewportWidth <= 0) {
          return
        }
        this.dragging = true
        this.dragX = this.baseX()
      })
      .onActionUpdate((event: GestureEvent) => {
        if (!this.dragging || this.reduced) {
          return
        }
        const next = this.baseX() + event.offsetX
        this.dragX = Math.min(0, Math.max(-this.viewportWidth, next))
      })
      .onActionEnd((event: GestureEvent) => {
        if (!this.dragging || this.reduced) {
          return
        }
        const delta = event.offsetX
        if (Math.abs(delta) >= SWITCH_THRESHOLD_VP) {
          const target = delta < 0 ? RelayNav.CHANNEL_MAIL : RelayNav.CHANNEL_SMS
          this.switchChannel(target)
        } else {
          this.settleBack()
        }
      })
      .onActionCancel(() => {
        if (this.dragging && !this.reduced) {
          this.settleBack()
        }
      }))
  }
}
