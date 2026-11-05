/*
 * SettingsPage — 全局配置页（FORMAL-UI-MAIL-PLAN §2.1/§2.2）：
 * - 外观：亮色 / 暗色 / 跟随系统，覆盖写入 preferences 并经
 *   applicationContext.setColorMode 生效（页面颜色全走资源引用，立即切换）；
 * - 服务：配对与撤销（未接通）、邮箱管理（跳转）、安全与同步状态（未接通）；
 * - 开发：合成验证（仅开发）——PoC 唯一入口，不占正式首页。
 */
import { common } from '@kit.AbilityKit'
import { RelayStore, ThemeModeValue } from '../common/RelayStore'
import { RelayNav } from '../common/RelayNav'
import { DetailTopBar } from '../components/DetailTopBar'

@Component
export struct SettingsPage {
  pathStack: NavPathStack = new NavPathStack()
  @State private themeMode: string = ThemeModeValue.SYSTEM

  aboutToAppear(): void {
    const context = getContext(this) as common.UIAbilityContext
    this.themeMode = RelayStore.themeMode(context)
  }

  private selectTheme(mode: string) {
    this.themeMode = mode
    const context = getContext(this) as common.UIAbilityContext
    RelayStore.saveThemeMode(context, mode)
    RelayStore.applyColorMode(context.getApplicationContext(), mode)
  }

  @Builder
  sectionLabel(label: ResourceStr) {
    Text(label)
      .fontSize(12)
      .fontColor($r('app.color.relay_muted'))
      .width('100%')
      .margin({ left: 6, top: 18, bottom: 8 })
  }

  @Builder
  themeOption(label: Resource, mode: string) {
    Row({ space: 10 }) {
      Column() {
        if (this.themeMode === mode) {
          Column()
            .width(10)
            .height(10)
            .borderRadius(5)
            .backgroundColor($r('app.color.relay_ink'))
        }
      }
      .width(20)
      .height(20)
      .borderRadius(10)
      .borderWidth(2)
      .borderColor(this.themeMode === mode
        ? $r('app.color.relay_ink') : $r('app.color.relay_muted'))
      .justifyContent(FlexAlign.Center)
      .alignItems(HorizontalAlign.Center)

      Text(label)
        .fontSize(14)
        .fontColor($r('app.color.relay_ink'))
    }
    .width('100%')
    .padding({ left: 14, right: 14, top: 8, bottom: 8 })
    .onClick(() => {
      this.selectTheme(mode)
    })
    .accessibilityText(label)
  }

  @Builder
  serviceRow(title: Resource, trailing: ResourceStr | null, onPress: (() => void) | null) {
    Row() {
      Text(title)
        .fontSize(14)
        .fontColor($r('app.color.relay_ink'))
        .layoutWeight(1)
      if (trailing !== null) {
        Text(trailing)
          .fontSize(12)
          .fontColor($r('app.color.relay_muted'))
      }
    }
    .width('100%')
    .padding({ left: 14, right: 14, top: 14, bottom: 14 })
    .onClick(() => {
      if (onPress !== null) {
        onPress()
      }
    })
    .accessibilityText(title)
  }

  build() {
    NavDestination() {
      Column() {
        Column() {
          DetailTopBar({
            pathStack: this.pathStack,
            title: $r('app.string.settings_title')
          })

          Scroll() {
            Column() {
              this.sectionLabel($r('app.string.settings_appearance'))
              Column() {
                this.themeOption($r('app.string.theme_light'), 'light')
                this.themeOption($r('app.string.theme_dark'), 'dark')
                this.themeOption($r('app.string.theme_system'), 'system')
              }
              .width('100%')
              .borderRadius(18)
              .backgroundColor($r('app.color.relay_surface'))
              .padding({ top: 4, bottom: 4 })

              this.sectionLabel($r('app.string.settings_services'))
              Column() {
                this.serviceRow($r('app.string.settings_pairing'), $r('app.string.not_connected_short'), null)
                this.serviceRow($r('app.string.settings_mail_accounts'), null, () => {
                  this.pathStack.pushPathByName(RelayNav.PAGE_MAIL_ACCOUNTS, '')
                })
                this.serviceRow($r('app.string.settings_security_sync'),
                  $r('app.string.settings_security_unavailable'), null)
              }
              .width('100%')
              .borderRadius(18)
              .backgroundColor($r('app.color.relay_surface'))
              .padding({ top: 4, bottom: 4 })

              this.sectionLabel($r('app.string.settings_developer'))
              Column() {
                this.serviceRow($r('app.string.settings_synthetic_verification'), null, () => {
                  this.pathStack.pushPathByName(RelayNav.PAGE_SYNTHETIC, '')
                })
              }
              .width('100%')
              .borderRadius(18)
              .backgroundColor($r('app.color.relay_surface'))
              .padding({ top: 4, bottom: 4 })

              Text($r('app.string.settings_footer_notice'))
                .fontSize(11)
                .lineHeight(16)
                .fontColor($r('app.color.relay_muted'))
                .width('100%')
                .margin({ top: 24, bottom: 24 })
            }
            .alignItems(HorizontalAlign.Start)
            .width('100%')
            .padding({ left: 22, right: 22, top: 4 })
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
