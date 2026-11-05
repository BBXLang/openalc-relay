/*
 * RelayCards — 短信 / 邮件卡片完整字段渲染（FORMAL-UI-MAIL-PLAN §2.2）：
 * - 短信：来源设备、SIM 显示名与「号码未确认」语义、发送方、正文缩略、时间、收发状态；
 * - 邮件：收件账号、发件人、主题、正文缩略、时间、附件标记、未读状态。
 * 正式列表暂不渲染卡片；卡片只出现在「合成验证」页且 demo = true 必须带「演示数据」标记。
 */
import { SmsCardData, MailCardData } from '../model/RelayModels'

@Component
export struct DemoBadge {
  build() {
    Text($r('app.string.demo_badge'))
      .fontSize(10)
      .fontColor($r('app.color.relay_accent_text'))
      .backgroundColor($r('app.color.relay_accent'))
      .borderRadius(50)
      .padding({ left: 8, right: 8, top: 3, bottom: 3 })
  }
}

@Component
export struct SmsCard {
  data: SmsCardData | null = null
  demo: boolean = false

  build() {
    Column() {
      if (this.demo) {
        DemoBadge()
        Blank().height(8)
      }
      Row() {
        Text(this.data?.sender ?? '')
          .fontSize(14)
          .fontWeight(FontWeight.Medium)
          .fontColor($r('app.color.relay_ink'))
          .layoutWeight(1)
          .maxLines(1)
          .textOverflow({ overflow: TextOverflow.Ellipsis })
        Text(this.data?.directionText ?? '')
          .fontSize(10)
          .fontColor($r('app.color.relay_muted'))
          .margin({ right: 8 })
        Text(this.data?.timeText ?? '')
          .fontSize(10)
          .fontColor($r('app.color.relay_muted'))
      }
      .width('100%')
      .alignItems(VerticalAlign.Center)

      Blank().height(4)
      Row() {
        Text(this.data?.deviceName ?? '')
          .fontSize(11)
          .fontColor($r('app.color.relay_muted'))
        Text(' · ').fontSize(11).fontColor($r('app.color.relay_muted'))
        Text(this.data?.simLabel ?? '')
          .fontSize(11)
          .fontColor($r('app.color.relay_muted'))
        Text(' · ').fontSize(11).fontColor($r('app.color.relay_muted'))
        Text(this.data?.confirmedLocalNumber ?? $r('app.string.number_unconfirmed'))
          .fontSize(11)
          .fontColor($r('app.color.relay_muted'))
      }
      .width('100%')

      Blank().height(6)
      Text(this.data?.snippet ?? '')
        .fontSize(13)
        .lineHeight(19)
        .fontColor($r('app.color.relay_ink'))
        .width('100%')
        .maxLines(2)
        .textOverflow({ overflow: TextOverflow.Ellipsis })

      Blank().height(6)
      Text(this.data?.statusText ?? '')
        .fontSize(11)
        .fontColor($r('app.color.relay_muted'))
        .width('100%')
    }
    .alignItems(HorizontalAlign.Start)
    .width('100%')
    .borderRadius(18)
    .backgroundColor($r('app.color.relay_surface'))
    .borderWidth(1)
    .borderColor($r('app.color.relay_border'))
    .padding(14)
  }
}

@Component
export struct MailCard {
  data: MailCardData | null = null
  demo: boolean = false

  build() {
    Column() {
      if (this.demo) {
        DemoBadge()
        Blank().height(8)
      }
      Row() {
        if (this.data?.unread ?? false) {
          Column()
            .width(8)
            .height(8)
            .borderRadius(4)
            .backgroundColor($r('app.color.relay_accent'))
          Blank().width(8)
        }
        Text(this.data?.sender ?? '')
          .fontSize(14)
          .fontWeight(FontWeight.Medium)
          .fontColor($r('app.color.relay_ink'))
          .layoutWeight(1)
          .maxLines(1)
          .textOverflow({ overflow: TextOverflow.Ellipsis })
        Text(this.data?.timeText ?? '')
          .fontSize(10)
          .fontColor($r('app.color.relay_muted'))
      }
      .width('100%')
      .alignItems(VerticalAlign.Center)

      Blank().height(4)
      Text(this.data?.subject ?? '')
        .fontSize(13)
        .fontColor($r('app.color.relay_ink'))
        .width('100%')
        .maxLines(1)
        .textOverflow({ overflow: TextOverflow.Ellipsis })

      Blank().height(4)
      Text(this.data?.snippet ?? '')
        .fontSize(12)
        .lineHeight(17)
        .fontColor($r('app.color.relay_muted'))
        .width('100%')
        .maxLines(2)
        .textOverflow({ overflow: TextOverflow.Ellipsis })

      Blank().height(6)
      Row() {
        Text($r('app.string.mail_card_account_prefix'))
          .fontSize(11)
          .fontColor($r('app.color.relay_muted'))
        Text(this.data?.accountAddress ?? '')
          .fontSize(11)
          .fontColor($r('app.color.relay_muted'))
          .layoutWeight(1)
          .maxLines(1)
          .textOverflow({ overflow: TextOverflow.Ellipsis })
        if (this.data?.hasAttachment ?? false) {
          Text($r('app.string.attachment_mark'))
            .fontSize(11)
            .fontColor($r('app.color.relay_muted'))
            .margin({ right: 8 })
        }
        Text((this.data?.unread ?? false) ? $r('app.string.mail_unread_state') : $r('app.string.mail_read_state'))
          .fontSize(11)
          .fontColor($r('app.color.relay_muted'))
      }
      .width('100%')
      .alignItems(VerticalAlign.Center)
    }
    .alignItems(HorizontalAlign.Start)
    .width('100%')
    .borderRadius(18)
    .backgroundColor($r('app.color.relay_surface'))
    .borderWidth(1)
    .borderColor($r('app.color.relay_border'))
    .padding(14)
  }
}
