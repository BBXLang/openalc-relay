/*
 * ComposeDock — 底部悬浮撰写栏（FORMAL-UI-MAIL-PLAN §2.1 底栏 / §2.3）：
 * - 短信页：小号收件号码输入 + 左侧设备/SIM 选择钮（正式链路未接通，点击说明原因）；
 * - 邮件页：小号收件地址输入 + 左侧发件账号选择钮（点击进入邮箱账号页）；
 * - 发送钮恒禁用（黄色但降低不透明度），原因文案可见：正式链路未接通；
 * - 草稿按频道独立保存于 preferences（正式链路开放时须改加密保存）。
 */
import { common } from '@kit.AbilityKit'
import { RelayStore } from '../common/RelayStore'
import { RelayNav } from '../common/RelayNav'
import { DeviceSimGlyph, EnvelopeGlyph, SendGlyph } from './RelayGlyphs'

@Component
export struct ComposeDock {
  pathStack: NavPathStack = new NavPathStack()
  @StorageProp('relay_channel') channel: string = RelayNav.CHANNEL_SMS
  @State private smsRecipient: string = ''
  @State private smsBody: string = ''
  @State private mailRecipient: string = ''
  @State private mailBody: string = ''

  aboutToAppear(): void {
    const context = getContext(this) as common.UIAbilityContext
    this.smsRecipient = RelayStore.loadDraft(context, RelayNav.CHANNEL_SMS, 'recipient')
    this.smsBody = RelayStore.loadDraft(context, RelayNav.CHANNEL_SMS, 'body')
    this.mailRecipient = RelayStore.loadDraft(context, RelayNav.CHANNEL_MAIL, 'recipient')
    this.mailBody = RelayStore.loadDraft(context, RelayNav.CHANNEL_MAIL, 'body')
  }

  private saveDraft(channel: string, field: string, value: string) {
    const context = getContext(this) as common.UIAbilityContext
    RelayStore.saveDraft(context, channel, field, value)
  }

  @Builder
  sourceButton() {
    // Outlined chip (user feedback 2026-11-06): icon + two-line label with a light border,
    // replacing the bare circle + floating text pair.
    Row({ space: 6 }) {
      Column() {
        if (this.channel === RelayNav.CHANNEL_SMS) {
          DeviceSimGlyph()
        } else {
          EnvelopeGlyph()
        }
      }
      .width(18)
      .height(18)

      Column({ space: 1 }) {
        Text(this.channel === RelayNav.CHANNEL_SMS
          ? $r('app.string.compose_source_sms') : $r('app.string.compose_source_mail'))
          .fontSize(9)
          .fontColor($r('app.color.relay_muted'))
          .maxLines(1)
        Text($r('app.string.compose_source_unselected'))
          .fontSize(11)
          .fontColor($r('app.color.relay_ink'))
          .maxLines(1)
      }
      .alignItems(HorizontalAlign.Start)
      .constraintSize({ minWidth: 64 })
    }
    .alignItems(VerticalAlign.Center)
    .padding({ left: 10, right: 10, top: 5, bottom: 5 })
    .border({ width: 1, color: $r('app.color.relay_border'), radius: 14 })
    .onClick(() => {
      if (this.channel === RelayNav.CHANNEL_MAIL) {
        this.pathStack.pushPathByName(RelayNav.PAGE_MAIL_ACCOUNTS, '')
      }
    })
    .accessibilityText(this.channel === RelayNav.CHANNEL_SMS
      ? $r('app.string.compose_source_sms') : $r('app.string.compose_source_mail'))
    .accessibilityDescription($r('app.string.compose_source_unavailable'))
  }

  @Builder
  sendButton() {
    Column() {
      SendGlyph()
    }
    .width(44)
    .height(44)
    .borderRadius(22)
    .backgroundColor($r('app.color.relay_accent'))
    .opacity(0.35)
    .enabled(false)
    .justifyContent(FlexAlign.Center)
    .alignItems(HorizontalAlign.Center)
    .accessibilityText($r('app.string.compose_send'))
    .accessibilityDescription($r('app.string.compose_send_disabled_reason'))
  }

  build() {
    Column() {
      TextInput({ placeholder: this.channel === RelayNav.CHANNEL_SMS
        ? $r('app.string.compose_recipient_sms_hint') : $r('app.string.compose_recipient_mail_hint'),
        text: this.channel === RelayNav.CHANNEL_SMS ? this.smsRecipient : this.mailRecipient })
        .fontSize(13)
        .fontColor($r('app.color.relay_ink'))
        .placeholderColor($r('app.color.relay_muted'))
        .placeholderFont({ size: 13 })
        .backgroundColor(Color.Transparent)
        .padding({ left: 10, right: 10 })
        .height(40)
        .maxLength(200)
        .onChange((value: string) => {
          if (this.channel === RelayNav.CHANNEL_SMS) {
            this.smsRecipient = value
            this.saveDraft(RelayNav.CHANNEL_SMS, 'recipient', value)
          } else {
            this.mailRecipient = value
            this.saveDraft(RelayNav.CHANNEL_MAIL, 'recipient', value)
          }
        })

      // Light divider between the recipient row and the body row (user feedback 2026-11-06).
      Divider()
        .color($r('app.color.relay_segment'))
        .strokeWidth(1)
        .width('100%')

      Row({ space: 8 }) {
        this.sourceButton()
        TextInput({ placeholder: this.channel === RelayNav.CHANNEL_SMS
          ? $r('app.string.compose_body_sms_hint') : $r('app.string.compose_body_mail_hint'),
          text: this.channel === RelayNav.CHANNEL_SMS ? this.smsBody : this.mailBody })
          .fontSize(15)
          .fontColor($r('app.color.relay_ink'))
          .placeholderColor($r('app.color.relay_muted'))
          .placeholderFont({ size: 15 })
          .backgroundColor($r('app.color.relay_segment'))
          .borderRadius(24)
          .height(46)
          .layoutWeight(1)
          .maxLength(2000)
          .onChange((value: string) => {
            if (this.channel === RelayNav.CHANNEL_SMS) {
              this.smsBody = value
              this.saveDraft(RelayNav.CHANNEL_SMS, 'body', value)
            } else {
              this.mailBody = value
              this.saveDraft(RelayNav.CHANNEL_MAIL, 'body', value)
            }
          })
        this.sendButton()
      }
      .width('100%')
      .alignItems(VerticalAlign.Center)
      .margin({ top: 4 })

      Text($r('app.string.compose_send_disabled_reason'))
        .fontSize(10)
        .fontColor($r('app.color.relay_muted'))
        .width('100%')
        .margin({ top: 4, left: 10 })
    }
    .alignItems(HorizontalAlign.Start)
    .width('100%')
    .constraintSize({ maxWidth: 740 })
    .backgroundColor($r('app.color.relay_surface'))
    .borderRadius(32)
    .padding({ left: 12, right: 12, top: 10, bottom: 8 })
    .margin({ left: 20, right: 20, bottom: 14, top: 6 })
  }
}
