/*
 * MailAccountsPage — 邮箱账号（设置）页（FORMAL-UI-MAIL-PLAN §2.1）。
 * 正式邮件服务未接通：不伪造账号与连接测试，说明首批将支持的通用 IMAP／SMTP 能力。
 */
import { DetailTopBar } from '../components/DetailTopBar'

@Component
export struct MailAccountsPage {
  pathStack: NavPathStack = new NavPathStack()

  build() {
    NavDestination() {
      Column() {
        Column() {
          DetailTopBar({
            pathStack: this.pathStack,
            title: $r('app.string.mail_accounts_title')
          })

          Scroll() {
            Column() {
              Text($r('app.string.mail_accounts_status'))
                .fontSize(14)
                .fontWeight(FontWeight.Medium)
                .fontColor($r('app.color.relay_ink'))
                .width('100%')
              Text($r('app.string.mail_accounts_plan'))
                .fontSize(12)
                .lineHeight(18)
                .fontColor($r('app.color.relay_muted'))
                .width('100%')
                .margin({ top: 10 })

              Row() {
                Text($r('app.string.mail_accounts_add'))
                  .fontSize(13)
                  .fontColor($r('app.color.relay_muted'))
              }
              .borderRadius(22)
              .backgroundColor($r('app.color.relay_segment'))
              .padding({ left: 18, right: 18, top: 10, bottom: 10 })
              .margin({ top: 18 })
              .enabled(false)
              .accessibilityText($r('app.string.mail_accounts_add_disabled'))

              Text($r('app.string.mail_accounts_add_disabled'))
                .fontSize(11)
                .fontColor($r('app.color.relay_muted'))
                .width('100%')
                .margin({ top: 8 })
            }
            .alignItems(HorizontalAlign.Start)
            .width('100%')
            .padding({ left: 28, right: 28, top: 12, bottom: 24 })
          }
          .width('100%')
          .layoutWeight(1)
          .align(Alignment.TopStart)
          .scrollable(ScrollDirection.Vertical)
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
    })
  }
}
